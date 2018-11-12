package main

import (
	"fmt"
	"github.com/buildpack/packs"
	herokuapp "github.com/buildpack/packs/heroku/app"
	"os"
	"runtime"
	"strings"
	"syscall"
)

const shellScript = `
cd $1

if [ -n "$(ls ../profile.d/* 2> /dev/null)" ]; then
  for env_file in ../profile.d/*; do
    source $env_file
  done
fi

if [ -n "$(ls .profile.d/* 2> /dev/null)" ]; then
  for env_file in .profile.d/*; do
    source $env_file
  done
fi

shift

exec bash
`

func main() {
	err := os.Chdir("/app")
	check(err, packs.CodeFailed, "change directory")

	app, err := herokuapp.New()
	check(err, packs.CodeInvalidEnv, "build app env")
	for k, v := range app.Launch() {
		err := os.Setenv(k, v)
		check(err, packs.CodeInvalidEnv, "set app env")
	}

	runtime.GOMAXPROCS(1)

	err = syscall.Exec("/bin/bash", []string{
		"bash", "-c", shellScript, "bash", "/app",
	}, os.Environ())

	check(err, packs.CodeFailedLaunch, "run")
}

func check(err error, code int, action ...string) {
	if err == nil {
		return
	}
	message := "failed to " + strings.Join(action, " ")
	fmt.Fprintf(os.Stderr, "Error: %s: %s", message, err)
	os.Exit(code)
}
