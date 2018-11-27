package main

import (
	"os"
	"syscall"

	"github.com/buildpack/packs"
	"github.com/buildpack/packs/cf"
)

const appDir = "/home/vcap/app"

func main() {
	packs.Exit(shell())
}

func shell() error {
	if err := os.Chdir(appDir); err != nil {
		return packs.FailErr(err, "change directory")
	}
	app, err := cf.New()
	if err != nil {
		return packs.FailErrCode(err, packs.CodeInvalidEnv, "build app env")
	}
	for k, v := range app.Launch() {
		if err := os.Setenv(k, v); err != nil {
			return packs.FailErrCode(err, packs.CodeInvalidEnv, "set app env")
		}
	}
	args := append([]string{"/lifecycle/shell", appDir}, os.Args[1:]...)
	if err := syscall.Exec("/lifecycle/shell", args, os.Environ()); err != nil {
		return packs.FailErrCode(err, packs.CodeFailedLaunch, "run")
	}
	return nil
}
