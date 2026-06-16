//go:build darwin && sparkle

package core

import (
	"context"
	"fmt"

	sparkle "github.com/abemedia/go-sparkle"
)

// UpdateBackend reports Sparkle on release macOS builds.
func UpdateBackend() string { return "sparkle" }

// ConfigureUpdateOnStartup toggles Sparkle's automatic update checks.
func ConfigureUpdateOnStartup(enabled bool) {
	sparkle.SetAutomaticallyChecksForUpdates(enabled)
	sparkle.SetAutomaticallyDownloadsUpdates(false)
	sparkle.ResetUpdateCycle()
}

// CheckForUpdate opens Sparkle's interactive update UI.
func CheckForUpdate(_ context.Context, _ string) (*UpdateInfo, error) {
	sparkle.CheckForUpdates()
	return nil, nil
}

// DownloadAndApplyUpdate is not used on macOS; Sparkle handles installation.
func DownloadAndApplyUpdate(_ context.Context, _ *UpdateInfo, _ func(float64)) error {
	return fmt.Errorf("macOS updates are installed by Sparkle")
}
