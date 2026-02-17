package docker

import (
	"encoding/json"
	"strings"
)

// Volume represents a Docker volume from `docker volume ls`
type Volume struct {
	Name       string `json:"Name"`
	Driver     string `json:"Driver"`
	Mountpoint string `json:"Mountpoint"`
}

// UnmarshalJSON supports both Docker and Podman output shapes.
func (v *Volume) UnmarshalJSON(data []byte) error {
	raw, err := decodeJSONMap(data)
	if err != nil {
		return err
	}

	v.Name = pickString(raw, "Name", "name")
	v.Driver = pickString(raw, "Driver", "driver")
	v.Mountpoint = pickString(raw, "Mountpoint", "mountpoint")

	return nil
}

// ListVolumes returns all volumes
func ListVolumes() ([]Volume, error) {
	return RunJSON[Volume]("volume", "ls", "--format", "{{json .}}")
}

// GetVolumesInUse returns a set of volume names that are in use by containers
func GetVolumesInUse() (map[string]bool, error) {
	// Get all containers and their mounts
	out, err := Run("ps", "-a", "--no-trunc", "--format", "{{.ID}}")
	if err != nil {
		return nil, err
	}

	inUse := make(map[string]bool)
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

	inspectOut, err := Run(append([]string{"inspect"}, ids...)...)
	if err != nil {
		return inUse, nil // non-fatal
	}

	var containers []struct {
		Mounts []struct {
			Type   string `json:"Type"`
			Name   string `json:"Name"`
			Source string `json:"Source"`
		} `json:"Mounts"`
	}
	if err := json.Unmarshal(inspectOut, &containers); err != nil {
		return inUse, nil // non-fatal
	}

	for _, c := range containers {
		for _, m := range c.Mounts {
			if m.Type == "volume" && m.Name != "" {
				inUse[m.Name] = true
			}
		}
	}

	return inUse, nil
}

// IsAnonymousVolume checks if a volume name looks like an anonymous volume (64 char hex)
func IsAnonymousVolume(name string) bool {
	if len(name) != 64 {
		return false
	}
	for _, c := range name {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// VolumeInspect holds detailed volume info
type VolumeInspect struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Mountpoint string            `json:"Mountpoint"`
	CreatedAt  string            `json:"CreatedAt"`
	Labels     map[string]string `json:"Labels"`
}

// InspectVolume returns detailed info about a volume
func InspectVolume(name string) (*VolumeInspect, error) {
	out, err := Run("volume", "inspect", "--format", "{{json .}}", name)
	if err != nil {
		return nil, err
	}

	var inspect VolumeInspect
	if err := json.Unmarshal(out, &inspect); err != nil {
		return nil, err
	}

	return &inspect, nil
}

// InspectVolumes inspects many volumes in batches for better performance.
func InspectVolumes(names []string) (map[string]*VolumeInspect, error) {
	result := make(map[string]*VolumeInspect)
	if len(names) == 0 {
		return result, nil
	}

	const batchSize = 100
	for start := 0; start < len(names); start += batchSize {
		end := start + batchSize
		if end > len(names) {
			end = len(names)
		}

		batch := names[start:end]
		out, err := Run(append([]string{"volume", "inspect"}, batch...)...)
		if err != nil {
			return nil, err
		}

		var inspected []VolumeInspect
		if err := json.Unmarshal(out, &inspected); err != nil {
			return nil, err
		}

		for i := range inspected {
			item := inspected[i]
			if item.Name == "" {
				continue
			}
			copyItem := item
			result[item.Name] = &copyItem
		}
	}

	return result, nil
}
