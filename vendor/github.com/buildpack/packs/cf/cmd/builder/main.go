package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	bal "code.cloudfoundry.org/buildpackapplifecycle"
	"code.cloudfoundry.org/cli/cf/appfiles"

	"github.com/buildpack/packs"
	"github.com/buildpack/packs/cf"
)

var (
	appName string
	appZip  string
	appDir  string

	buildDir     string
	cacheDir     string
	cachePath    string
	metadataPath string
	dropletPath  string

	buildpacksDir  string
	buildpackOrder []string
	skipDetect     bool
)

func main() {
	config := bal.NewLifecycleBuilderConfig(nil, false, false)
	if err := config.Parse(os.Args[1:]); err != nil {
		packs.Exit(packs.FailErrCode(err, packs.CodeInvalidArgs, "parse arguments"))
	}

	buildDir = config.BuildDir()
	cacheDir = config.BuildArtifactsCacheDir()
	cachePath = config.OutputBuildArtifactsCache()
	metadataPath = config.OutputMetadata()
	dropletPath = config.OutputDroplet()

	buildpacksDir = config.BuildpacksDir()
	buildpackOrder = config.BuildpackOrder()
	skipDetect = config.SkipDetect()

	appName = os.Getenv(packs.EnvAppName)
	appZip = os.Getenv(packs.EnvAppZip)
	appDir = os.Getenv(packs.EnvAppDir)

	if wd, err := os.Getwd(); appDir == "" && err == nil {
		appDir = wd
	}

	packs.Exit(stage())
}

func stage() error {
	var (
		extraArgs  []string
		appVersion string

		cacheTarDir   = filepath.Dir(cachePath)
		metadataDir   = filepath.Dir(metadataPath)
		dropletDir    = filepath.Dir(dropletPath)
		buildpackConf = filepath.Join(buildpacksDir, "config.json")
	)

	if appZip != "" {
		appVersion = fileSHA(appZip)
		if err := copyAppZip(appZip, buildDir); err != nil {
			return packs.FailErr(err, "extract app zip")
		}
	} else if appDir != "" {
		appVersion = commitSHA(appDir)
		if !cmpDir(appDir, buildDir) {
			if err := copyAppDir(appDir, buildDir); err != nil {
				return packs.FailErr(err, "copy app directory")
			}
		}
	} else {
		return packs.FailCode(packs.CodeInvalidArgs, "parse app directory")
	}

	if _, err := os.Stat(cachePath); err == nil {
		if err := untar(cachePath, cacheDir); err != nil {
			return packs.FailErr(err, "extract cache")
		}
	}

	if err := vcapDir(dropletDir, metadataDir, cacheTarDir); err != nil {
		return packs.FailErr(err, "prepare destination directories")
	}
	if err := vcapDirAll(buildDir, cacheDir, "/home/vcap/tmp"); err != nil {
		return packs.FailErr(err, "prepare source directories")
	}
	if err := copyBuildpacks("/buildpacks", buildpacksDir); err != nil {
		return packs.FailErr(err, "add buildpacks")
	}

	if strings.Join(buildpackOrder, "") == "" && !skipDetect {
		names, err := reduceJSON(buildpackConf, "name")
		if err != nil {
			return packs.FailErr(err, "determine buildpack names")
		}
		extraArgs = append(extraArgs, "-buildpackOrder", names)
	}

	uid, gid, err := userLookup("vcap")
	if err != nil {
		return packs.FailErr(err, "determine vcap UID/GID")
	}
	if err := setupStdFds(); err != nil {
		return packs.FailErr(err, "adjust fd ownership")
	}
	if err := setupEnv(); err != nil {
		return packs.FailErrCode(err, packs.CodeInvalidEnv, "setup env")
	}

	cmd := exec.Command("/lifecycle/builder", append(os.Args[1:], extraArgs...)...)
	cmd.Dir = buildDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: uid, Gid: gid},
	}
	if err := cmd.Run(); err != nil {
		return packs.FailErrCode(err, packs.CodeFailedBuild, "build")
	}
	if err := setKeyJSON(metadataPath, "pack_metadata", cf.PackMetadata{
		App: packs.AppMetadata{
			Name: appName,
			SHA:  appVersion,
		},
	}); err != nil {
		return packs.FailErr(err, "write metadata")
	}
	return nil
}

func copyAppDir(src, dst string) error {
	copier := appfiles.ApplicationFiles{}
	files, err := copier.AppFilesInDir(src)
	if err != nil {
		return packs.FailErr(err, "analyze app in", src)
	}
	if err := copier.CopyFiles(files, src, dst); err != nil {
		return packs.FailErr(err, "copy app from", src, "to", dst)
	}
	return nil
}

