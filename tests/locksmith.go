package tests

import (
	"os"
	"path/filepath"
	"strings"

	errorspkg "github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

type Locksmith struct {
	locksDir string
	lockType int
}

func NewExclusiveLocksmith(locksDir string) *Locksmith {
	return &Locksmith{
		locksDir: locksDir,
		lockType: unix.LOCK_EX,
	}
}

var FlockSyscall = unix.Flock

func (l *Locksmith) Lock(key string) (*os.File, error) {
	if err := os.MkdirAll(l.locksDir, 0755); err != nil {
		return nil, err
	}
	key = strings.Replace(key, "/", "", -1)
	lockFile, err := os.OpenFile(l.path(key), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, errorspkg.Wrapf(err, "creating lock file for key `%s`", key)
	}

	fd := int(lockFile.Fd())
	if err := FlockSyscall(fd, l.lockType); err != nil {
		return nil, err
	}

	return lockFile, nil
}

func (l *Locksmith) Unlock(lockFile *os.File) error {
	defer lockFile.Close()
	fd := int(lockFile.Fd())
	return FlockSyscall(fd, unix.LOCK_UN)
}

func (l *Locksmith) path(key string) string {
	return filepath.Join(l.locksDir, key+".lock")
}
