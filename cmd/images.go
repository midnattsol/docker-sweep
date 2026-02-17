package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/midnattsol/docker-sweep/internal/docker"
	"github.com/midnattsol/docker-sweep/internal/sweep"
	"github.com/midnattsol/docker-sweep/internal/ui"
)

func NewImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "images",
		Aliases: []string{"i", "image"},
		Short:   "Clean up images",
		RunE:    runImages,
	}

	cmd.Flags().StringVar(&flagMinSize, "min-size", "", "Only images larger than size (e.g., 100MB, 1GB)")
	cmd.Flags().BoolVar(&flagDangling, "dangling", false, "Only dangling images")
	cmd.Flags().BoolVar(&flagNoDangling, "no-dangling", false, "Exclude dangling images")

	return cmd
}

func runImages(cmd *cobra.Command, args []string) error {
	if err := validateTypeSpecificFlags(false, true, false, false); err != nil {
		fmt.Print(ui.RenderError(err.Error()))
		return err
	}

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

	var images []sweep.ImageResource
	if err := ui.RunWithSpinner("Analyzing images...", func() error {
		var err error
		images, err = sweep.AnalyzeImagesWithConfig(cfg)
		return err
	}); err != nil {
		if err.Error() == "cancelled" {
			return nil
		}
		fmt.Print(ui.RenderError(err.Error()))
		return err
	}

	if len(images) == 0 {
		fmt.Print(ui.RenderNoResources())
		return nil
	}

	result := &sweep.Result{Images: images}

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
	if err := ui.RunWithSpinner("Deleting images...", func() error {
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
