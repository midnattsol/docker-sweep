package docker

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Image represents a Docker image from `docker images`
type Image struct {
	ID         string `json:"ID"`
	Repository string `json:"Repository"`
	Tag        string `json:"Tag"`
	CreatedAt  string `json:"CreatedAt"`
	Size       string `json:"Size"`

	// Parsed metadata from list output when available.
	SizeBytes     int64             `json:"-"`
	HasSize       bool              `json:"-"`
	CreatedAtTime time.Time         `json:"-"`
	HasCreatedAt  bool              `json:"-"`
	ListLabels    map[string]string `json:"-"`
	HasListLabels bool              `json:"-"`
}

// UnmarshalJSON supports both Docker and Podman output shapes.
func (i *Image) UnmarshalJSON(data []byte) error {
	raw, err := decodeJSONMap(data)
	if err != nil {
		return err
	}

	i.ID = pickString(raw, "ID", "Id", "id")
	i.Repository = pickString(raw, "Repository", "repository")
	i.Tag = pickString(raw, "Tag", "tag")
	i.CreatedAt = pickString(raw, "CreatedAt", "Created", "created")
	i.Size = pickString(raw, "Size", "size")
	i.ListLabels = parseLabelsRaw(pickRaw(raw, "Labels", "labels"))
	i.HasListLabels = pickRaw(raw, "Labels", "labels") != nil

	if sizeRaw := pickRaw(raw, "Size", "size"); sizeRaw != nil {
		i.HasSize = true
		if parsed, ok := parseSizeRaw(sizeRaw); ok {
			i.SizeBytes = parsed
		}
	}

	if createdRaw := pickRaw(raw, "Created", "created", "CreatedAt", "createdAt"); createdRaw != nil {
		i.HasCreatedAt = true
		if t, ok := parseCreatedRaw(createdRaw); ok {
			i.CreatedAtTime = t
		}
	}

	if i.Repository == "" {
		i.Repository = "<none>"
	}
	if i.Tag == "" {
		i.Tag = "<none>"
	}

	return nil
}

func parseSizeRaw(raw json.RawMessage) (int64, bool) {
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return int64(n), true
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return parseHumanSizeToBytes(s)
	}

	return 0, false
}

func parseHumanSizeToBytes(s string) (int64, bool) {
	s = strings.TrimSpace(strings.ToUpper(s))
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return 0, false
	}

	units := []struct {
		suffix string
		mul    float64
	}{
		{"TB", 1024 * 1024 * 1024 * 1024},
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
		{"B", 1},
	}

	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			num := strings.TrimSuffix(s, u.suffix)
			f, err := strconv.ParseFloat(num, 64)
			if err != nil {
				return 0, false
			}
			return int64(f * u.mul), true
		}
	}

	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f), true
	}

	return 0, false
}

func parseCreatedRaw(raw json.RawMessage) (time.Time, bool) {
	var unixSec int64
	if err := json.Unmarshal(raw, &unixSec); err == nil {
		return time.Unix(unixSec, 0), true
	}

	var unixF float64
	if err := json.Unmarshal(raw, &unixF); err == nil {
		return time.Unix(int64(unixF), 0), true
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" {
			return time.Time{}, false
		}

		layouts := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05 -0700 MST",
			"2006-01-02 15:04:05 -0700",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, s); err == nil {
				return t, true
			}
		}

		if sec, err := strconv.ParseInt(s, 10, 64); err == nil {
			return time.Unix(sec, 0), true
		}
	}

	return time.Time{}, false
}

// ListImages returns all images
func ListImages() ([]Image, error) {
	return RunJSON[Image]("images", "-a", "--no-trunc", "--format", "{{json .}}")
}

// ImageInUse represents which containers use which images
type ImageUsage struct {
	ImageID     string
	ContainerID string
}

// GetImagesInUse returns a set of image IDs that are in use by containers
func GetImagesInUse() (map[string]bool, error) {
	// Get all containers (including stopped) and their image names
	out, err := Run("ps", "-a", "--format", "{{.Image}}")
	if err != nil {
		return nil, err
	}

	inUse := make(map[string]bool)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line != "" {
			inUse[line] = true
		}
	}

	// Also get container IDs and inspect their image IDs in one batch call
	out, err = Run("ps", "-a", "--no-trunc", "--format", "{{.ID}}")
	if err != nil {
		return nil, err
	}

	containerIDs := strings.Split(strings.TrimSpace(string(out)), "\n")
	var ids []string
	for _, cid := range containerIDs {
		if cid != "" {
			ids = append(ids, cid)
		}
	}

	if len(ids) == 0 {
		return inUse, nil
	}

	inspectOut, err := Run(append([]string{"inspect", "--format", "{{.Image}}"}, ids...)...)
	if err != nil {
		return inUse, nil // non-fatal, keep what we already have from image names
	}

	for _, line := range strings.Split(strings.TrimSpace(string(inspectOut)), "\n") {
		imageID := NormalizeImageID(strings.TrimSpace(line))
		if imageID != "" {
			inUse[imageID] = true
		}
	}

	return inUse, nil
}

// ImageInspect returns detailed info about an image
type ImageInspect struct {
	ID      string            `json:"Id"`
	Size    int64             `json:"Size"`
	Created string            `json:"Created"`
	Labels  map[string]string `json:"Labels"`
	Config  struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

// NormalizeImageID removes known prefixes from an image ID.
func NormalizeImageID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "sha256:")
	return id
}

func InspectImage(id string) (*ImageInspect, error) {
	out, err := Run("inspect", "--format", "{{json .}}", id)
	if err != nil {
		return nil, err
	}

	var inspect ImageInspect
	if err := json.Unmarshal(out, &inspect); err != nil {
		return nil, err
	}

	// Labels can be in .Labels or .Config.Labels depending on Docker version
	if inspect.Labels == nil {
		inspect.Labels = inspect.Config.Labels
	}

	return &inspect, nil
}

// InspectImages inspects many images in batches for better performance.
func InspectImages(ids []string) (map[string]*ImageInspect, error) {
	result := make(map[string]*ImageInspect)
	if len(ids) == 0 {
		return result, nil
	}

	const batchSize = 100
	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}

		batch := ids[start:end]
		out, err := Run(append([]string{"inspect"}, batch...)...)
		if err != nil {
			return nil, err
		}

		var inspected []ImageInspect
		if err := json.Unmarshal(out, &inspected); err != nil {
			return nil, fmt.Errorf("failed to parse inspect output: %w", err)
		}

		for i := range inspected {
			item := inspected[i]
			if item.Labels == nil {
				item.Labels = item.Config.Labels
			}

			key := NormalizeImageID(item.ID)
			if key == "" {
				continue
			}
			copyItem := item
			result[key] = &copyItem
		}
	}

	return result, nil
}
