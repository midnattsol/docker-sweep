package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/midnattsol/docker-sweep/internal/config"
	"github.com/midnattsol/docker-sweep/internal/docker"
	"github.com/midnattsol/docker-sweep/internal/sweep"
	"github.com/midnattsol/docker-sweep/internal/ui"
)

var (
	flagYes        bool
	flagDryRun     bool
	flagVersion    bool
	flagOlderThan  string
	flagMinSize    string
	flagDangling   bool
	flagNoDangling bool
	flagGC         bool
	flagExited     bool
	flagAnonymous  bool

	flagContainers bool
	flagImages     bool
	flagVolumes    bool
	flagNetworks   bool
)

func NewRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker-sweep",
		Short: "Interactive Docker resource cleanup",
		Long: `docker-sweep analyzes Docker resources and opens an interactive picker.

Suggested resources are pre-selected (stopped containers, unused volumes and
networks). Dangling images are excluded by default from root sweeps.
Use --dangling to target dangling images, --gc for automatic cleanup, or --yes
to skip interaction and delete all suggested resources.

Resources with the label sweep.protect=true are never deleted.`,
		RunE:         runRoot,
		SilenceUsage: true,
	}
	cmd.Version = version

	// Global flags
	cmd.PersistentFlags().BoolVarP(&flagYes, "yes", "y", false, "Skip interaction and delete all suggested resources")
	cmd.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "Show what would be deleted without deleting")
	cmd.PersistentFlags().BoolVarP(&flagVersion, "version", "V", false, "Show version")
	cmd.PersistentFlags().StringVar(&flagOlderThan, "older-than", "", "Only resources older than duration (e.g., 7d, 24h, 1w)")
	cmd.PersistentFlags().BoolVarP(&flagContainers, "containers", "c", false, "Only include containers")
	cmd.PersistentFlags().BoolVarP(&flagImages, "images", "i", false, "Only include images")
	cmd.PersistentFlags().BoolVarP(&flagNetworks, "networks", "n", false, "Only include networks")
	cmd.PersistentFlags().BoolVarP(&flagVolumes, "volumes", "v", false, "Only include volumes")

	// Type-specific flags (only on root)
	cmd.Flags().StringVar(&flagMinSize, "min-size", "", "Only images larger than size (e.g., 100MB, 1GB)")
	cmd.Flags().BoolVar(&flagDangling, "dangling", false, "Only dangling images")
	cmd.Flags().BoolVar(&flagNoDangling, "no-dangling", false, "Exclude dangling images")
	cmd.Flags().BoolVar(&flagGC, "gc", false, "Non-interactive garbage collection mode (implies --yes and includes dangling images)")
	cmd.Flags().BoolVar(&flagExited, "exited", false, "Only exited containers")
	cmd.Flags().BoolVar(&flagAnonymous, "anonymous", false, "Only anonymous volumes")

	// Subcommands
	cmd.AddCommand(NewContainersCmd())
	cmd.AddCommand(NewImagesCmd())
	cmd.AddCommand(NewVolumesCmd())
	cmd.AddCommand(NewNetworksCmd())

	return cmd
}

// buildConfig creates a Config from the current flags
func buildConfig() (*config.Config, error) {
	cfg := config.DefaultConfig()
	cfg.Yes = flagYes
	cfg.DryRun = flagDryRun
	cfg.Dangling = flagDangling
	cfg.NoDangling = flagNoDangling
	cfg.Exited = flagExited
	cfg.Anonymous = flagAnonymous

	if flagGC {
		cfg.Yes = true
		cfg.Dangling = false
		cfg.NoDangling = false
	} else if !flagDangling && !flagNoDangling {
		// Default policy for root sweeps: hide dangling images unless requested.
		cfg.NoDangling = true
	}

	if flagOlderThan != "" {
		d, err := config.ParseDuration(flagOlderThan)
		if err != nil {
			return nil, err
		}
		cfg.OlderThan = d
	}

	if flagMinSize != "" {
		s, err := config.ParseSize(flagMinSize)
		if err != nil {
			return nil, err
		}
		cfg.MinSize = s
	}

	return cfg, nil
}

