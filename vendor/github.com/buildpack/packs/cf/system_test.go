package cf_test

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/sclevine/spec"
)

func TestSystem(t *testing.T) {
	spec.Run(t, "build and launch", testBuildAndLaunch)
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

		body, err := get("http://localhost:8080/some-path", 2*time.Second)
		if err != nil {
			t.Fatalf("failed to reach app: %v", err)
		}
		defer body.Close()
		path, err := ioutil.ReadAll(body)
		if err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if p := string(path); p != "Path: /some-path" {
			t.Fatalf("unexpected response: %s != /some-path", p)
		}

		http.Get("http://localhost:8080/exit")
	})
}

func get(uri string, timeout time.Duration) (io.ReadCloser, error) {
	err := errors.New("timeout")
	t := time.NewTimer(timeout).C
	for {
		select {
		case <-t:
			return nil, err
		default:
			var out *http.Response
			out, err = http.Get(uri)
			if err == nil {
				return out.Body, nil
			}
		}
	}
}
