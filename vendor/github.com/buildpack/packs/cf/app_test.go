package cf_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/sclevine/spec"

	"github.com/buildpack/packs"
	"github.com/buildpack/packs/cf"
)

var memory = flag.Uint64("memory", 1024, "expected memory usage in mb")

type cmpMap []struct {
	k, v2 string
	cmp   func(t *testing.T, v1, v2 string)
}

func TestApp(t *testing.T) {
	spec.Run(t, "#Stage", testStage)
	spec.Run(t, "#Launch", testLaunch)
}

func testStage(t *testing.T, when spec.G, it spec.S) {
	var (
		app *cf.App
		set func(k, v string)
	)

	it.Before(func() {
		var err error
		if app, err = cf.New(); err != nil {
			t.Fatalf("Failed to create app: %s\n", err)
		}
		app.Env, set = env()
	})

	it("should return the default staging env", func() {
		env := app.Stage()

		vcapApp, err := vcapAppExpect(env["VCAP_APPLICATION"])
		if err != nil {
			t.Fatalf("Error: %s\n", err)
		}
		vcapAppJSON, err := json.Marshal(vcapApp)
		if err != nil {
			t.Fatalf("Error: %s\n", err)
		}
		expected := cmpMap{
			{"CF_INSTANCE_ADDR", "", nil},
			{"CF_INSTANCE_INTERNAL_IP", "", hostIPCmp},
			{"CF_INSTANCE_IP", "", hostIPCmp},
			{"CF_INSTANCE_PORT", "", nil},
			{"CF_INSTANCE_PORTS", "[]", nil},
			{"CF_STACK", "cflinuxfs2", nil},
			{"HOME", "/home/vcap", nil},
			{"LANG", "en_US.UTF-8", nil},
			{"MEMORY_LIMIT", fmt.Sprintf("%dm", *memory), nil},
			{"PATH", "/usr/local/bin:/usr/bin:/bin", nil},
			{"USER", "vcap", nil},
			{"VCAP_APPLICATION", string(vcapAppJSON), vcapAppCmp},
			{"VCAP_SERVICES", "{}", nil},
		}
		if v1, v2 := len(env), len(expected); v1 != v2 {
			t.Fatalf("Different lengths: %d != %d\n", v1, v2)
		}
		compare(t, env, expected)
	})

	when("PACK env variables are set", func() {
		it("should customize the staging env accordingly", func() {
			set(packs.EnvAppName, "some-name")
			set(packs.EnvAppURI, "some-uri")
			set(packs.EnvAppDisk, "10")
			set(packs.EnvAppFds, "20")
			set(packs.EnvAppMemory, "30")

			env := app.Stage()

			if mem := env["MEMORY_LIMIT"]; mem != "30m" {
				t.Fatalf("Incorrect memory: %s", mem)
			}

			vcapApp, err := vcapAppExpect(env["VCAP_APPLICATION"])
			if err != nil {
				t.Fatalf("Error: %s\n", err)
			}
			vcapApp.ApplicationName = "some-name"
			vcapApp.ApplicationURIs = []string{"some-uri"}
			vcapApp.Limits = map[string]uint64{"disk": 10, "fds": 20, "mem": 30}
			vcapApp.Name = "some-name"
			vcapApp.SpaceName = "some-name-space"
			vcapApp.URIs = []string{"some-uri"}
			vcapAppJSON, err := json.Marshal(vcapApp)
			if err != nil {
				t.Fatalf("Error: %s\n", err)
			}
			vcapAppCmp(t, env["VCAP_APPLICATION"], string(vcapAppJSON))
		})
	})

	when("buildpack env vars are set", func() {
		it("should always override other values", func() {
			set(packs.EnvAppMemory, "30")
			set("CF_INSTANCE_IP", "some-ip")
			set("CF_INSTANCE_PORT", "some-port")
			set("CF_INSTANCE_PORTS", "some-ports")
			set("MEMORY_LIMIT", "some-memory")

			env := app.Stage()

			expected := cmpMap{
				{"CF_INSTANCE_IP", "some-ip", nil},
				{"CF_INSTANCE_PORT", "some-port", nil},
				{"CF_INSTANCE_PORTS", "some-ports", nil},
				{"MEMORY_LIMIT", "some-memory", nil},
			}
			compare(t, env, expected)
		})
	})

	when("a custom app name is set", func() {
		it("should use the name for the uri as well", func() {
			set(packs.EnvAppName, "some-name")

			env := app.Stage()

			vcapApp, err := vcapAppExpect(env["VCAP_APPLICATION"])
			if err != nil {
				t.Fatalf("Error: %s\n", err)
			}
			vcapApp.ApplicationName = "some-name"
			vcapApp.ApplicationURIs = []string{"some-name.local"}
			vcapApp.Name = "some-name"
			vcapApp.SpaceName = "some-name-space"
			vcapApp.URIs = []string{"some-name.local"}
			vcapAppJSON, err := json.Marshal(vcapApp)
			if err != nil {
				t.Fatalf("Error: %s\n", err)
			}
			vcapAppCmp(t, env["VCAP_APPLICATION"], string(vcapAppJSON))
		})
	})
}

