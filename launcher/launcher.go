package launcher

const (
	Launch   = "/lifecycle/launch"
	Launcher = "/lifecycle/launcher"
)

func SetupEnv(startCmd string) map[string]string {
	return map[string]string{
		"HOME": "/home/vcap/app",
		"LANG": "en_US.UTF-8",
		"PATH": "/usr/local/bin:/usr/bin:/bin",
		"USER": "vcap",

		"CF_INSTANCE_ADDR":  "0.0.0.0:8080",
		"CF_INSTANCE_GUID":  "guid",
		"CF_INSTANCE_PORT":  "8080",
		"CF_INSTANCE_PORTS": `[{"external":8080,"internal":8080}]`,
		"INSTANCE_GUID":     "instance_id",
		"TMPDIR":            "/home/vcap/tmp",
		"START_COMMAND":     startCmd,
	}
}
