package img

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func Append(base v1.Image, tar string) (v1.Image, error) {
	layer, err := tarball.LayerFromFile(tar)
	if err != nil {
		return nil, fmt.Errorf("get layer from file: %s", err)
	}

	image, err := mutate.AppendLayers(base, layer)
	if err != nil {
		return nil, fmt.Errorf("append layer: %s", err)
	}
	return image, nil
}

type ImageFinder func(labels map[string]string) (v1.Image, error)

func Rebase(orig v1.Image, newBase v1.Image, oldBaseFinder ImageFinder) (v1.Image, error) {
	origConfig, err := orig.ConfigFile()
	if err != nil {
		return nil, err
	}
	oldBase, err := oldBaseFinder(origConfig.Config.Labels)
	if err != nil {
		return nil, fmt.Errorf("find old base: %s", err)
	}
	image, err := mutate.Rebase(orig, oldBase, newBase, nil)
	if err != nil {
		return nil, fmt.Errorf("rebase image: %s", err)
	}
	return image, nil
}

func Label(image v1.Image, k, v string) (v1.Image, error) {
	configFile, err := image.ConfigFile()
	if err != nil {
		return nil, err
	}
	config := *configFile.Config.DeepCopy()
	if config.Labels == nil {
		config.Labels = map[string]string{}
	}
	config.Labels[k] = v
	return mutate.Config(image, config)
}

func SetupCredHelpers(configPath string, refs ...string) error {
	// configPath := filepath.Join(homePath, ".docker", "config.json")
	config := map[string]interface{}{}
	if f, err := os.Open(configPath); err == nil {
		err := json.NewDecoder(f).Decode(&config)
		if f.Close(); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	var credHelpers map[string]interface{}
	if hash, ok := config["credHelpers"].(map[string]interface{}); ok {
		credHelpers = hash
	} else {
		credHelpers = make(map[string]interface{})
		config["credHelpers"] = credHelpers
	}
	added := false
	for _, refStr := range refs {
		ref, err := name.ParseReference(refStr, name.WeakValidation)
		if err != nil {
			return err
		}
		registry := ref.Context().RegistryStr()
		for _, ch := range []struct {
			domain string
			helper string
		}{
			{"([.]|^)gcr[.]io$", "gcr"},
			{"[.]amazonaws[.]", "ecr-login"},
			{"([.]|^)azurecr[.]io$", "acr"},
		} {
			match, err := regexp.MatchString("(?i)"+ch.domain, registry)
			if err != nil || !match {
				continue
			}
			credHelpers[registry] = ch.helper
			added = true
		}
	}
	if !added {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0777); err != nil {
		return err
	}
	f, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(config)
}
