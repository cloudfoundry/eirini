package cf

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/buildpack/packs"
)

const (
	kernelUUIDPath     = "/proc/sys/kernel/random/uuid"
	cgroupMemLimitPath = "/sys/fs/cgroup/memory/memory.limit_in_bytes"
	cgroupMemUnlimited = 9223372036854771712
)

type VCAPApplication struct {
	ApplicationID      string            `json:"application_id"`
	ApplicationName    string            `json:"application_name"`
	ApplicationURIs    []string          `json:"application_uris"`
	ApplicationVersion string            `json:"application_version"`
	Host               string            `json:"host,omitempty"`
	InstanceID         string            `json:"instance_id,omitempty"`
	InstanceIndex      *uint             `json:"instance_index,omitempty"`
	Limits             map[string]uint64 `json:"limits"`
	Name               string            `json:"name"`
	Port               *uint             `json:"port,omitempty"`
	SpaceID            string            `json:"space_id"`
	SpaceName          string            `json:"space_name"`
	URIs               []string          `json:"uris"`
	Version            string            `json:"version"`
}

type App struct {
	Env func(string) (string, bool)

	name       string
	mem        uint64
	disk       uint64
	fds        uint64
	id         string
	instanceID string
	spaceID    string
	version    string
	ip         string
}

func New() (*App, error) {
	var err error
	app := &App{Env: os.LookupEnv}
	app.name = "app"
	if app.mem, err = totalMem(); err != nil {
		return nil, err
	}
	app.disk = 1024
	var fds syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &fds); err != nil {
		return nil, err
	}
	app.fds = fds.Cur
	if app.ip, err = containerIP(); err != nil {
		return nil, err
	}
	for _, id := range []*string{&app.id, &app.instanceID, &app.spaceID, &app.version} {
		if *id, err = uuid(); err != nil {
			return nil, err
		}
	}
	return app, nil
}

func containerIP() (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return "", err
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ip := addr.To4(); ip != nil {
			return ip.String(), nil
		}
	}
	return "", errors.New("no valid ipv4 address found")
}

func uuid() (string, error) {
	id, err := ioutil.ReadFile(kernelUUIDPath)
	return strings.TrimSpace(string(id)), err
}

func totalMem() (uint64, error) {
	contents, err := ioutil.ReadFile(cgroupMemLimitPath)
	if err != nil {
		return 0, err
	}
	memBytes, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 64)
	if err != nil {
		return 0, err
	}
	if memBytes == cgroupMemUnlimited {
		return 1024, nil
	}
	return memBytes / 1024 / 1024, nil
}

func (a *App) config() (name, uri string, limits map[string]uint64) {
	name = a.envStr(packs.EnvAppName, a.name)
	uri = a.envStr(packs.EnvAppURI, name+".local")

	disk := a.envInt(packs.EnvAppDisk, a.disk)
	fds := a.envInt(packs.EnvAppFds, a.fds)
	mem := a.envInt(packs.EnvAppMemory, a.mem)
	limits = map[string]uint64{"disk": disk, "fds": fds, "mem": mem}

	return name, uri, limits
}

func (a *App) Stage() map[string]string {
	name, uri, limits := a.config()

	vcapApp, err := json.Marshal(&VCAPApplication{
		ApplicationID:      a.id,
		ApplicationName:    name,
		ApplicationURIs:    []string{uri},
		ApplicationVersion: a.version,
		Limits:             limits,
		Name:               name,
		SpaceID:            a.spaceID,
		SpaceName:          fmt.Sprintf("%s-space", name),
		URIs:               []string{uri},
		Version:            a.version,
	})
	if err != nil {
		vcapApp = []byte("{}")
	}

	sysEnv := map[string]string{
		"HOME": "/home/vcap",
		"LANG": "en_US.UTF-8",
		"PATH": "/usr/local/bin:/usr/bin:/bin",
		"USER": "vcap",
	}

	appEnv := map[string]string{
		"CF_INSTANCE_ADDR":        "",
		"CF_INSTANCE_INTERNAL_IP": a.ip,
		"CF_INSTANCE_IP":          a.ip,
		"CF_INSTANCE_PORT":        "",
		"CF_INSTANCE_PORTS":       "[]",
		"CF_STACK":                "cflinuxfs2",
		"MEMORY_LIMIT":            fmt.Sprintf("%dm", limits["mem"]),
		"VCAP_APPLICATION":        string(vcapApp),
		"VCAP_SERVICES":           "{}",
	}
	a.envOverride(appEnv)

	return mergeMaps(sysEnv, appEnv)
}

func (a *App) Launch() map[string]string {
	name, uri, limits := a.config()

	vcapApp, err := json.Marshal(&VCAPApplication{
		ApplicationID:      a.id,
		ApplicationName:    name,
		ApplicationURIs:    []string{uri},
		ApplicationVersion: a.version,
		Host:               "0.0.0.0",
		InstanceID:         a.instanceID,
		InstanceIndex:      uintPtr(0),
		Limits:             limits,
		Name:               name,
		Port:               uintPtr(8080),
		SpaceID:            a.spaceID,
		SpaceName:          fmt.Sprintf("%s-space", name),
		URIs:               []string{uri},
		Version:            a.version,
	})
	if err != nil {
		vcapApp = []byte("{}")
	}

	sysEnv := map[string]string{
		"HOME": "/home/vcap/app",
		"LANG": "en_US.UTF-8",
		"PATH": "/usr/local/bin:/usr/bin:/bin",
		"USER": "vcap",
	}

	appEnv := map[string]string{
		"CF_INSTANCE_ADDR":        a.ip + ":8080",
		"CF_INSTANCE_GUID":        a.instanceID,
		"CF_INSTANCE_INDEX":       "0",
		"CF_INSTANCE_INTERNAL_IP": a.ip,
		"CF_INSTANCE_IP":          a.ip,
		"CF_INSTANCE_PORT":        "8080",
		"CF_INSTANCE_PORTS":       `[{"external":8080,"internal":8080}]`,
		"INSTANCE_GUID":           a.instanceID,
		"INSTANCE_INDEX":          "0",
		"MEMORY_LIMIT":            fmt.Sprintf("%dm", limits["mem"]),
		"PORT":                    "8080",
		"TMPDIR":                  "/home/vcap/tmp",
		"VCAP_APP_HOST":           "0.0.0.0",
		"VCAP_APPLICATION":        string(vcapApp),
		"VCAP_APP_PORT":           "8080",
		"VCAP_SERVICES":           "{}",
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

func uintPtr(i uint) *uint {
	return &i
}
