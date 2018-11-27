package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/buildpack/packs"
	herokuapp "github.com/buildpack/packs/heroku/app"
)

func main() {
	err := os.Chdir("/app")
	check(err, packs.CodeFailed, "change directory")

	app, err := herokuapp.New()
	check(err, packs.CodeInvalidEnv, "build app env")
	for k, v := range app.Launch() {
		err := os.Setenv(k, v)
		check(err, packs.CodeInvalidEnv, "set app env")
	}

	args := append([]string{"/lifecycle/shell", "/app"}, os.Args[1:]...)
	err = syscall.Exec("/lifecycle/shell", args, os.Environ())
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
