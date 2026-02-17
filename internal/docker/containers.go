package docker

import (
	"encoding/json"
	"strings"
	"time"
)

// Container represents a Docker container from `docker ps -a`
type Container struct {
	ID        string            `json:"ID"`
	Names     string            `json:"Names"`
	Image     string            `json:"Image"`
	State     string            `json:"State"`
	Status    string            `json:"Status"`
	CreatedAt time.Time         `json:"CreatedAt"`
	Size      string            `json:"Size"`
	Labels    map[string]string `json:"Labels"`
}

// UnmarshalJSON supports both Docker and Podman output shapes.
func (c *Container) UnmarshalJSON(data []byte) error {
	raw, err := decodeJSONMap(data)
	if err != nil {
		return err
	}

	c.ID = pickString(raw, "ID", "Id", "id")
	c.Names = parseNameField(pickRaw(raw, "Names", "names"))
	c.Image = pickString(raw, "Image", "image")
	c.State = pickString(raw, "State", "state")
	c.Status = pickString(raw, "Status", "status")
	c.Size = pickString(raw, "Size", "size")
	c.Labels = parseLabelsRaw(pickRaw(raw, "Labels", "labels"))

	createdAt := pickString(raw, "CreatedAt", "createdAt")
	if createdAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
			c.CreatedAt = t
		}
	}

	return nil
}

// ListContainers returns all containers
func ListContainers() ([]Container, error) {
	return RunJSON[Container]("ps", "-a", "--no-trunc", "--format", "{{json .}}")
}

// ContainerInspect holds detailed container info
type ContainerInspect struct {
	ID      string    `json:"Id"`
	Created time.Time `json:"Created"`
	Config  struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

// InspectContainer returns detailed info about a container
func InspectContainer(id string) (*ContainerInspect, error) {
	out, err := Run("inspect", "--format", "{{json .}}", id)
	if err != nil {
		return nil, err
	}

	var inspect ContainerInspect
	if err := json.Unmarshal(out, &inspect); err != nil {
		return nil, err
	}

	return &inspect, nil
}

// InspectContainers inspects many containers in batches for better performance.
func InspectContainers(ids []string) (map[string]*ContainerInspect, error) {
	result := make(map[string]*ContainerInspect)
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

		var inspected []ContainerInspect
		if err := json.Unmarshal(out, &inspected); err != nil {
			return nil, err
		}

		for i := range inspected {
			item := inspected[i]
			if item.ID == "" {
				continue
			}
			copyItem := item
			result[item.ID] = &copyItem
		}
	}

	return result, nil
}

// ParseLabels parses the comma-separated labels string into a map
func ParseLabels(labels string) map[string]string {
	result := make(map[string]string)
	if labels == "" {
		return result
	}

	pairs := strings.Split(labels, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}
