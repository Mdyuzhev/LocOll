//go:build !linux

package system

type syscallStatfs struct {
	Blocks uint64
	Bavail uint64
	Bsize  int64
}

func statfs(path string, stat *syscallStatfs) error {
	return nil
}
