package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"gopkg.in/yaml.v2"

	"github.com/buildpack/packs"
	herokuapp "github.com/buildpack/packs/heroku/app"
)

type ErrProcfileNoProcess string

func (e ErrProcfileNoProcess) Error() string {
	return fmt.Sprintf("%s", e)
}

type ErrNoCommandFound string

func (e ErrNoCommandFound) Error() string {
	return fmt.Sprintf("%s", e)
}

func main() {
	var inputDroplet string
	flag.StringVar(&inputDroplet, "inputDroplet", "/tmp/droplet", "file containing compressed droplet")
	flag.Parse()
	command := strings.Join(flag.Args(), " ")

	supplyApp(inputDroplet, "/")

	// Heroku Container Registry will break on chown
	chownAll("heroku", "heroku", "/app")

	err := os.Chdir("/app")
	check(err, packs.CodeFailed, "change directory")

	if command == "" {
		processType := getProcessType()
		command, err = readCommand(processType)
		check(err, packs.CodeFailed, "please add a Procfile with a web process")
	}

	app, err := herokuapp.New()
	check(err, packs.CodeInvalidEnv, "build app env")
	for k, v := range app.Launch() {
		err := os.Setenv(k, v)
		check(err, packs.CodeInvalidEnv, "set app env")
	}

	args := []string{"/lifecycle/launcher", "/app", command, ""}
	err = syscall.Exec("/lifecycle/launcher", args, os.Environ())
	check(err, packs.CodeFailedLaunch, "launch")
}

func getProcessType() string {
	if value, ok := os.LookupEnv("DYNO"); ok {
		return strings.Split(value, ".")[0]
	}
	return "web"
}

func supplyApp(tgz, dst string) {
	if _, err := os.Stat(tgz); os.IsNotExist(err) {
		return
	} else {
		check(err, packs.CodeFailed, "stat", tgz)
	}
	err := exec.Command("tar", "-C", dst, "-xzf", tgz).Run()
	check(err, packs.CodeFailed, "untar", tgz, "to", dst)
}

func readCommand(processType string) (string, error) {
	if command, err := parseProcfile("/app/Procfile", processType); err == nil {
		return command, nil
	} else if command, err = parseReleaseYml("/app/release.yml", processType); err == nil {
		return command, nil
	}
	return "", ErrNoCommandFound("No command found, please specify one in your Procfile.")
}

func parseProcfile(path, processType string) (string, error) {
	if _, err := os.Stat(path); err == nil {
		buf, err := ioutil.ReadFile(path)
		procfile := string(buf)
		check(err, packs.CodeFailed, "parse Procfile")

		processes := make(map[string]string)

		for _, line := range strings.Split(procfile, "\n") {
			array := strings.SplitAfterN(line, ":", 2)
			if len(array) == 2 {
				processes[array[0]] = strings.TrimSpace(array[1])
			}
		}

		if process, ok := processes[fmt.Sprintf("%s:", processType)]; ok {
			return process, nil
		}
	}

	return "", ErrProcfileNoProcess("No web process in Procfile.")
}

func parseReleaseYml(path, processType string) (string, error) {
	releaseYml, err := ioutil.ReadFile(path)
	check(err, packs.CodeFailed, "read start command")
	var info struct {
		Addons              []string          `yaml:"addons"`
		DefaultProcessTypes map[string]string `yaml:"default_process_types"`
	}
	err = yaml.Unmarshal(releaseYml, &info)
	if err == nil {
		return info.DefaultProcessTypes[processType], nil
	} else {
		return "", err
	}
}

func chownAll(user, group, path string) error {
	err := exec.Command("chown", "-R", user+":"+group, path).Run()
	return err
}

func check(err error, code int, action ...string) {
	if err == nil {
		return
	}
	message := "failed to " + strings.Join(action, " ")
	fmt.Fprintf(os.Stderr, "Error: %s: %s", message, err)
	os.Exit(code)
}
