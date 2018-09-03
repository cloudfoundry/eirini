package app_test

import (
	"testing"

	"github.com/sclevine/spec"

	pkgapp "github.com/buildpack/packs/heroku/app"
)

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
		app *pkgapp.App
	)

	it.Before(func() {
		var err error
		if app, err = pkgapp.New(); err != nil {
			t.Fatalf("Failed to create app: %s\n", err)
		}
		app.Env, _ = env()
	})

	it("should return the default staging env", func() {
		env := app.Stage()

		expected := cmpMap{
			{"STACK", "heroku-16", nil},
			{"DYNO", "local.1", nil},
			{"HOME", "/app", nil},
			{"LANG", "en_US.UTF-8", nil},
			{"TMPDIR", "/tmp", nil},
			{"PATH", "/usr/local/bin:/usr/bin:/bin", nil},
			{"USER", "heroku", nil},
		}
		if v1, v2 := len(env), len(expected); v1 != v2 {
			t.Fatalf("Different lengths: %d != %d\n", v1, v2)
		}
		compare(t, env, expected)
	})
}

func testLaunch(t *testing.T, when spec.G, it spec.S) {
	var (
		app *pkgapp.App
	)

	it.Before(func() {
		var err error
		if app, err = pkgapp.New(); err != nil {
			t.Fatalf("Failed to create app: %s\n", err)
		}
		app.Env, _ = env()
	})

	it("should return the default launch env", func() {
		env := app.Launch()
		expected := cmpMap{
			{"HOME", "/app", nil},
			{"LANG", "en_US.UTF-8", nil},
			{"PATH", "/usr/local/bin:/usr/bin:/bin", nil},
			{"PORT", "5000", nil},
			{"TMPDIR", "/tmp", nil},
			{"USER", "heroku", nil},
			{"STACK", "heroku-16", nil},
			{"DYNO", "local.1", nil},
		}
		if v1, v2 := len(env), len(expected); v1 != v2 {
			t.Fatalf("Different lengths: %d != %d\n", v1, v2)
		}
		compare(t, env, expected)
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

func env() (env func(string) (string, bool), set func(k, v string)) {
	m := map[string]string{}
	return func(k string) (string, bool) {
			v, ok := m[k]
			return v, ok
		}, func(k, v string) {
			m[k] = v
		}
}
