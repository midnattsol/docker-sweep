package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Protection labels - resources with these labels are protected from deletion
const (
	LabelProtect        = "sweep.protect"              // "true" to protect
	LabelComposeProject = "com.docker.compose.project" // Docker Compose project name
	LabelPodmanProject  = "io.podman.compose.project"  // Podman Compose project name
)

// ComposeProjectFromLabels returns the compose project label value if present.
func ComposeProjectFromLabels(labels map[string]string) string {
	if labels == nil {
		return ""
	}
	if project := labels[LabelComposeProject]; project != "" {
		return project
	}
	return labels[LabelPodmanProject]
}

var cliRuntime = "docker"

// Runtime returns the currently selected container CLI runtime.
func Runtime() string {
	return cliRuntime
}

// InitRuntime selects the runtime in this priority order:
// 1. DOCKER_SWEEP_RUNTIME env var (docker|podman)
// 2. Invoked binary name contains "podman"
// 3. Auto-detect: docker first, then podman
func InitRuntime(invokedPath string) error {
	envRuntime := strings.ToLower(strings.TrimSpace(os.Getenv("DOCKER_SWEEP_RUNTIME")))
	if envRuntime != "" {
		switch envRuntime {
		case "docker", "podman":
			cliRuntime = envRuntime
			return nil
		default:
			return fmt.Errorf("invalid DOCKER_SWEEP_RUNTIME value %q (expected docker or podman)", envRuntime)
		}
	}

	binary := strings.ToLower(filepath.Base(invokedPath))
	if strings.Contains(binary, "podman") {
		cliRuntime = "podman"
		return nil
	}

	if probeRuntime("docker") {
		cliRuntime = "docker"
		return nil
	}

	if probeRuntime("podman") {
		cliRuntime = "podman"
		return nil
	}

	// Default keeps current Docker-first UX and emits a clearer error later.
	cliRuntime = "docker"
	return nil
}

func probeRuntime(runtime string) bool {
	cmd := exec.Command(runtime, "version")
	return cmd.Run() == nil
}

// CheckAvailable checks if the selected runtime CLI is available.
func CheckAvailable() error {
	cmd := exec.Command(cliRuntime, "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s is not available: %w", cliRuntime, err)
	}
	return nil
}

// Run executes a runtime command and returns stdout.
func Run(args ...string) ([]byte, error) {
	cmd := exec.Command(cliRuntime, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %s: %s", cliRuntime, strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// RunJSON executes a docker command and parses JSON output (line-delimited)
func RunJSON[T any](args ...string) ([]T, error) {
	out, err := Run(args...)
	if err != nil {
		return nil, err
	}

	var results []T
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var item T
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		results = append(results, item)
	}

	return results, nil
}

// Remove removes a docker resource
func Remove(resourceType, id string) error {
	var args []string
	switch resourceType {
	case "container":
		args = []string{"rm", id}
	case "image":
		args = []string{"rmi", id}
	case "volume":
		args = []string{"volume", "rm", id}
	case "network":
		args = []string{"network", "rm", id}
	default:
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}

	_, err := Run(args...)
	return err
}
