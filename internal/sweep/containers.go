package sweep

import (
	"fmt"
	"strings"
	"time"

	"github.com/midnattsol/docker-sweep/internal/config"
	"github.com/midnattsol/docker-sweep/internal/docker"
)

// ContainerResource represents an analyzed container
type ContainerResource struct {
	container      docker.Container
	category       Category
	labels         map[string]string
	createdAt      time.Time
	composeProject string
	protectReason  string
}

// Implement Resource interface
func (c *ContainerResource) ID() string             { return c.container.ID }
func (c *ContainerResource) Type() ResourceType     { return TypeContainer }
func (c *ContainerResource) Category() Category     { return c.category }
func (c *ContainerResource) Size() int64            { return 0 } // Container size is complex to parse
func (c *ContainerResource) IsProtected() bool      { return c.category == CategoryProtected }
func (c *ContainerResource) IsSuggested() bool      { return c.category == CategorySuggested }
func (c *ContainerResource) CreatedAt() time.Time   { return c.createdAt }
func (c *ContainerResource) ProtectReason() string  { return c.protectReason }
func (c *ContainerResource) ComposeProject() string { return c.composeProject }

func (c *ContainerResource) DisplayName() string {
	name := strings.TrimPrefix(c.container.Names, "/")
	if len(name) > 20 {
		name = name[:17] + "..."
	}
	return name
}

func (c *ContainerResource) Details() string {
	state := c.container.State
	image := c.container.Image
	if len(image) > 25 {
		image = image[:22] + "..."
	}
	return fmt.Sprintf("%s  %s", state, image)
}

// State returns the container state
func (c *ContainerResource) State() string {
	return c.container.State
}

// Image returns the container image
func (c *ContainerResource) Image() string {
	return c.container.Image
}

// AnalyzeContainers lists and categorizes all containers
func AnalyzeContainers() ([]ContainerResource, error) {
	return AnalyzeContainersWithConfig(config.DefaultConfig())
}

// AnalyzeContainersWithConfig lists and categorizes containers with config options
func AnalyzeContainersWithConfig(cfg *config.Config) ([]ContainerResource, error) {
	containers, err := docker.ListContainers()
	if err != nil {
		return nil, err
	}

	containerIDs := make([]string, 0, len(containers))
	for _, c := range containers {
		if c.ID != "" {
			containerIDs = append(containerIDs, c.ID)
		}
	}

	inspectByID, err := docker.InspectContainers(containerIDs)
	if err != nil {
		inspectByID = make(map[string]*docker.ContainerInspect)
	}

	var results []ContainerResource
	for _, c := range containers {
		labels := make(map[string]string)
		for k, v := range c.Labels {
			labels[k] = v
		}

		// Get detailed info for timestamp
		var createdAt time.Time
		if inspect, ok := inspectByID[c.ID]; ok {
			createdAt = inspect.Created
			// Merge labels from inspect (more complete)
			for k, v := range inspect.Config.Labels {
				labels[k] = v
			}
		} else if inspect, err := docker.InspectContainer(c.ID); err == nil {
			createdAt = inspect.Created
			for k, v := range inspect.Config.Labels {
				labels[k] = v
			}
		}

		// Get compose project if any
		composeProject := docker.ComposeProjectFromLabels(labels)

		// Categorize
		category, protectReason := categorizeContainer(c, labels, cfg)

		// Apply filters
		if cfg.OlderThan > 0 && !createdAt.IsZero() {
			if time.Since(createdAt) < cfg.OlderThan {
				continue // Skip: not old enough
			}
		}

		if cfg.Exited && c.State != "exited" {
			continue // Skip: not exited
		}

		results = append(results, ContainerResource{
			container:      c,
			category:       category,
			labels:         labels,
			createdAt:      createdAt,
			composeProject: composeProject,
			protectReason:  protectReason,
		})
	}

	return results, nil
}

func categorizeContainer(c docker.Container, labels map[string]string, cfg *config.Config) (Category, string) {
	// Check protection label
	if labels[docker.LabelProtect] == "true" {
		return CategoryProtected, "protected by label"
	}

	// Check state
	switch c.State {
	case "running":
		return CategoryProtected, "running"
	case "paused":
		return CategoryProtected, "paused"
	case "restarting":
		return CategoryProtected, "restarting"
	case "exited", "dead", "created":
		return CategorySuggested, ""
	default:
		return CategoryUnused, ""
	}
}
