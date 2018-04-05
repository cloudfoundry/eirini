package main

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
)

//This is a smal wrapper arounc the launcher which is required to
//get the start_command from the staging_info.yml
//inside the container. The command is passed to the
//code.cloudfoundry.org/buildpackapplifecycle/launcher.
//Build this binary as "launch".
func main() {
	command := os.Getenv("START_COMMAND")
	if command == "" {
		command = readCommand("/home/vcap/staging_info.yml")
	}

	args := []string{"/lifecycle/launcher", "/home/vcap/app", command, ""}
	err := syscall.Exec("/lifecycle/launcher", args, os.Environ())
	check(err, "execute launcher")
}

func readCommand(path string) string {
	stagingInfo, err := os.Open(path)
	check(err, "read start command")
	var info struct {
		StartCommand string `json:"start_command"`
	}
	err = json.NewDecoder(stagingInfo).Decode(&info)
	check(err, "parse start command")
	return info.StartCommand
}

func check(err error, action string) {
	if err == nil {
		return
	}
	message := "failed to " + action
	fmt.Fprintf(os.Stderr, "Error: %s: %s", message, err)
	os.Exit(1)
}
