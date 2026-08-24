// Package cli defines the process exit-code contract for scripting, CI
// pipelines and monitoring integrations.
//
//	0  success, no warnings
//	1  success with warnings (e.g. protected port, force-kill required)
//	2  not found (no matching process / port is free)
//	3  blocked by the Safety Shield or permission denied
//	4  invalid input or ambiguous match
//	5  internal error
//	6  uncertain — ancestry chain incomplete (PID reuse, orphan, missing parent)
package cli

// Exit codes.
const (
	ExitOK          = 0
	ExitWarnings    = 1
	ExitNotFound    = 2
	ExitBlocked     = 3
	ExitInvalid     = 4
	ExitInternal    = 5
	ExitUncertain   = 6
)
