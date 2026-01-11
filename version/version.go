// Package version provides version information for the momentum CLI.
// These values are set at build time using ldflags.
package version

import (
	"fmt"
	"runtime"
)

// Build-time variables set via ldflags
var (
	// Version is the semantic version (e.g., "1.2.3")
	Version = "dev"

	// Commit is the git commit SHA
	Commit = "none"

	// Date is the build date
	Date = "unknown"
)

// Info returns a formatted version string
func Info() string {
	return fmt.Sprintf("momentum %s (commit: %s, built: %s, go: %s)",
		Version, Commit, Date, runtime.Version())
}

// Short returns just the version number
func Short() string {
	return Version
}