func copyAppZip(src, dst string) error {
	zipper := appfiles.ApplicationZipper{}
	tmpDir, err := ioutil.TempDir("", "pack")
	if err != nil {
		return packs.FailErr(err, "create temp dir")
	}
	defer os.RemoveAll(tmpDir)
	if err := zipper.Unzip(src, tmpDir); err != nil {
		return packs.FailErr(err, "unzip app from", src, "to", tmpDir)
	}
	return copyAppDir(tmpDir, dst)
}

func cmpDir(dirs ...string) bool {
	var last string
	for _, dir := range dirs {
		next, err := filepath.Abs(dir)
		if err != nil {
			return false
		}
		switch last {
		case "", next:
			last = next
		default:
			return false
		}
	}
	return true
}

func vcapDir(dirs ...string) error {
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0777); err != nil {
			return packs.FailErr(err, "make directory", dir)
		}
		if _, err := packs.Run("chown", "vcap:vcap", dir); err != nil {
			return packs.FailErr(err, "chown", dir, "to vcap:vcap")
		}
	}
	return nil
}

func vcapDirAll(dirs ...string) error {
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0777); err != nil {
			return packs.FailErr(err, "make directory", dir)
		}
		if _, err := packs.Run("chown", "-R", "vcap:vcap", dir); err != nil {
			return packs.FailErr(err, "recursively chown", dir, "to", "vcap:vcap")
		}
	}
	return nil
}

func commitSHA(dir string) string {
	v, err := packs.Run("git", "-C", dir, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return v
}

func fileSHA(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// TODO: test with /dev/null
func setKeyJSON(path, key string, value interface{}) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return packs.FailErr(err, "open metadata")
	}
	defer f.Close()

	var contents map[string]interface{}
	if err := json.NewDecoder(f).Decode(&contents); err != nil {
		return packs.FailErr(err, "decode JSON at", path)
	}
	contents[key] = value
	if _, err := f.Seek(0, 0); err != nil {
		return packs.FailErr(err, "seek file at", path)
	}
	if err := f.Truncate(0); err != nil {
		return packs.FailErr(err, "truncate file at", path)
	}
	if err := json.NewEncoder(f).Encode(contents); err != nil {
		return packs.FailErr(err, "encode JSON to", path)
	}
	return nil
}

func copyBuildpacks(src, dst string) error {
	files, err := ioutil.ReadDir(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return packs.FailErr(err, "setup buildpacks", src)
	}

	for _, f := range files {
		filename := f.Name()
		ext := filepath.Ext(filename)
		if strings.ToLower(ext) != ".zip" || len(filename) != 36 {
			continue
		}
		sum := strings.ToLower(strings.TrimSuffix(filename, ext))
		unzip(filepath.Join(src, filename), filepath.Join(dst, sum))
	}
	return nil
}

func reduceJSON(path string, key string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", packs.FailErr(err, "open", path)
	}
	var list []map[string]string
	if err := json.NewDecoder(f).Decode(&list); err != nil {
		return "", packs.FailErr(err, "decode", path)
	}

	var out []string
	for _, m := range list {
		out = append(out, m[key])
	}
	return strings.Join(out, ","), nil
}

func setupEnv() error {
	app, err := cf.New()
	if err != nil {
		return packs.FailErr(err, "build app env")
	}
	for k, v := range app.Stage() {
		err := os.Setenv(k, v)
		if err != nil {
			return packs.FailErr(err, "set app env")
		}
	}
	return nil
}

func setupStdFds() error {
	cmd := exec.Command("chown", "vcap", "/dev/stdout", "/dev/stderr")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return packs.FailErr(err, "fix permissions of stdout and stderr")
	}
	return nil
}

func unzip(zip, dst string) error {
	if err := os.MkdirAll(dst, 0777); err != nil {
		return packs.FailErr(err, "ensure directory", dst)
	}
	if _, err := packs.Run("unzip", "-qq", zip, "-d", dst); err != nil {
		return packs.FailErr(err, "unzip", zip, "to", dst)
	}
	return nil
}

func untar(tar, dst string) error {
	if err := os.MkdirAll(dst, 0777); err != nil {
		return packs.FailErr(err, "ensure directory", dst)
	}
	if _, err := packs.Run("tar", "-C", dst, "-xzf", tar); err != nil {
		return packs.FailErr(err, "untar", tar, "to", dst)
	}
	return nil
}

func userLookup(u string) (uid, gid uint32, err error) {
	usr, err := user.Lookup(u)
	if err != nil {
		return 0, 0, packs.FailErr(err, "find user", u)
	}
	uid64, err := strconv.ParseUint(usr.Uid, 10, 32)
	if err != nil {
		return 0, 0, packs.FailErr(err, "parse uid", usr.Uid)
	}
	gid64, err := strconv.ParseUint(usr.Gid, 10, 32)
	if err != nil {
		return 0, 0, packs.FailErr(err, "parse gid", usr.Gid)
	}
	return uint32(uid64), uint32(gid64), nil
}
