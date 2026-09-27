//go:build windows

package doctor

// freeBytes reports free space on Windows.
//
// GetDiskFreeSpaceEx would give the real number, but it needs a syscall and
// this build tag file exists precisely to keep the package dependency-free.
// Returning 0 makes doctor print "unknown" rather than a wrong figure, which
// is the better failure: a phone user on a desktop build learns that the
// check is unavailable instead of being told the disk is empty.
func freeBytes(path string) uint64 { return 0 }