func testLaunch(t *testing.T, when spec.G, it spec.S) {
	var (
		app *cf.App
		set func(k, v string)
	)

	it.Before(func() {
		var err error
		if app, err = cf.New(); err != nil {
			t.Fatalf("Failed to create app: %s\n", err)
		}
		app.Env, set = env()
	})

	it("should return the default launch env", func() {
		env := app.Launch()
		vcapApp, err := vcapAppExpect(env["VCAP_APPLICATION"])
		if err != nil {
			t.Fatalf("Error: %s\n", err)
		}
		vcapApp.Host = "0.0.0.0"
		vcapApp.InstanceID = env["CF_INSTANCE_GUID"]
		vcapApp.InstanceIndex = uintPtr(0)
		vcapApp.Port = uintPtr(8080)
		vcapAppJSON, err := json.Marshal(vcapApp)
		if err != nil {
			t.Fatalf("Error: %s\n", err)
		}
		expected := cmpMap{
			{"CF_INSTANCE_ADDR", ":8080", hostIPCmp},
			{"CF_INSTANCE_GUID", "", uuidCmp},
			{"CF_INSTANCE_INDEX", "0", nil},
			{"CF_INSTANCE_INTERNAL_IP", "", hostIPCmp},
			{"CF_INSTANCE_IP", "", hostIPCmp},
			{"CF_INSTANCE_PORT", "8080", nil},
			{"CF_INSTANCE_PORTS", `[{"external":8080,"internal":8080}]`, nil},
			{"HOME", "/home/vcap/app", nil},
			{"INSTANCE_GUID", env["CF_INSTANCE_GUID"], nil},
			{"INSTANCE_INDEX", "0", nil},
			{"LANG", "en_US.UTF-8", nil},
			{"MEMORY_LIMIT", fmt.Sprintf("%dm", *memory), nil},
			{"PATH", "/usr/local/bin:/usr/bin:/bin", nil},
			{"PORT", "8080", nil},
			{"TMPDIR", "/home/vcap/tmp", nil},
			{"USER", "vcap", nil},
			{"VCAP_APP_HOST", "0.0.0.0", nil},
			{"VCAP_APPLICATION", string(vcapAppJSON), vcapAppCmp},
			{"VCAP_APP_PORT", "8080", nil},
			{"VCAP_SERVICES", "{}", nil},
		}
		if v1, v2 := len(env), len(expected); v1 != v2 {
			t.Fatalf("Different lengths: %d != %d\n", v1, v2)
		}
		compare(t, env, expected)
	})

	when("PACK env variables are set", func() {
		it("should customize the launch env accordingly", func() {
			set(packs.EnvAppName, "some-name")
			set(packs.EnvAppURI, "some-uri")
			set(packs.EnvAppDisk, "10")
			set(packs.EnvAppFds, "20")
			set(packs.EnvAppMemory, "30")

			env := app.Launch()

			if mem := env["MEMORY_LIMIT"]; mem != "30m" {
				t.Fatalf("Incorrect memory: %s", mem)
			}

			vcapApp, err := vcapAppExpect(env["VCAP_APPLICATION"])
			if err != nil {
				t.Fatalf("Error: %s\n", err)
			}

			vcapApp.Host = "0.0.0.0"
			vcapApp.InstanceID = env["CF_INSTANCE_GUID"]
			vcapApp.InstanceIndex = uintPtr(0)
			vcapApp.Port = uintPtr(8080)

			vcapApp.ApplicationName = "some-name"
			vcapApp.ApplicationURIs = []string{"some-uri"}
			vcapApp.Limits = map[string]uint64{"disk": 10, "fds": 20, "mem": 30}
			vcapApp.Name = "some-name"
			vcapApp.SpaceName = "some-name-space"
			vcapApp.URIs = []string{"some-uri"}

			vcapAppJSON, err := json.Marshal(vcapApp)
			if err != nil {
				t.Fatalf("Error: %s\n", err)
			}
			vcapAppCmp(t, env["VCAP_APPLICATION"], string(vcapAppJSON))
		})
	})

	when("buildpack env vars are set", func() {
		it("should always override other values", func() {
			set(packs.EnvAppMemory, "30")
			set("CF_INSTANCE_ADDR", "some-addr")
			set("CF_INSTANCE_GUID", "some-guid")
			set("CF_INSTANCE_INDEX", "some-index")
			set("MEMORY_LIMIT", "some-memory")

			env := app.Launch()

			expected := cmpMap{
				{"CF_INSTANCE_ADDR", "some-addr", nil},
				{"CF_INSTANCE_GUID", "some-guid", nil},
				{"CF_INSTANCE_INDEX", "some-index", nil},
				{"MEMORY_LIMIT", "some-memory", nil},
			}
			compare(t, env, expected)
		})
	})

	when("a custom app name is set", func() {
		it("should use the name for the uri as well", func() {
			set(packs.EnvAppName, "some-name")

			env := app.Launch()

			vcapApp, err := vcapAppExpect(env["VCAP_APPLICATION"])
			if err != nil {
				t.Fatalf("Error: %s\n", err)
			}

			vcapApp.Host = "0.0.0.0"
			vcapApp.InstanceID = env["CF_INSTANCE_GUID"]
			vcapApp.InstanceIndex = uintPtr(0)
			vcapApp.Port = uintPtr(8080)

			vcapApp.ApplicationName = "some-name"
			vcapApp.ApplicationURIs = []string{"some-name.local"}
			vcapApp.Name = "some-name"
			vcapApp.SpaceName = "some-name-space"
			vcapApp.URIs = []string{"some-name.local"}

			vcapAppJSON, err := json.Marshal(vcapApp)
			if err != nil {
				t.Fatalf("Error: %s\n", err)
			}
			vcapAppCmp(t, env["VCAP_APPLICATION"], string(vcapAppJSON))
		})
	})
}

