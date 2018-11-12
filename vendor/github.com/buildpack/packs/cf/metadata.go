package cf

import (
	"code.cloudfoundry.org/buildpackapplifecycle"
	"github.com/buildpack/packs"
)

type DropletMetadata struct {
	buildpackapplifecycle.StagingResult
	PackMetadata PackMetadata `json:"pack_metadata"`
}

func (d *DropletMetadata) Buildpacks() []packs.BuildpackMetadata {
	var out []packs.BuildpackMetadata
	for _, bp := range d.LifecycleMetadata.Buildpacks {
		out = append(out, packs.BuildpackMetadata{
			Key:     bp.Key,
			Name:    bp.Name,
			Version: bp.Version,
		})
	}
	return out
}

type PackMetadata struct {
	App packs.AppMetadata `json:"app"`
}
