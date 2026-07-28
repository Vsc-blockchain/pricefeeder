package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Version is the pricefeeder's version. It is not maintained in the source:
// the build stamps it in from the git tag via
//
//	-ldflags "-X github.com/vsc-blockchain/pricefeeder/cmd.Version=$(git describe --tags)"
//
// See the Makefile and the Dockerfile. Builds that do not set it fall back to
// the revision the Go toolchain records in the binary, so that a `version` call
// always says something true about what is running.
var Version string

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the pricefeeder version",
	Long:  `Print the version this binary was built from, as derived from the git tag.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version())
	},
}

// version returns the stamped version, or falls back to the VCS revision the
// toolchain embedded at build time.
func version() string {
	if Version != "" {
		return Version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	var revision, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				modified = "-dirty"
			}
		}
	}

	if revision == "" {
		return "unknown"
	}
	if len(revision) > 12 {
		revision = revision[:12]
	}
	return revision + modified
}
