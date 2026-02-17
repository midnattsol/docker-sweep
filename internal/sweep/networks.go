package sweep

import (
	"time"

	"github.com/midnattsol/docker-sweep/internal/config"
	"github.com/midnattsol/docker-sweep/internal/docker"
)

// NetworkResource represents an analyzed network
type NetworkResource struct {
	network        docker.Network
	category       Category
	inUse          bool
	labels         map[string]string
	createdAt      time.Time
	composeProject string
	protectReason  string
}

// Implement Resource interface
func (n *NetworkResource) ID() string             { return n.network.ID }
func (n *NetworkResource) Type() ResourceType     { return TypeNetwork }
func (n *NetworkResource) Category() Category     { return n.category }
func (n *NetworkResource) Size() int64            { return 0 }
func (n *NetworkResource) IsProtected() bool      { return n.category == CategoryProtected }
func (n *NetworkResource) IsSuggested() bool      { return n.category == CategorySuggested }
func (n *NetworkResource) CreatedAt() time.Time   { return n.createdAt }
func (n *NetworkResource) ProtectReason() string  { return n.protectReason }
func (n *NetworkResource) ComposeProject() string { return n.composeProject }

func (n *NetworkResource) DisplayName() string {
	name := n.network.Name
	if len(name) > 30 {
		name = name[:27] + "..."
	}
	return name
}

func (n *NetworkResource) Details() string {
	if docker.SystemNetworks[n.network.Name] {
		return "system"
	}
	if n.inUse {
		return "in use"
	}
	return n.network.Driver
}

// Driver returns the network driver
func (n *NetworkResource) Driver() string {
	return n.network.Driver
}

// AnalyzeNetworks lists and categorizes all networks
func AnalyzeNetworks() ([]NetworkResource, error) {
	return AnalyzeNetworksWithConfig(config.DefaultConfig())
}

// AnalyzeNetworksWithConfig lists and categorizes networks with config options
func AnalyzeNetworksWithConfig(cfg *config.Config) ([]NetworkResource, error) {
	networks, err := docker.ListNetworks()
	if err != nil {
		return nil, err
	}

	inUse, err := docker.GetNetworksInUse()
	if err != nil {
		// Non-fatal, continue without in-use info
		inUse = make(map[string]bool)
	}

	var results []NetworkResource
	for _, net := range networks {
		used := inUse[net.Name]

		// Get detailed info
		var labels map[string]string
		var createdAt time.Time
		var composeProject string
		if inspect, err := docker.InspectNetwork(net.ID); err == nil {
			labels = inspect.Labels
			if t, err := time.Parse(time.RFC3339Nano, inspect.Created); err == nil {
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

		category, protectReason := categorizeNetwork(net, used, labels, cfg)

		results = append(results, NetworkResource{
			network:        net,
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

func categorizeNetwork(net docker.Network, inUse bool, labels map[string]string, cfg *config.Config) (Category, string) {
	// Check protection label
	if labels != nil && labels[docker.LabelProtect] == "true" {
		return CategoryProtected, "protected by label"
	}

	// System networks are always protected
	if docker.SystemNetworks[net.Name] {
		return CategoryProtected, "system network"
	}

	if inUse {
		return CategoryProtected, "in use by container"
	}

	// Unused custom networks are suggested
	return CategorySuggested, ""
}
