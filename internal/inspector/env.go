package inspector

import "os"

// osEnviron is a tiny indirection so platform exec helpers can share it.
func osEnviron() []string { return os.Environ() }
