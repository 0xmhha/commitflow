package upstream

import (
	"github.com/spf13/cobra"
)

// UpstreamCmd is the parent command for all upstream subcommands.
// Initialized at package level so that sub-command init() functions can
// call UpstreamCmd.AddCommand without relying on init() ordering.
var UpstreamCmd = &cobra.Command{
	Use:   "upstream",
	Short: "Manage upstream repository tracking",
	Long:  "Track, scan, and selectively apply commits from an upstream repository to your fork.",
}

// Subcommands register themselves via UpstreamCmd.AddCommand in their own init() functions.
