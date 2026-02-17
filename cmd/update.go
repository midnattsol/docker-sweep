package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/midnattsol/docker-sweep/internal/ui"
	"github.com/midnattsol/docker-sweep/internal/update"
)

var (
	flagCheckUpdate bool
	flagYesUpdate   bool
)

func NewUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update docker-sweep to the latest version",
		Long: `Check for and install updates to docker-sweep.

Examples:
  docker sweep update         # Check and prompt to update
  docker sweep update --check # Only check, don't install
  docker sweep update --yes   # Update without confirmation`,
		RunE: runUpdate,
	}

	cmd.Flags().BoolVar(&flagCheckUpdate, "check", false, "Only check for updates, don't install")
	cmd.Flags().BoolVar(&flagYesUpdate, "yes", false, "Update without confirmation")

	return cmd
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	fmt.Printf("\n  %s Current version: %s\n", ui.CheckStyle.Render(), ui.BoldStyle.Render(update.CurrentVersion))

	var release *update.Release
	var hasUpdate bool
	if err := ui.RunWithSpinner("Checking for updates...", func() error {
		var err error
		release, hasUpdate, err = update.CheckForUpdate(ctx)
		return err
	}); err != nil {
		fmt.Print(ui.RenderError(err.Error()))
		return err
	}

	if !hasUpdate {
		fmt.Printf("\n  %s You're on the latest version\n\n", ui.CheckStyle.Render())
		return nil
	}

	fmt.Printf("\n  %s New version available: %s\n\n", ui.WarningStyle.Render("●"), ui.SuccessStyle.Render(release.TagName))

	if flagCheckUpdate {
		fmt.Printf("  Run %s to update.\n\n", ui.BoldStyle.Render("docker sweep update"))
		return nil
	}

	if !flagYesUpdate {
		fmt.Print("  Do you want to update? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Printf("\n  %s Update cancelled\n\n", ui.MutedStyle.Render("●"))
			return nil
		}
		fmt.Println()
	}

	downloadURL, err := release.GetAssetForPlatform()
	if err != nil {
		fmt.Print(ui.RenderError(err.Error()))
		return err
	}

	if err := ui.RunWithSpinner(fmt.Sprintf("Downloading %s...", release.TagName), func() error {
		return update.DownloadAndInstall(ctx, downloadURL)
	}); err != nil {
		msg := err.Error()
		if strings.Contains(strings.ToLower(msg), "permission denied") {
			msg += "\n  Hint: if installed as Docker plugin, ensure write access to ~/.docker/cli-plugins/docker-sweep"
		}
		fmt.Print(ui.RenderError(msg))
		return err
	}

	fmt.Printf("\n  %s Updated to %s\n\n", ui.CheckStyle.Render(), ui.SuccessStyle.Render(release.TagName))
	return nil
}
