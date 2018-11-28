package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/satori/go.uuid"
)

//This is a small wrapper around the launcher which is required to
//get the start_command from the staging_info.yml
//inside the container. The command is passed to the
//code.cloudfoundry.org/buildpackapplifecycle/launcher.
//Build this binary as "launch".
func main() {
	fmt.Println("ARGS:", os.Args)
	command := os.Getenv("START_COMMAND")
	if command == "" {
		command = readCommand("/home/vcap/staging_info.yml")
	}

	index, err := parsePodIndex()

	check(err, "parse pod index")

	err = os.Setenv("INSTANCE_INDEX", index)
	check(err, "setting instance index env var")

	err = os.Setenv("CF_INSTANCE_INDEX", index)
	check(err, "setting instance index env var")

	uuid := uuid.NewV4()
	err = os.Setenv("CF_INSTANCE_GUID", uuid.String())
	check(err, "setting instance guid env var")

	var launcherPath string
	if len(os.Args) > 1 {
		launcherPath = os.Args[1]
	}

	if launcherPath == "" {
		launcherPath = "/lifecycle/launcher"
	}

	args := []string{launcherPath, "/home/vcap/app", command, ""}

	err = syscall.Exec(launcherPath, args, os.Environ()) //#nosec
	check(err, "execute launcher")
}

func parsePodIndex() (string, error) {
	podName := os.Getenv("POD_NAME")
	sl := strings.Split(podName, "-")

	if len(sl) <= 1 {
		return "", fmt.Errorf("Could not parse pod name from %s", podName)
	}
	return sl[len(sl)-1], nil
}

func readCommand(path string) string {
	stagingInfo, err := os.Open(filepath.Clean(path))
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
