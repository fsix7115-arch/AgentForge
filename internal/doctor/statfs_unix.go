//go:build linux || darwin || android

package doctor

import "syscall"

// statfsT and statfs wrap syscall.Statfs so the rest of the package needs no
// build tags. Android reports GOOS=android but shares Linux's syscall surface,
// which is why android is listed here explicitly.
type statfsT = syscall.Statfs_t

func statfs(path string, st *statfsT) error {
	return syscall.Statfs(path, st)
}

// freeBytes reports free space on the filesystem holding path.
func freeBytes(path string) uint64 {
	var st statfsT
	if err := statfs(path, &st); err != nil {
		return 0
	}
	return uint64(st.Bavail) * uint64(st.Bsize)
}
