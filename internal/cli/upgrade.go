package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/agilercloud/cli/internal/app"
	"github.com/agilercloud/cli/internal/selfupdate"
	"github.com/spf13/cobra"
)

func newUpgradeCmd(a *app.App) *cobra.Command {
	var check, force bool
	var version string

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade the agiler CLI to the latest release",
		Long:  "Check GitHub for a newer release and replace the current binary. Refuses to run for Homebrew or go-install paths and points at the right command instead.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(cmd.Context(), a, check, force, version)
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Check for a newer version without installing")
	cmd.Flags().BoolVar(&force, "force", false, "Proceed even if versions match, the source is a non-canonical install, or this is a dev build")
	cmd.Flags().StringVar(&version, "version", "", "Install a specific release tag (e.g. v0.1.0) instead of the latest")
	return cmd
}

func runUpgrade(ctx context.Context, a *app.App, check, force bool, version string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	exe, err := selfupdate.ResolveExecutable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	source, hint := selfupdate.DetectInstallSource(exe, a.Version)
	if !force {
		switch source {
		case selfupdate.SourceHomebrew, selfupdate.SourceGoInstall, selfupdate.SourceDev:
			fmt.Fprintln(a.Out, hint)
			return nil
		}
	}

	rel, err := selfupdate.FetchRelease(ctx, version)
	if err != nil {
		return err
	}

	current := selfupdate.NormalizeVersion(a.Version)
	latest := selfupdate.NormalizeVersion(rel.TagName)

	if check {
		printCurrentLatest(a, a.Version, rel.TagName)
		cmp := selfupdate.CompareVersions(current, latest)
		switch {
		case cmp < 0:
			fmt.Fprintln(a.Out, "run 'agiler upgrade' to update")
		case cmp == 0:
			fmt.Fprintf(a.Out, "agiler is up to date (%s)\n", rel.TagName)
		default:
			fmt.Fprintln(a.Out, "you are ahead of the latest published release")
		}
		return nil
	}

	if !force && version == "" && selfupdate.CompareVersions(current, latest) >= 0 {
		fmt.Fprintf(a.Out, "agiler is up to date (%s)\n", rel.TagName)
		return nil
	}

	if err := selfupdate.PreflightWritable(exe); err != nil {
		return err
	}

	archive := selfupdate.ArchiveName(rel.TagName, runtime.GOOS, runtime.GOARCH)
	tmpDir, err := os.MkdirTemp("", "agiler-upgrade-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	printCurrentLatest(a, a.Version, rel.TagName)
	fmt.Fprintf(a.Out, "downloading %s\n", archive)
	archivePath, err := selfupdate.DownloadAndVerify(ctx, rel.TagName, archive, tmpDir)
	if err != nil {
		return err
	}
	fmt.Fprintln(a.Out, "verifying sha256... ok")

	newBinary := filepath.Join(tmpDir, "agiler.new")
	if err := selfupdate.ExtractBinary(archivePath, newBinary); err != nil {
		return err
	}

	fmt.Fprintf(a.Out, "installing to %s\n", exe)
	if err := selfupdate.ReplaceExecutable(newBinary, exe); err != nil {
		return err
	}

	fmt.Fprintf(a.Out, "upgraded agiler %s -> %s\n", displayVersion(a.Version), rel.TagName)
	return nil
}

func printCurrentLatest(a *app.App, current, latest string) {
	fmt.Fprintf(a.Out, "current: %s\n", displayVersion(current))
	fmt.Fprintf(a.Out, "latest:  %s\n", latest)
}

func displayVersion(v string) string {
	if selfupdate.NormalizeVersion(v) == "" {
		return "dev"
	}
	return v
}
