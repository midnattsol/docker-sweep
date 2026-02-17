package sweep

import (
	"time"

	"github.com/midnattsol/docker-sweep/internal/config"
	"github.com/midnattsol/docker-sweep/internal/docker"
)

// VolumeResource represents an analyzed volume
type VolumeResource struct {
	volume         docker.Volume
	category       Category
	inUse          bool
	labels         map[string]string
	createdAt      time.Time
	composeProject string
	protectReason  string
}

// Implement Resource interface
func (v *VolumeResource) ID() string             { return v.volume.Name }
func (v *VolumeResource) Type() ResourceType     { return TypeVolume }
func (v *VolumeResource) Category() Category     { return v.category }
func (v *VolumeResource) Size() int64            { return 0 } // Volume size requires filesystem access
func (v *VolumeResource) IsProtected() bool      { return v.category == CategoryProtected }
func (v *VolumeResource) IsSuggested() bool      { return v.category == CategorySuggested }
func (v *VolumeResource) CreatedAt() time.Time   { return v.createdAt }
func (v *VolumeResource) ProtectReason() string  { return v.protectReason }
func (v *VolumeResource) ComposeProject() string { return v.composeProject }

func (v *VolumeResource) DisplayName() string {
	name := v.volume.Name
	if len(name) > 30 {
		name = name[:27] + "..."
	}
	return name
}

func (v *VolumeResource) Details() string {
	if v.inUse {
		return "in use"
	}
	if docker.IsAnonymousVolume(v.volume.Name) {
		return "anonymous"
	}
	return "unused"
}

// IsAnonymous returns true if this is an anonymous volume
func (v *VolumeResource) IsAnonymous() bool {
	return docker.IsAnonymousVolume(v.volume.Name)
}

// AnalyzeVolumes lists and categorizes all volumes
func AnalyzeVolumes() ([]VolumeResource, error) {
	return AnalyzeVolumesWithConfig(config.DefaultConfig())
}

// AnalyzeVolumesWithConfig lists and categorizes volumes with config options
func AnalyzeVolumesWithConfig(cfg *config.Config) ([]VolumeResource, error) {
	volumes, err := docker.ListVolumes()
	if err != nil {
		return nil, err
	}

	volumeNames := make([]string, 0, len(volumes))
	for _, vol := range volumes {
		if vol.Name != "" {
			volumeNames = append(volumeNames, vol.Name)
		}
	}

	inspectByName, err := docker.InspectVolumes(volumeNames)
	if err != nil {
		inspectByName = make(map[string]*docker.VolumeInspect)
	}

	inUse, err := docker.GetVolumesInUse()
	if err != nil {
		// Non-fatal, continue without in-use info
		inUse = make(map[string]bool)
	}

	var results []VolumeResource
	for _, vol := range volumes {
		used := inUse[vol.Name]

		// Get detailed info
		var labels map[string]string
		var createdAt time.Time
		var composeProject string
		if inspect, ok := inspectByName[vol.Name]; ok {
			labels = inspect.Labels
			if t, err := time.Parse(time.RFC3339Nano, inspect.CreatedAt); err == nil {
				createdAt = t
			}
			composeProject = docker.ComposeProjectFromLabels(labels)
		} else if inspect, err := docker.InspectVolume(vol.Name); err == nil {
			labels = inspect.Labels
			if t, err := time.Parse(time.RFC3339Nano, inspect.CreatedAt); err == nil {
				createdAt = t
			}
			composeProject = docker.ComposeProjectFromLabels(labels)
		}

		// Apply filters
		if cfg.OlderThan > 0 && !createdAt.IsZero() {
			if time.Since(createdAt) < cfg.OlderThan {
				continue // Skip: not old enough
			}
		}

		if cfg.Anonymous {
			if !docker.IsAnonymousVolume(vol.Name) {
				continue // Skip: not anonymous
			}
		}

		category, protectReason := categorizeVolume(vol, used, labels, cfg)

		results = append(results, VolumeResource{
			volume:         vol,
			category:       category,
			inUse:          used,
			labels:         labels,
			createdAt:      createdAt,
			composeProject: composeProject,
			protectReason:  protectReason,
		})
	}

	return results, nil
}

func categorizeVolume(vol docker.Volume, inUse bool, labels map[string]string, cfg *config.Config) (Category, string) {
	// Check protection label
	if labels != nil && labels[docker.LabelProtect] == "true" {
		return CategoryProtected, "protected by label"
	}

	if inUse {
		return CategoryProtected, "mounted by container"
	}

	// Anonymous volumes are suggested for deletion
	if docker.IsAnonymousVolume(vol.Name) {
		return CategorySuggested, ""
	}

	// Named volumes are just unused
	return CategoryUnused, ""
}
