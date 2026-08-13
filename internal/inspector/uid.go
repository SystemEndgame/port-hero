//go:build !windows

package inspector

import (
	"os/user"
	"strconv"
	"sync"
)

var (
	uidCacheMu sync.Mutex
	uidCache   = map[string]string{}
)

// lookupUID resolves a numeric UID to a username, caching results.
// Returns ok=false when the lookup fails (e.g. inside a container without
// /etc/passwd entries).
func lookupUID(uid string) (string, bool) {
	uidCacheMu.Lock()
	defer uidCacheMu.Unlock()
	if name, ok := uidCache[uid]; ok {
		return name, true
	}
	u, err := user.LookupId(uid)
	if err != nil {
		return "", false
	}
	uidCache[uid] = u.Username
	return u.Username, true
}

// ParseUIDToName converts a numeric UID string to a username.
func ParseUIDToName(uid string) string {
	if uid == "" {
		return ""
	}
	if _, err := strconv.Atoi(uid); err != nil {
		return uid
	}
	if name, ok := lookupUID(uid); ok {
		return name
	}
	return uid
}
