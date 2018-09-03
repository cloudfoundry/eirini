package cf_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/sclevine/spec"
)

func TestSystem(t *testing.T) {
	spec.Run(t, "build and launch", testBuildAndLaunch)
}

func run(t *testing.T, name string, args ...string) string {
	t.Helper()
	b, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func testBuildAndLaunch(t *testing.T, _ spec.G, it spec.S) {
	it("should build and launch an app", func() {
		cmd := exec.Command(
			"/packs/builder",
			"-buildpacksDir", "/var/lib/buildpacks",
			"-outputDroplet", "/tmp/droplet.tgz",
			"-outputBuildArtifactsCache", "/tmp/cache.tgz",
			"-outputMetadata", "/tmp/result.json",
		)
		cmd.Dir = "./fixtures/go-app"
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to build: %v", err)
		}

		cmd = exec.Command(
			"/packs/launcher",
			"-droplet", "/tmp/droplet.tgz",
			"-metadata", "/tmp/result.json",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatalf("failed to start app: %v", err)
		}

		if body, err := get("http://localhost:8080/some-path", 2*time.Second); err != nil {
			t.Fatalf("failed to reach app: %v", err)
		} else if body != "Path: /some-path" {
			t.Fatalf(`unexpected response: "%s" != "Path: /some-path"`, body)
		}

		http.Get("http://localhost:8080/exit")
		cmd.Wait()

		appName := "test-app-" + uuid(t)
		cmd = exec.Command(
			"/packs/exporter",
			"-droplet", "/tmp/droplet.tgz",
			"-metadata", "/tmp/result.json",
			"-stack", "packs/cflinuxfs2:run",
			"-daemon",
			appName,
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to export app: %v", err)
		}

		containerName := uuid(t)
		buf := &bytes.Buffer{}
		cmd = exec.Command(
			"docker", "run", "--rm", "--name", containerName, appName,
		)
		cmd.Stdout = buf
		cmd.Stderr = buf
		if err := cmd.Start(); err != nil {
			t.Fatalf("failed to start app: %v", err)
		}

		if err := contains(buf, "Fixture app about to listen", 2*time.Second); err != nil {
			t.Fatalf("failed to start app: %v", err)
		}

		output, err := exec.Command("docker", "exec", containerName, "curl", "--silent", "localhost:8080/some-path").CombinedOutput()
		if err != nil {
			t.Fatalf("failed to reach app: %v: %s", err, string(output))
		}
		if string(output) != "Path: /some-path" {
			t.Fatalf(`unexpected response: "%s" != "Path: /some-path"`, string(output))
		}

		exec.Command("docker", "exec", containerName, "curl", "--silent", "localhost:8080/exit").Run()
		cmd.Wait()
	})
}

func contains(buf *bytes.Buffer, txt string, timeout time.Duration) error {
	t := time.NewTimer(timeout).C
	for {
		select {
		case <-t:
			return errors.New("timeout")
		default:
			if strings.Contains(buf.String(), txt) {
				return nil
			}
		}
	}
}

func get(uri string, timeout time.Duration) (string, error) {
	err := errors.New("timeout")
	t := time.NewTimer(timeout).C
	for {
		select {
		case <-t:
			return "", err
		default:
			var out *http.Response
			out, err = http.Get(uri)
			if err != nil {
				continue
			}
			body, err := ioutil.ReadAll(out.Body)
			out.Body.Close()
			if err != nil {
				continue
			}
			return string(body), nil
		}
	}
}

func uuid(t *testing.T) string {
	t.Helper()
	kernelUUIDPath := "/proc/sys/kernel/random/uuid"
	id, err := ioutil.ReadFile(kernelUUIDPath)
	if err != nil {
		t.Fatalf("failed to generate uuid: %s", err)
	}
	return strings.TrimSpace(string(id))
}
