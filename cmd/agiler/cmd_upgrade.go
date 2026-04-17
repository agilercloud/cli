package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

var (
	upgradeFlagCheck   bool
	upgradeFlagForce   bool
	upgradeFlagVersion string
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade the agiler CLI to the latest release",
	Long:  "Check GitHub for a newer release and replace the current binary. Refuses to run for Homebrew or go-install paths and points at the right command instead.",
	RunE:  runUpgrade,
}

func init() {
	upgradeCmd.Flags().BoolVar(&upgradeFlagCheck, "check", false, "Check for a newer version without installing")
	upgradeCmd.Flags().BoolVar(&upgradeFlagForce, "force", false, "Proceed even if versions match, the source is a non-canonical install, or this is a dev build")
	upgradeCmd.Flags().StringVar(&upgradeFlagVersion, "version", "", "Install a specific release tag (e.g. v0.1.0) instead of the latest")
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()

	exe, err := resolveExecutable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	source, hint := detectInstallSource(exe, Version)
	if !upgradeFlagForce {
		switch source {
		case sourceHomebrew:
			fmt.Println(hint)
			return nil
		case sourceGoInstall:
			fmt.Println(hint)
			return nil
		case sourceDev:
			fmt.Println(hint)
			return nil
		}
	}

	rel, err := fetchRelease(ctx, upgradeFlagVersion)
	if err != nil {
		return err
	}

	current := normalizeVersion(Version)
	latest := normalizeVersion(rel.TagName)

	if upgradeFlagCheck {
		printCurrentLatest(Version, rel.TagName)
		cmp := compareVersions(current, latest)
		switch {
		case cmp < 0:
			fmt.Println("run 'agiler upgrade' to update")
		case cmp == 0:
			fmt.Printf("agiler is up to date (%s)\n", rel.TagName)
		default:
			fmt.Println("you are ahead of the latest published release")
		}
		return nil
	}

	if !upgradeFlagForce && upgradeFlagVersion == "" && compareVersions(current, latest) >= 0 {
		fmt.Printf("agiler is up to date (%s)\n", rel.TagName)
		return nil
	}

	if err := preflightWritable(exe); err != nil {
		return err
	}

	archive := archiveName(rel.TagName, runtime.GOOS, runtime.GOARCH)
	tmpDir, err := os.MkdirTemp("", "agiler-upgrade-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	printCurrentLatest(Version, rel.TagName)
	fmt.Printf("downloading %s\n", archive)
	archivePath, err := downloadAndVerify(ctx, rel.TagName, archive, tmpDir)
	if err != nil {
		return err
	}
	fmt.Println("verifying sha256... ok")

	newBinary := filepath.Join(tmpDir, "agiler.new")
	if err := extractBinary(archivePath, newBinary); err != nil {
		return err
	}

	fmt.Printf("installing to %s\n", exe)
	if err := replaceExecutable(newBinary, exe); err != nil {
		return err
	}

	fmt.Printf("upgraded agiler %s -> %s\n", displayVersion(Version), rel.TagName)
	return nil
}

func printCurrentLatest(current, latest string) {
	fmt.Printf("current: %s\n", displayVersion(current))
	fmt.Printf("latest:  %s\n", latest)
}

func displayVersion(v string) string {
	if normalizeVersion(v) == "" {
		return "dev"
	}
	return v
}
