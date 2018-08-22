package execshim

import (
	"io"
	"syscall"
)

//go:generate counterfeiter -o exec_fake/fake_cmd.go . Cmd

type Cmd interface {
	Start() error
	StdoutPipe() (io.ReadCloser, error)
	StderrPipe() (io.ReadCloser, error)
	Wait() error
	Run() error
	CombinedOutput() ([]byte, error)

	SysProcAttr() *syscall.SysProcAttr
}


