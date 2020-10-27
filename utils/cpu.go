package utils

import "runtime"

// SetGOMAXPROCS sets precise number of Go processors
func SetGOMAXPROCS() (int, int) {
	numCPUs := runtime.NumCPU()
	orginalCPUs := numCPUs
	switch {
	case numCPUs <= 1:
		numCPUs = 2
	case numCPUs <= 4:
		numCPUs *= 3
	default:
		numCPUs *= 2
	}
	runtime.GOMAXPROCS(numCPUs)

	return orginalCPUs, numCPUs
}
