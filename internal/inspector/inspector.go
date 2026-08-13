package inspector

// inspector is the platform-agnostic facade. Actual implementation is in
// darwin.go, linux.go and windows.go — only one is compiled per target.

// isAlive reports whether a PID currently exists.
func isAlive(pid int) bool { return platformIsAlive(pid) }

// parseCommandTokens splits a reconstructed command line into argv.
// Falls back to strings.Fields when the platform has no better parser.
func parseCommandTokens(cmd string) []string { return platformParseCommand(cmd) }
