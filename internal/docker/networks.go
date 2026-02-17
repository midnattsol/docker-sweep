package docker

import (
	"encoding/json"
	"strings"
)

// Network represents a Docker network from `docker network ls`
type Network struct {
	ID     string `json:"ID"`
	Name   string `json:"Name"`
	Driver string `json:"Driver"`
	Scope  string `json:"Scope"`
}

// UnmarshalJSON supports both Docker and Podman output shapes.
func (n *Network) UnmarshalJSON(data []byte) error {
	raw, err := decodeJSONMap(data)
	if err != nil {
		return err
	}

	n.ID = pickString(raw, "ID", "Id", "id")
	n.Name = pickString(raw, "Name", "name")
	n.Driver = pickString(raw, "Driver", "driver")
	n.Scope = pickString(raw, "Scope", "scope")

	return nil
}

// ListNetworks returns all networks
func ListNetworks() ([]Network, error) {
	return RunJSON[Network]("network", "ls", "--no-trunc", "--format", "{{json .}}")
}

// SystemNetworks are built-in networks that should not be deleted
var SystemNetworks = map[string]bool{
	"bridge": true,
	"host":   true,
	"none":   true,
	"podman": true,
}

// GetNetworksInUse returns a set of network IDs that are in use by containers
func GetNetworksInUse() (map[string]bool, error) {
	// Get all containers and their networks
	out, err := Run("ps", "-a", "--format", "{{.ID}}")
	if err != nil {
		return nil, err
	}

	inUse := make(map[string]bool)
	containerIDs := strings.Split(strings.TrimSpace(string(out)), "\n")

	for _, cid := range containerIDs {
		if cid == "" {
			continue
		}
		// Get networks for this container
		netOut, err := Run("inspect", "--format", "{{json .NetworkSettings.Networks}}", cid)
		if err != nil {
			continue
		}

		var networks map[string]interface{}
		if err := json.Unmarshal(netOut, &networks); err != nil {
			continue
		}

		for netName := range networks {
			inUse[netName] = true
		}
	}

	return inUse, nil
}

// NetworkInspect holds detailed network info
type NetworkInspect struct {
	ID      string            `json:"Id"`
	Name    string            `json:"Name"`
	Created string            `json:"Created"`
	Driver  string            `json:"Driver"`
	Labels  map[string]string `json:"Labels"`
}

// InspectNetwork returns detailed info about a network
func InspectNetwork(id string) (*NetworkInspect, error) {
	out, err := Run("network", "inspect", "--format", "{{json .}}", id)
	if err != nil {
		return nil, err
	}

	var inspect NetworkInspect
	if err := json.Unmarshal(out, &inspect); err != nil {
		return nil, err
	}

	if inspect.ID == "" || inspect.Name == "" || inspect.Driver == "" || inspect.Created == "" || inspect.Labels == nil {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(out, &raw); err == nil {
			if inspect.ID == "" {
				inspect.ID = pickString(raw, "Id", "ID", "id")
			}
			if inspect.Name == "" {
				inspect.Name = pickString(raw, "Name", "name")
			}
			if inspect.Created == "" {
				inspect.Created = pickString(raw, "Created", "created")
			}
			if inspect.Driver == "" {
				inspect.Driver = pickString(raw, "Driver", "driver")
			}
			if inspect.Labels == nil {
				inspect.Labels = parseLabelsRaw(pickRaw(raw, "Labels", "labels"))
			}
		}
	}

	return &inspect, nil
}
