package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1"

	"github.com/buildpack/packs"
	"github.com/buildpack/packs/cf"
	"github.com/buildpack/packs/img"
)

var (
	dropletPath  string
	metadataPath string
	repoName     string
	stackName    string
	useDaemon    bool
	useHelpers   bool
)

func init() {
	packs.InputDropletPath(&dropletPath)
	packs.InputMetadataPath(&metadataPath)
	packs.InputStackName(&stackName)
	packs.InputUseDaemon(&useDaemon)
	packs.InputUseHelpers(&useHelpers)
}

func main() {
	flag.Parse()
	repoName = flag.Arg(0)
	if flag.NArg() != 1 || repoName == "" || stackName == "" || (metadataPath != "" && dropletPath == "") {
		packs.Exit(packs.FailCode(packs.CodeInvalidArgs, "parse arguments"))
	}
	packs.Exit(export())
}

func export() error {
	if useHelpers {
		if err := img.SetupCredHelpers(repoName, stackName); err != nil {
			return packs.FailErr(err, "setup credential helpers")
		}
	}

	newRepoStore := img.NewRegistry
	if useDaemon {
		newRepoStore = img.NewDaemon
	}
	repoStore, err := newRepoStore(repoName)
	if err != nil {
		return packs.FailErr(err, "access", repoName)
	}

	stackStore, err := img.NewRegistry(stackName)
	if err != nil {
		return packs.FailErr(err, "access", stackName)
	}
	stackImage, err := stackStore.Image()
	if err != nil {
		return packs.FailErr(err, "get image for", stackName)
	}

	var (
		repoImage v1.Image
		metadata  packs.BuildMetadata
	)
	if dropletPath != "" {
		if metadataPath != "" {
			var err error
			metadata.App, metadata.Buildpacks, err = readDropletMetadata(metadataPath)
			if err != nil {
				return packs.FailErr(err, "get droplet metadata")
			}
		}
		layer, err := dropletToLayer(dropletPath)
		if err != nil {
			return packs.FailErr(err, "transform", dropletPath, "into layer")
		}
		defer os.Remove(layer)
		repoImage, _, err = img.Append(stackImage, layer)
		if err != nil {
			return packs.FailErr(err, "append droplet to", stackName)
		}
	} else {
		repoImage, err = repoStore.Image()
		if err != nil {
			return packs.FailErr(err, "get image for", repoName)
		}
		repoImage, err = img.Rebase(repoImage, stackImage, func(labels map[string]string) (v1.Image, error) {
			if err := json.Unmarshal([]byte(labels[packs.BuildLabel]), &metadata); err != nil {
				return nil, err
			}
			oldStore, err := img.NewRegistry(metadata.RunImage.Name + "@" + metadata.RunImage.SHA)
			if err != nil {
				return nil, err
			}
			return oldStore.Image()
		})
		if err != nil {
			return packs.FailErr(err, "rebase", repoName, "on", stackName)
		}
	}
	stackDigest, err := stackImage.Digest()
	if err != nil {
		return packs.FailErr(err, "get digest for", stackName)
	}
	metadata.RunImage.Name = stackStore.Ref().Context().String()
	metadata.RunImage.SHA = stackDigest.String()
	buildJSON, err := json.Marshal(metadata)
	if err != nil {
		return packs.FailErr(err, "get encode metadata for", repoName)
	}
	repoImage, err = img.Label(repoImage, packs.BuildLabel, string(buildJSON))
	if err != nil {
		return packs.FailErr(err, "label", repoName)
	}
	if err := repoStore.Write(repoImage); err != nil {
		return packs.FailErrCode(err, packs.CodeFailedUpdate, "write", repoName)
	}
	return nil
}

func readDropletMetadata(path string) (packs.AppMetadata, []packs.BuildpackMetadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return packs.AppMetadata{}, nil, packs.FailErr(err, "failed to open", path)
	}
	defer f.Close()
	var metadata cf.DropletMetadata
	if err := json.NewDecoder(f).Decode(&metadata); err != nil {
		return packs.AppMetadata{}, nil, packs.FailErr(err, "failed to decode", path)
	}
	return metadata.PackMetadata.App, metadata.Buildpacks(), nil
}

func dropletToLayer(dropletPath string) (layer string, err error) {
	tmpDir, err := ioutil.TempDir("", "pack.export.layer")
	if err != nil {
		return "", packs.FailErr(err, "create temp directory")
	}
	defer os.RemoveAll(tmpDir)

	layer = tmpDir + ".tgz"
	dropletRoot := filepath.Join(tmpDir, "home", "vcap")

	if err := os.MkdirAll(dropletRoot, 0777); err != nil {
		return "", packs.FailErr(err, "setup droplet directory")
	}
	if _, err := packs.Run("tar", "-C", dropletRoot, "-xzf", dropletPath); err != nil {
		return "", packs.FailErr(err, "untar", dropletPath, "to", dropletRoot)
	}
	if _, err := packs.Run("chown", "-R", "vcap:vcap", dropletRoot); err != nil {
		return "", packs.FailErr(err, "recursively chown", dropletRoot, "to", "vcap:vcap")
	}
	if _, err := packs.Run("tar", "-C", tmpDir, "-czf", layer, "home"); err != nil {
		defer os.Remove(layer)
		return "", packs.FailErr(err, "tar", tmpDir, "to", layer)
	}
	return layer, nil
}
