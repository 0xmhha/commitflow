package cmd

import (
	"github.com/0xmhha/commitflow/cmd/upstream"
	"github.com/0xmhha/commitflow/internal/config"
)

// GetConfig returns the current application configuration resolved after
// flag parsing. Safe to call from sub-packages via SetConfigProvider.
func GetConfig() config.Config {
	return appConfig
}

func init() {
	upstream.SetConfigProvider(GetConfig)
	rootCmd.AddCommand(upstream.UpstreamCmd)
}
