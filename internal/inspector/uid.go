//go:build !windows

package inspector

import (
	"os/user"
	"strconv"
	"time"

	"github.com/SystemEndgame/port-hero/internal/cache"
)

// uidCache resolves numeric UIDs to usernames. UID→name mappings are very
// stable in practice, so a 10-minute TTL bounds memory without ever showing
// stale owners for long.
var uidCache = cache.New[string](10 * time.Minute)

// lookupUID resolves a numeric UID to a username, caching results.
// Returns ok=false when the lookup fails (e.g. inside a container without
// /etc/passwd entries).
func lookupUID(uid string) (string, bool) {
	if name, ok := uidCache.Get(uid); ok {
		return name, true
	}
	u, err := user.LookupId(uid)
	if err != nil {
		return "", false
	}
	uidCache.Set(uid, u.Username)
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
