package sweep

import (
	"fmt"
	"strings"

	"github.com/midnattsol/docker-sweep/internal/docker"
)

// ResourceType identifies the type of Docker resource
type ResourceType string

const (
	TypeContainer ResourceType = "container"
	TypeImage     ResourceType = "image"
	TypeVolume    ResourceType = "volume"
	TypeNetwork   ResourceType = "network"
)

// Category represents why a resource is suggested for deletion
type Category string

const (
	CategoryProtected Category = "protected" // Cannot be deleted (running, in use, system)
	CategoryInUse     Category = "in_use"    // In use but could be stopped/removed
	CategorySuggested Category = "suggested" // Safe to delete (stopped, unused, dangling)
	CategoryUnused    Category = "unused"    // Not in use but not suggested (has custom name/tag)
)

// Resource is the interface for all Docker resources
type Resource interface {
	ID() string
	Type() ResourceType
	DisplayName() string
	Category() Category
	Details() string
	Size() int64 // Size in bytes, 0 if unknown
	IsProtected() bool
	IsSuggested() bool
}

// ComposeResource is an optional interface for resources that belong to a Compose project
type ComposeResource interface {
	Resource
	ComposeProject() string
}

// GetComposeProject returns the Compose project name if the resource implements ComposeResource
func GetComposeProject(r Resource) string {
	if cr, ok := r.(ComposeResource); ok {
		return cr.ComposeProject()
	}
	return ""
}

// Result holds all analyzed resources
type Result struct {
	Containers []ContainerResource
	Images     []ImageResource
	Volumes    []VolumeResource
	Networks   []NetworkResource
}

// IsEmpty returns true if there are no resources to show
func (r *Result) IsEmpty() bool {
	return len(r.Containers) == 0 &&
		len(r.Images) == 0 &&
		len(r.Volumes) == 0 &&
		len(r.Networks) == 0
}

// Suggested returns all resources suggested for deletion
func (r *Result) Suggested() []Resource {
	var suggested []Resource

	for i := range r.Containers {
		if r.Containers[i].IsSuggested() {
			suggested = append(suggested, &r.Containers[i])
		}
	}
	for i := range r.Images {
		if r.Images[i].IsSuggested() {
			suggested = append(suggested, &r.Images[i])
		}
	}
	for i := range r.Volumes {
		if r.Volumes[i].IsSuggested() {
			suggested = append(suggested, &r.Volumes[i])
		}
	}
	for i := range r.Networks {
		if r.Networks[i].IsSuggested() {
			suggested = append(suggested, &r.Networks[i])
		}
	}

	return suggested
}

// All returns all non-protected resources
func (r *Result) All() []Resource {
	var all []Resource

	for i := range r.Containers {
		if !r.Containers[i].IsProtected() {
			all = append(all, &r.Containers[i])
		}
	}
	for i := range r.Images {
		if !r.Images[i].IsProtected() {
			all = append(all, &r.Images[i])
		}
	}
	for i := range r.Volumes {
		if !r.Volumes[i].IsProtected() {
			all = append(all, &r.Volumes[i])
		}
	}
	for i := range r.Networks {
		if !r.Networks[i].IsProtected() {
			all = append(all, &r.Networks[i])
		}
	}

	return all
}

// TotalSize returns the total size of suggested resources
func (r *Result) TotalSize() int64 {
	var total int64
	for _, res := range r.Suggested() {
		total += res.Size()
	}
	return total
}

// DeleteResources deletes the given resources in the correct order:
// 1. Containers first (so images/volumes/networks can be freed)
// 2. Networks and Volumes (order doesn't matter between them)
// 3. Images last (with retry for dependency resolution)
func DeleteResources(resources []Resource) (int, []error) {
	// Separate by type
	var containers, images, volumes, networks []Resource
	for _, r := range resources {
		switch r.Type() {
		case TypeContainer:
			containers = append(containers, r)
		case TypeImage:
			images = append(images, r)
		case TypeVolume:
			volumes = append(volumes, r)
		case TypeNetwork:
			networks = append(networks, r)
		}
	}

	var totalDeleted int
	var allErrors []error

	// 1. Containers first
	d, e := deleteAll(containers)
	totalDeleted += d
	allErrors = append(allErrors, e...)

	// 2. Networks
	d, e = deleteAll(networks)
	totalDeleted += d
	allErrors = append(allErrors, e...)

	// 3. Volumes
	d, e = deleteAll(volumes)
	totalDeleted += d
	allErrors = append(allErrors, e...)

	// 4. Images last, with retry for dependencies
	d, e = deleteImagesWithRetry(images)
	totalDeleted += d
	allErrors = append(allErrors, e...)

	return totalDeleted, allErrors
}

// deleteAll deletes resources without retry
func deleteAll(resources []Resource) (int, []error) {
	var deleted int
	var errors []error

	for _, res := range resources {
		if err := docker.Remove(string(res.Type()), res.ID()); err != nil {
			if isAlreadyRemovedError(res.Type(), err) {
				deleted++
				continue
			}
			errors = append(errors, fmt.Errorf("%s: %w", res.DisplayName(), err))
		} else {
			deleted++
		}
	}

	return deleted, errors
}

// deleteImagesWithRetry deletes images with retry for dependency resolution.
// Images can have parent-child relationships, so we may need multiple passes.
func deleteImagesWithRetry(resources []Resource) (int, []error) {
	var deleted int
	var errors []error
	pending := resources

	// Maximum 3 passes to resolve dependencies
	for attempt := 0; attempt < 3 && len(pending) > 0; attempt++ {
		var failed []Resource
		for _, r := range pending {
			if err := docker.Remove(string(r.Type()), r.ID()); err != nil {
				if isAlreadyRemovedError(r.Type(), err) {
					deleted++
					continue
				}
				// If it's a dependency error, retry later
				if isDependencyError(err) {
					failed = append(failed, r)
				} else {
					errors = append(errors, fmt.Errorf("%s: %w", r.DisplayName(), err))
				}
			} else {
				deleted++
			}
		}
		pending = failed
	}

	// What's left after 3 attempts has unresolvable dependencies
	for _, r := range pending {
		errors = append(errors, fmt.Errorf("%s: has dependent images (not deleted)", r.DisplayName()))
	}

	return deleted, errors
}

// isDependencyError checks if the error is due to image dependencies
func isDependencyError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "dependent") ||
		strings.Contains(errStr, "has dependent child images") ||
		strings.Contains(errStr, "image is being used") ||
		strings.Contains(errStr, "image has dependent child images")
}

// isAlreadyRemovedError checks if the resource is already gone.
// These errors should be treated as idempotent success.
func isAlreadyRemovedError(resourceType ResourceType, err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())

	common := strings.Contains(errStr, "not found") || strings.Contains(errStr, "no such")
	if common {
		switch resourceType {
		case TypeImage:
			return strings.Contains(errStr, "image")
		case TypeContainer:
			return strings.Contains(errStr, "container")
		case TypeVolume:
			return strings.Contains(errStr, "volume")
		case TypeNetwork:
			return strings.Contains(errStr, "network")
		}
	}

	if resourceType == TypeImage && strings.Contains(errStr, "image not known") {
		return true
	}

	return false
}
