//go:build windows

package inspector

// lookupUID on Windows: accounts are SIDs, not numeric UIDs.
func lookupUID(uid string) (string, bool) {
	return "", false
}
