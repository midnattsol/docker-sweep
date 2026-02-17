package sweep

import (
	"fmt"
	"time"

	"github.com/midnattsol/docker-sweep/internal/config"
	"github.com/midnattsol/docker-sweep/internal/docker"
)

// ImageResource represents an analyzed image
type ImageResource struct {
	image         docker.Image
	category      Category
	inUse         bool
	size          int64
	labels        map[string]string
	createdAt     time.Time
	protectReason string
}

// Implement Resource interface
func (i *ImageResource) ID() string            { return i.image.ID }
func (i *ImageResource) Type() ResourceType    { return TypeImage }
func (i *ImageResource) Category() Category    { return i.category }
func (i *ImageResource) Size() int64           { return i.size }
func (i *ImageResource) IsProtected() bool     { return i.category == CategoryProtected }
func (i *ImageResource) IsSuggested() bool     { return i.category == CategorySuggested }
func (i *ImageResource) CreatedAt() time.Time  { return i.createdAt }
func (i *ImageResource) ProtectReason() string { return i.protectReason }

func (i *ImageResource) DisplayName() string {
	if i.image.Repository == "<none>" {
		// Show short ID for dangling images
		id := i.image.ID
		id = trimImageID(id)
		return fmt.Sprintf("<none>:%s", id)
	}

	name := i.image.Repository
	if i.image.Tag != "<none>" {
		name += ":" + i.image.Tag
	}
	if len(name) > 30 {
		name = name[:27] + "..."
	}
	return name
}

func trimImageID(id string) string {
	// Remove sha256: prefix
	if len(id) > 7 && id[:7] == "sha256:" {
		id = id[7:]
	}
	if len(id) > 12 {
		id = id[:12]
	}
	return id
}

func (i *ImageResource) Details() string {
	status := "unused"
	if i.inUse {
		status = "in use"
	} else if i.image.Repository == "<none>" {
		status = "dangling"
	}
	return status
}

// IsDangling returns true if this is a dangling image
func (i *ImageResource) IsDangling() bool {
	return i.image.Repository == "<none>" && i.image.Tag == "<none>"
}

// AnalyzeImages lists and categorizes all images
func AnalyzeImages() ([]ImageResource, error) {
	return AnalyzeImagesWithConfig(config.DefaultConfig())
}

// AnalyzeImagesWithConfig lists and categorizes images with config options
func AnalyzeImagesWithConfig(cfg *config.Config) ([]ImageResource, error) {
	images, err := docker.ListImages()
	if err != nil {
		return nil, err
	}

	inUse, err := docker.GetImagesInUse()
	if err != nil {
		// Non-fatal, continue without in-use info
		inUse = make(map[string]bool)
	}

	inspectNeeded := make(map[string]bool)
	imageIDs := make([]string, 0, len(images))
	for _, img := range images {
		if id := docker.NormalizeImageID(img.ID); id != "" {
			imageIDs = append(imageIDs, id)

			needsInspect := false
			if cfg.MinSize > 0 && (!img.HasSize || img.SizeBytes == 0) {
				needsInspect = true
			}
			if cfg.OlderThan > 0 && !img.HasCreatedAt {
				needsInspect = true
			}
			if !img.HasListLabels {
				needsInspect = true
			}

			if needsInspect {
				inspectNeeded[id] = true
			}
		}
	}

	inspectByID := make(map[string]*docker.ImageInspect)
	if len(inspectNeeded) > 0 {
		idsToInspect := make([]string, 0, len(inspectNeeded))
		for _, id := range imageIDs {
			if inspectNeeded[id] {
				idsToInspect = append(idsToInspect, id)
			}
		}

		if batchInspect, err := docker.InspectImages(idsToInspect); err == nil {
			inspectByID = batchInspect
		}
	}

	var results []ImageResource
	for _, img := range images {
		// Check if in use by repository:tag or by ID
		normalizedID := docker.NormalizeImageID(img.ID)
		used := inUse[img.Repository+":"+img.Tag] || inUse[normalizedID]

		// Get detailed info
		size := img.SizeBytes
		labels := img.ListLabels
		createdAt := img.CreatedAtTime
		if inspect, ok := inspectByID[normalizedID]; ok {
			size = inspect.Size
			labels = inspect.Labels
			if t, err := time.Parse(time.RFC3339Nano, inspect.Created); err == nil {
				createdAt = t
			}
		} else if inspectNeeded[normalizedID] {
			if inspect, err := docker.InspectImage(img.ID); err == nil {
				size = inspect.Size
				labels = inspect.Labels
				if t, err := time.Parse(time.RFC3339Nano, inspect.Created); err == nil {
					createdAt = t
				}
			}
		}

		if labels == nil {
			labels = make(map[string]string)
		}

		if size == 0 && img.HasSize {
			size = img.SizeBytes
		}

		if createdAt.IsZero() && img.HasCreatedAt {
			createdAt = img.CreatedAtTime
		}

		if createdAt.IsZero() && img.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339Nano, img.CreatedAt); err == nil {
				createdAt = t
			}
		}

		// Apply filters
		if cfg.OlderThan > 0 && !createdAt.IsZero() {
			if time.Since(createdAt) < cfg.OlderThan {
				continue // Skip: not old enough
			}
		}

		if cfg.MinSize > 0 && size < cfg.MinSize {
			continue // Skip: too small
		}

		if cfg.Dangling {
			isDangling := img.Repository == "<none>" && img.Tag == "<none>"
			if !isDangling {
				continue // Skip: not dangling
			}
		}

		if cfg.NoDangling {
			isDangling := img.Repository == "<none>" && img.Tag == "<none>"
			if isDangling {
				continue // Skip: dangling image excluded
			}
		}

		category, protectReason := categorizeImage(img, used, labels, cfg)

		results = append(results, ImageResource{
			image:         img,
			category:      category,
			inUse:         used,
			size:          size,
			labels:        labels,
			createdAt:     createdAt,
			protectReason: protectReason,
		})
	}

	return results, nil
}

func categorizeImage(img docker.Image, inUse bool, labels map[string]string, cfg *config.Config) (Category, string) {
	// Check protection label
	if labels != nil && labels[docker.LabelProtect] == "true" {
		return CategoryProtected, "protected by label"
	}

	if inUse {
		return CategoryProtected, "in use by container"
	}

	// Dangling images (no repo, no tag) are suggested
	if img.Repository == "<none>" && img.Tag == "<none>" {
		return CategorySuggested, ""
	}

	// Images with tags but not in use are just "unused"
	return CategoryUnused, ""
}
