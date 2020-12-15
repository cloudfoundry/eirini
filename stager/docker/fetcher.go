package docker

import (
	"context"
	"fmt"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func Fetch(dockerRef string, sysCtx types.SystemContext) (*v1.ImageConfig, error) {
	ref, err := docker.ParseReference(dockerRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse docker reference")
	}

	ctx := context.Background()

	imgSrc, err := ref.NewImageSource(ctx, &sysCtx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get image source")
	}
	defer imgSrc.Close()
	blob, mimeType, err := imgSrc.GetManifest(ctx, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get manifest blob")
	}

	fmt.Println("this the mime type: ", mimeType)
	fmt.Println("this the blob: ", string(blob))
	normalized := manifest.NormalizedMIMEType(mimeType)
	if normalized == manifest.DockerV2ListMediaType {
		imageManifests, err := manifest.Schema2ListFromManifest(blob)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse manifest blob")
		}

		if len(imageManifests.Manifests) != 1 {
			return nil, fmt.Errorf("unexpected number of image manifestss: %+v", imageManifests.Manifests)
		}

		manifest := imageManifests.Manifests[0]
		if err != nil {
			return nil, errors.Wrap(err, "failed to inspect manifest")
		}

		fmt.Println("this is the os: ", manifest.Platform.OS)
		sysCtx.OSChoice = manifest.Platform.OS
	}

	img, err := image.FromUnparsedImage(ctx, &sysCtx, image.UnparsedInstance(imgSrc, nil))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get image")
	}

	imgV1, err := img.OCIConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get oci config")
	}

	return &imgV1.Config, nil
}
