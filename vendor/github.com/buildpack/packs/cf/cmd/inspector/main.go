package main

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/buildpack/packs"
	"github.com/buildpack/packs/img"
)

var (
	refName   string
	useDaemon bool
)

func init() {
	packs.InputUseDaemon(&useDaemon)
}

func main() {
	flag.Parse()
	refName = flag.Arg(0)
	if flag.NArg() != 1 || refName == "" {
		packs.Exit(packs.FailCode(packs.CodeInvalidArgs, "parse arguments"))
	}
	packs.Exit(inspect())
}

func inspect() error {
	if err := img.SetupCredHelpers(refName); err != nil {
		return packs.FailErr(err, "setup credential helper")
	}

	store, err := img.NewRegistry(refName)
	if err != nil {
		return packs.FailErr(err, "access", refName)
	}
	image, err := store.Image()
	if err != nil {
		if rErr, ok := err.(*remote.Error); ok && len(rErr.Errors) > 0 {
			switch rErr.Errors[0].Code {
			case remote.UnauthorizedErrorCode, remote.ManifestUnknownErrorCode:
				return packs.FailCode(packs.CodeNotFound, "find", refName)
			}
		}
		return packs.FailErr(err, "get", refName)
	}
	config, err := image.ConfigFile()
	if err != nil {
		return packs.FailErr(err, "get config")
	}
	out, err := encode(config.Config.Labels)
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func encode(m map[string]string) ([]byte, error) {
	out := map[string]json.RawMessage{}
	for k, v := range m {
		switch k {
		case packs.BuildLabel, packs.BuildpackLabel:
			out[k] = []byte(v)
		}
	}
	return json.Marshal(out)
}