func compare(t *testing.T, env map[string]string, cmp cmpMap) {
	t.Helper()
	for _, exp := range cmp {
		if v1, ok := env[exp.k]; !ok {
			t.Fatalf("Missing: %s\n", exp.k)
		} else if exp.cmp != nil {
			exp.cmp(t, v1, exp.v2)
		} else if v1 != exp.v2 {
			t.Fatalf("%s: %s != %s\n", exp.k, v1, exp.v2)
		}
	}
}

func uuidCmp(t *testing.T, uuid, _ string) {
	t.Helper()
	if len(uuid) != 36 {
		t.Fatalf("Invalid UUID: %s\n", uuid)
	}
}

func hostIPCmp(t *testing.T, ip, suffix string) {
	t.Helper()
	out, err := exec.Command("hostname", "-i").Output()
	if err != nil {
		t.Fatalf("Error: %s\n", err)
	}
	if expected := strings.TrimSpace(string(out)) + suffix; ip != expected {
		t.Fatalf("Mismatched IP: %s != %s\n", ip, expected)
	}
}

func vcapAppExpect(vcapAppJSON string) (cf.VCAPApplication, error) {
	var vcapApp cf.VCAPApplication
	if err := json.Unmarshal([]byte(vcapAppJSON), &vcapApp); err != nil {
		return cf.VCAPApplication{}, err
	}
	ulimit, err := exec.Command("bash", "-c", "ulimit -n").Output()
	if err != nil {
		return cf.VCAPApplication{}, err
	}
	fds, err := strconv.ParseUint(strings.TrimSpace(string(ulimit)), 10, 64)
	if err != nil {
		return cf.VCAPApplication{}, err
	}
	return cf.VCAPApplication{
		ApplicationID:      vcapApp.ApplicationID,
		ApplicationName:    "app",
		ApplicationURIs:    []string{"app.local"},
		ApplicationVersion: vcapApp.ApplicationVersion,
		Limits:             map[string]uint64{"disk": 1024, "fds": fds, "mem": *memory},
		Name:               "app",
		SpaceID:            vcapApp.SpaceID,
		SpaceName:          "app-space",
		URIs:               []string{"app.local"},
		Version:            vcapApp.Version,
	}, nil
}

func vcapAppCmp(t *testing.T, va1, va2 string) {
	t.Helper()
	var vcapApp1 cf.VCAPApplication
	if err := json.Unmarshal([]byte(va1), &vcapApp1); err != nil {
		t.Fatalf("Error: %s\n", err)
	}
	var vcapApp2 cf.VCAPApplication
	if err := json.Unmarshal([]byte(va2), &vcapApp2); err != nil {
		t.Fatalf("Error: %s\n", err)
	}
	if !reflect.DeepEqual(vcapApp1, vcapApp2) {
		t.Fatalf("Mismatched VCAP_APPLICATION:\n%#v\n!=\n%#v\n", vcapApp1, vcapApp2)
	}

	set := map[string]struct{}{}
	total := 0
	for _, uuid := range []string{
		vcapApp1.ApplicationID,
		vcapApp1.SpaceID,
		vcapApp1.Version,
		vcapApp1.InstanceID,
	} {
		if uuid != "" {
			uuidCmp(t, uuid, "")
			set[uuid] = struct{}{}
			total++
		}
	}
	if l := len(set); l != total {
		t.Fatalf("Duplicate UUIDs: %d\n", total-l)
	}
	if v1, v2 := vcapApp1.Version, vcapApp2.ApplicationVersion; v1 != v2 {
		t.Fatalf("Mismatched version UUIDs: %s != %s\n", v1, v2)
	}
}

func uintPtr(i uint) *uint {
	return &i
}

func env() (env func(string) (string, bool), set func(k, v string)) {
	m := map[string]string{}
	return func(k string) (string, bool) {
			v, ok := m[k]
			return v, ok
		}, func(k, v string) {
			m[k] = v
		}
}
