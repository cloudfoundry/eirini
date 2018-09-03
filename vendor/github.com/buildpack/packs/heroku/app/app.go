package app

import (
	"os"
	"strconv"
)

type App struct {
	Env func(string) (string, bool)
}

func New() (*App, error) {
	app := &App{Env: os.LookupEnv}
	return app, nil
}

func (a *App) Stage() map[string]string {
	sysEnv := map[string]string{
		"HOME":   "/app",
		"LANG":   "en_US.UTF-8",
		"PATH":   "/usr/local/bin:/usr/bin:/bin",
		"TMPDIR": "/tmp",
		"USER":   "heroku",
	}

	appEnv := map[string]string{
		"STACK": "heroku-16",
		"DYNO":  "local.1",
	}
	a.envOverride(appEnv)

	return mergeMaps(sysEnv, appEnv)
}

func (a *App) Launch() map[string]string {
	sysEnv := map[string]string{
		"HOME":   "/app",
		"LANG":   "en_US.UTF-8",
		"PATH":   "/usr/local/bin:/usr/bin:/bin",
		"TMPDIR": "/tmp",
		"USER":   "heroku",
	}

	appEnv := map[string]string{
		"PORT":  "5000",
		"STACK": "heroku-16",
		"DYNO":  "local.1",
	}
	a.envOverride(appEnv)

	return mergeMaps(sysEnv, appEnv)
}

func (a *App) envStr(key, val string) string {
	if v, ok := a.Env(key); ok {
		return v
	}
	return val
}

func (a *App) envInt(key string, val uint64) uint64 {
	if v, ok := a.Env(key); ok {
		if vInt, err := strconv.ParseUint(v, 10, 64); err == nil {
			return vInt
		}
	}
	return val
}

func (a *App) envOverride(m map[string]string) {
	for k, v := range m {
		m[k] = a.envStr(k, v)
	}
}

func mergeMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
