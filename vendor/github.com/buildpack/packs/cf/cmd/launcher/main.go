package main

import (
	"encoding/json"
	"flag"
	"os"
	"strings"
	"syscall"

	"github.com/buildpack/packs"
	"github.com/buildpack/packs/cf"
)

const (
	homeDir         = "/home/vcap"
	appDir          = "/home/vcap/app"
	stagingInfoFile = "/home/vcap/staging_info.yml"
)

var (
	dropletPath  string
	metadataPath string
	startCommand string
)

func init() {
	packs.InputDropletPath(&dropletPath)
	packs.InputMetadataPath(&metadataPath)
}

func main() {
	flag.Parse()
	startCommand = strings.Join(flag.Args(), " ")
	packs.Exit(launch())
}

func launch() error {
	if dropletPath != "" {
		if _, err := packs.Run("tar", "-C", homeDir, "-xzf", dropletPath); err != nil {
			return packs.FailErr(err, "supply app")
		}
	}

	var (
		command string
		err     error
	)
	switch {
	case startCommand != "":
		command = startCommand
	case metadataPath != "":
		command, err = readMetadataCommand(metadataPath)
	default:
		command, err = readDropletCommand(stagingInfoFile)
	}
	if err != nil {
		return packs.FailErr(err, "determine start command")
	}

	if err := os.Chdir(appDir); err != nil {
		return packs.FailErr(err, "change directory to", appDir)
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

	args := []string{"/lifecycle/launcher", appDir, command, ""}
	if err := syscall.Exec("/lifecycle/launcher", args, os.Environ()); err != nil {
		return packs.FailErrCode(err, packs.CodeFailedLaunch, "launch")
	}
	return nil
}

func readDropletCommand(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", packs.FailErr(err, "read droplet start command")
	}
	var info struct {
		StartCommand string `json:"start_command"`
	}
	if err := json.NewDecoder(f).Decode(&info); err != nil {
		return "", packs.FailErr(err, "parse start command")
	}
	return info.StartCommand, nil
}

func readMetadataCommand(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", packs.FailErr(err, "read metadata start command")
	}
	var metadata cf.DropletMetadata
	if err := json.NewDecoder(f).Decode(&metadata); err != nil {
		return "", packs.FailErr(err, "parse start command")
	}
	return metadata.ProcessTypes["web"], nil
}
