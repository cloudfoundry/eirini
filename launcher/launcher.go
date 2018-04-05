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

		"CF_INSTANCE_ADDR":        "0.0.0.0:8080",
		"CF_INSTANCE_GUID":        "guid",
		"CF_INSTANCE_INDEX":       "0",
		"CF_INSTANCE_INTERNAL_IP": "0.0.0.0",
		"CF_INSTANCE_IP":          "0.0.0.0",
		"CF_INSTANCE_PORT":        "8080",
		"CF_INSTANCE_PORTS":       `[{"external":8080,"internal":8080}]`,
		"INSTANCE_GUID":           "instance_id",
		"INSTANCE_INDEX":          "0",
		"PORT":                    "8080",
		"TMPDIR":                  "/home/vcap/tmp",
		"VCAP_APP_HOST":           "0.0.0.0",
		"VCAP_APP_PORT":           "8080",
		"VCAP_SERVICES":           "{}",
		"START_COMMAND":           startCmd,
	}
}
