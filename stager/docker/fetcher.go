package docker

import (
	"context"

	"github.com/containers/image/docker"
	"github.com/containers/image/image"
	"github.com/containers/image/types"
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
