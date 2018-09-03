package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/buildpack/packs"
)

const (
	Cytokine     = "/packs/cytokine"
	MetadataFile = "release.yml"
)

func main() {
	var buildpacksDir string
	var buildpackOrder string
	var skipDetect bool
	var appDir string
	var cacheDir string
	var envDir string
	var outputSlug string
	var outputCache string
	flag.StringVar(&buildpacksDir, "buildpacksDir", "/var/lib/buildpacks", "directory containing buildpacks")
	flag.StringVar(&buildpackOrder, "buildpackOrder", "heroku/ruby", "list of buildpacks to run")
	flag.BoolVar(&skipDetect, "skipDetect", false, "run detection")
	flag.StringVar(&appDir, "appDir", "/tmp/app", "directory containing the app")
	flag.StringVar(&cacheDir, "cacheDir", "/tmp/cache", "directory containing containing the cache")
	flag.StringVar(&envDir, "envDir", "/tmp/env", "directory containing the env vars")
	flag.StringVar(&outputSlug, "outputSlug", "/out/slug.tgz", "output file containing the slug")
	flag.StringVar(&outputCache, "outputCache", "/cache/cache.tgz", "output file containing the cache")

	flag.Parse()

	os.MkdirAll(appDir, os.ModeTemporary)
	os.MkdirAll(cacheDir, os.ModeTemporary)
	os.MkdirAll(envDir, os.ModeTemporary)
	os.MkdirAll(buildpacksDir, os.ModePerm)
	os.MkdirAll(filepath.Dir(outputSlug), os.ModePerm)
	os.MkdirAll(filepath.Dir(outputCache), os.ModePerm)

	buildpacks := strings.Split(buildpackOrder, ",")
	if strings.Join(buildpacks, "") == "" && !skipDetect {
		buildpack, err := detect(appDir, buildpacksDir)
		if err != nil || buildpack == "" {
			fatal(err, packs.CodeFailed, "detect")
		}

		buildpacks = []string{buildpack}
	}

	buildpackOptions := createBuildpackOptions(buildpacks)

	err := compile(appDir, cacheDir, envDir, buildpacksDir, buildpackOptions)
	if err != nil {
		fatal(err, packs.CodeFailedBuild, "compile")
	}

	err = release(appDir, buildpacksDir, filepath.Join(appDir, MetadataFile), buildpackOptions)
	if err != nil {
		fatal(err, packs.CodeFailedBuild, "release")
	}

	err = makeSlug("/tmp/slug.tgz", appDir)
	if err != nil {
		fatal(err, packs.CodeFailedBuild, "make-slug")
	}

	err = os.Rename("/tmp/slug.tgz", outputSlug)
	if err != nil {
		fatal(err, packs.CodeFailedBuild, "move-slug")
	}

	err = compress(cacheDir, outputCache)
	if err != nil {
		fatal(err, packs.CodeFailed, "tar", outputCache, "src", cacheDir)
	}
}

func detect(appDir, buildpackDir string) (string, error) {
	cmd := exec.Command(Cytokine, "detect-buildpack", "--verbose", appDir, buildpackDir)
	out, err := cmd.CombinedOutput()
	if err == nil {
		properties := strings.Split(string(out), " ")
		for _, property := range properties {
			if strings.LastIndex(property, "buildpack=") == 0 {
				rawBuildpackString := strings.Replace(property, "buildpack=", "", 1)
				return strings.TrimSpace(strings.Replace(rawBuildpackString, "\"", "", 2)), err
			}
		}
		return "", nil
	} else {
		return "", err
	}
}

func compile(appDir, cacheDir, envDir, buildpackDir string, buildpacks []string) error {

	args := append([]string{"run-buildpacks", appDir, cacheDir, envDir, buildpackDir}, buildpacks...)
	cmd := exec.Command(Cytokine, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// TODO don't run the buildpack as root
	//uid, gid := userLookup("heroku")
	//cmd.SysProcAttr = &syscall.SysProcAttr{
	//	Credential: &syscall.Credential{Uid: uid, Gid: gid},
	//}

	err := cmd.Run()
	return err
}

func release(appDir, buildpackDir, metadataFile string, buildpacks []string) error {
	args := append([]string{"release-buildpacks", appDir, buildpackDir, metadataFile}, buildpacks...)
	err := exec.Command(Cytokine, args...).Run()
	return err
}

func makeSlug(outputSlug, appDir string) error {
	cmd := exec.Command(Cytokine, "make-slug", outputSlug, appDir)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return err
}

func createBuildpackOptions(buildpacks []string) []string {
	buildpacksAsOpts := make([]string, len(buildpacks))
	for i, buildpack := range buildpacks {
		buildpacksAsOpts[i] = fmt.Sprintf("--buildpack=%s", buildpack)
	}
	return buildpacksAsOpts
}

func compress(src, tgz string) error {
	// TODO capture error messages and log them in debug mode
	return exec.Command("tar", "-C", src, "-czf", tgz, ".").Run()
}

func fatal(err error, code int, action ...string) {
	message := "failed to " + strings.Join(action, " ")
	fmt.Fprintf(os.Stderr, "Error: %s: %s", message, err)
	os.Exit(code)
}
