// +build windows

package config

import (
	"os"
	"syscall"
)

func userHomeDir() string {
	home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	return home
}

func makeDirectory() error {
	dir := ConfigDir()

	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}

	attrs, err := syscall.GetFileAttributes(p)
	if err != nil {
		return err
	}

	return syscall.SetFileAttributes(p, attrs|syscall.FILE_ATTRIBUTE_HIDDEN)
}
