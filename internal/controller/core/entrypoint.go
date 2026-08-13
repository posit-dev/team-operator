// SPDX-License-Identifier: MIT
package core

// legacyEntrypointCommand and legacyEntrypointArgs are the historical hardcoded
// container entrypoint override for Connect and Package Manager. They're used as
// the default whenever a CR leaves both Command and Args unset, so upgrading the
// operator doesn't change the runtime behavior of existing deployments.
var (
	legacyEntrypointCommand = []string{"tini", "--"}
	legacyEntrypointArgs    = []string{"/usr/local/bin/startup.sh"}
)

// resolveEntrypoint returns command/args as given, unless both are empty, in which
// case it falls back to the legacy tini wrapper for backwards compatibility.
func resolveEntrypoint(command, args []string) ([]string, []string) {
	if len(command) == 0 && len(args) == 0 {
		return legacyEntrypointCommand, legacyEntrypointArgs
	}
	return command, args
}
