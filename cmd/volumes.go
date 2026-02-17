package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/midnattsol/docker-sweep/internal/docker"
	"github.com/midnattsol/docker-sweep/internal/sweep"
	"github.com/midnattsol/docker-sweep/internal/ui"
)

func NewVolumesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "volumes",
		Aliases: []string{"v", "volume"},
		Short:   "Clean up volumes",
		RunE:    runVolumes,
	}

	cmd.Flags().BoolVar(&flagAnonymous, "anonymous", false, "Only anonymous volumes")

	return cmd
}

func runVolumes(cmd *cobra.Command, args []string) error {
	cfg, err := buildConfig()
	if err != nil {
		fmt.Print(ui.RenderError(err.Error()))
		return err
	}

	if err := docker.CheckAvailable(); err != nil {
		fmt.Print(ui.RenderError(err.Error()))
		return err
	}

	fmt.Print(ui.RenderHeader())

	var volumes []sweep.VolumeResource
	if err := ui.RunWithSpinner("Analyzing volumes...", func() error {
		var err error
		volumes, err = sweep.AnalyzeVolumesWithConfig(cfg)
		return err
	}); err != nil {
		if err.Error() == "cancelled" {
			return nil
		}
		fmt.Print(ui.RenderError(err.Error()))
		return err
	}

	if len(volumes) == 0 {
		fmt.Print(ui.RenderNoResources())
		return nil
	}

	result := &sweep.Result{Volumes: volumes}

	var toDelete []sweep.Resource

	if flagYes {
		toDelete = result.Suggested()
	} else {
		if !ui.IsTTY() {
			err := fmt.Errorf("interactive mode requires a terminal; use --yes")
			fmt.Print(ui.RenderError(err.Error()))
			return err
		}

		var err error
		toDelete, err = ui.RunPicker(result)
		if err != nil {
			fmt.Print(ui.RenderError(err.Error()))
			return err
		}
		if toDelete == nil {
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
	if err := ui.RunWithSpinner("Deleting volumes...", func() error {
		deleted, errors = sweep.DeleteResources(toDelete)
		return nil
	}); err != nil {
		if err.Error() == "cancelled" {
			return nil
		}
		fmt.Print(ui.RenderError(err.Error()))
		return err
	}

	for _, err := range errors {
		fmt.Printf("  %s\n", ui.RenderErrorInline(err.Error()))
	}

	fmt.Print(ui.RenderSummary(deleted, len(toDelete)))
	return nil
}
