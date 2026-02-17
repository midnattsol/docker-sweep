package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/midnattsol/docker-sweep/internal/docker"
	"github.com/midnattsol/docker-sweep/internal/sweep"
	"github.com/midnattsol/docker-sweep/internal/ui"
)

func NewContainersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "containers",
		Aliases: []string{"c", "container"},
		Short:   "Clean up containers",
		RunE:    runContainers,
	}

	cmd.Flags().BoolVar(&flagExited, "exited", false, "Only exited containers")

	return cmd
}

func runContainers(cmd *cobra.Command, args []string) error {
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

	var containers []sweep.ContainerResource
	if err := ui.RunWithSpinner("Analyzing containers...", func() error {
		var err error
		containers, err = sweep.AnalyzeContainersWithConfig(cfg)
		return err
	}); err != nil {
		if err.Error() == "cancelled" {
			return nil
		}
		fmt.Print(ui.RenderError(err.Error()))
		return err
	}

	if len(containers) == 0 {
		fmt.Print(ui.RenderNoResources())
		return nil
	}

	result := &sweep.Result{Containers: containers}

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
	if err := ui.RunWithSpinner("Deleting containers...", func() error {
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
