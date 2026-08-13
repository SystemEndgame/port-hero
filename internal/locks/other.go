//go:build !linux && !darwin

package locks

import "fmt"

func platformByFile(path string) ([]Lock, error) {
	return nil, fmt.Errorf("file lock detection is not supported on this platform")
}

func platformAll() ([]Lock, error) {
	return nil, fmt.Errorf("file lock detection is not supported on this platform")
}