func Execute(version string) {
	if err := NewRootCmd(version).Execute(); err != nil {
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, args []string) error {
	if flagVersion {
		fmt.Fprintln(cmd.OutOrStdout(), cmd.Root().Version)
		return nil
	}

	selectedTypes := flagContainers || flagImages || flagVolumes || flagNetworks
	analyzeContainers := flagContainers || !selectedTypes
	analyzeImages := flagImages || !selectedTypes
	analyzeVolumes := flagVolumes || !selectedTypes
	analyzeNetworks := flagNetworks || !selectedTypes

	if err := validateTypeSpecificFlags(analyzeContainers, analyzeImages, analyzeVolumes, analyzeNetworks); err != nil {
		fmt.Print(ui.RenderError(err.Error()))
		return err
	}

	// Build config from flags
	cfg, err := buildConfig()
	if err != nil {
		fmt.Print(ui.RenderError(err.Error()))
		return err
	}

	// Check Docker is available
	if err := docker.CheckAvailable(); err != nil {
		fmt.Print(ui.RenderError(err.Error()))
		return err
	}

	fmt.Print(ui.RenderHeader())

	// Analyze all resources
	ms := ui.NewMultiSpinner()

	result := &sweep.Result{}

	if analyzeContainers {
		ms.Add("Analyzing containers...", func() error {
			containers, err := sweep.AnalyzeContainersWithConfig(cfg)
			if err != nil {
				return err
			}
			result.Containers = containers
			return nil
		})
	}

	if analyzeImages {
		ms.Add("Analyzing images...", func() error {
			images, err := sweep.AnalyzeImagesWithConfig(cfg)
			if err != nil {
				return err
			}
			result.Images = images
			return nil
		})
	}

	if analyzeVolumes {
		ms.Add("Analyzing volumes...", func() error {
			volumes, err := sweep.AnalyzeVolumesWithConfig(cfg)
			if err != nil {
				return err
			}
			result.Volumes = volumes
			return nil
		})
	}

	if analyzeNetworks {
		ms.Add("Analyzing networks...", func() error {
			networks, err := sweep.AnalyzeNetworksWithConfig(cfg)
			if err != nil {
				return err
			}
			result.Networks = networks
			return nil
		})
	}

	if err := ms.Run(); err != nil {
		if err.Error() == "cancelled" {
			return nil
		}
		fmt.Print(ui.RenderError(err.Error()))
		return err
	}

	// Check if there's anything to clean
	if result.IsEmpty() {
		fmt.Print(ui.RenderNoResources())
		return nil
	}

	var toDelete []sweep.Resource

	if flagYes || flagGC {
		// Non-interactive: delete all suggested
		toDelete = result.Suggested()
	} else {
		if !ui.IsTTY() {
			err := fmt.Errorf("interactive mode requires a terminal; use --yes to delete suggested resources")
			fmt.Print(ui.RenderError(err.Error()))
			return err
		}

		// Interactive picker
		var err error
		toDelete, err = ui.RunPicker(result)
		if err != nil {
			fmt.Print(ui.RenderError(err.Error()))
			return err
		}

		if toDelete == nil {
			// User cancelled
			return nil
		}
	}

	if len(toDelete) == 0 {
		fmt.Print(ui.RenderNoResources())
		return nil
	}

	if flagDryRun {
		fmt.Print(ui.RenderDryRun(toDelete))
		return nil
	}

	var deleted int
	var errors []error
	if err := ui.RunWithSpinner("Deleting selected resources...", func() error {
		deleted, errors = sweep.DeleteResources(toDelete)
		return nil
	}); err != nil {
		if err.Error() == "cancelled" {
			return nil
		}
		fmt.Print(ui.RenderError(err.Error()))
		return err
	}

	// Show errors if any
	for _, err := range errors {
		fmt.Printf("  %s\n", ui.RenderErrorInline(err.Error()))
	}

	fmt.Print(ui.RenderSummary(deleted, len(toDelete)))

	return nil
}

func validateTypeSpecificFlags(includeContainers, includeImages, includeVolumes, includeNetworks bool) error {
	if flagExited && !includeContainers {
		return fmt.Errorf("--exited only applies to containers; include --containers or -c")
	}

	if flagMinSize != "" && !includeImages {
		return fmt.Errorf("--min-size only applies to images; include --images or -i")
	}

	if flagDangling && !includeImages {
		return fmt.Errorf("--dangling only applies to images; include --images or -i")
	}

	if flagNoDangling && !includeImages {
		return fmt.Errorf("--no-dangling only applies to images; include --images or -i")
	}

	if flagDangling && flagNoDangling {
		return fmt.Errorf("--dangling and --no-dangling are mutually exclusive")
	}

	if flagGC && flagDangling {
		return fmt.Errorf("--gc and --dangling are mutually exclusive")
	}

	if flagGC && flagNoDangling {
		return fmt.Errorf("--gc and --no-dangling are mutually exclusive")
	}

	if flagAnonymous && !includeVolumes {
		return fmt.Errorf("--anonymous only applies to volumes; include --volumes or -v")
	}

	return nil
}
