package bifrost

import (
	"context"
	"encoding/json"
	"fmt"

	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	"github.com/containers/image/types"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

//counterfeiter:generate . ImageMetadataFetcher
//counterfeiter:generate . ImageRefParser
//counterfeiter:generate . StagingCompleter

type StagingCompleter interface {
	CompleteStaging(ctx context.Context, req cf.StagingCompletedRequest) error
}

type ImageMetadataFetcher func(string, types.SystemContext) (*v1.ImageConfig, error)

func (f ImageMetadataFetcher) Fetch(dockerRef string, sysCtx types.SystemContext) (*v1.ImageConfig, error) {
	return f(dockerRef, sysCtx)
}

type ImageRefParser func(string) (string, error)

func (f ImageRefParser) Parse(img string) (string, error) {
	return f(img)
}

type DockerStaging struct {
	Logger               lager.Logger
	ImageMetadataFetcher ImageMetadataFetcher
	ImageRefParser       ImageRefParser
	StagingCompleter     StagingCompleter
}

type StagingResult struct {
	LifecycleType     string            `json:"lifecycle_type"`
	LifecycleMetadata LifecycleMetadata `json:"lifecycle_metadata"`
	ProcessTypes      ProcessTypes      `json:"process_types"`
	ExecutionMetadata string            `json:"execution_metadata"`
}

type LifecycleMetadata struct {
	DockerImage string `json:"docker_image"`
}

type ProcessTypes struct {
	Web string `json:"web"`
}

type port struct {
	Port     uint   `json:"Port"`
	Protocol string `json:"Protocol"`
}

type executionMetadata struct {
	Cmd   []string `json:"cmd"`
	Ports []port   `json:"ports"`
}

func (s DockerStaging) TransferStaging(ctx context.Context, stagingGUID string, request cf.StagingRequest) error {
	logger := s.Logger.Session("transfer-staging", lager.Data{"staging-guid": stagingGUID})

	taskCallbackResponse := cf.StagingCompletedRequest{
		TaskGUID:   stagingGUID,
		Annotation: fmt.Sprintf(`{"completion_callback": "%s"}`, request.CompletionCallback),
	}

	imageConfig, err := s.getImageConfig(request.Lifecycle.DockerLifecycle)
	if err != nil {
		logger.Error("failed-to-get-image-config", err)

		return s.respondWithFailure(ctx, taskCallbackResponse, errors.Wrap(err, "failed to get image config"))
	}

	ports, err := parseExposedPorts(imageConfig)
	if err != nil {
		logger.Error("failed-to-parse-exposed-ports", err)

		return s.respondWithFailure(ctx, taskCallbackResponse, errors.Wrap(err, "failed to parse exposed ports"))
	}

	stagingResult, err := buildStagingResult(request.Lifecycle.DockerLifecycle.Image, ports)
	if err != nil {
		logger.Error("failed-to-build-staging-result", err)

		return s.respondWithFailure(ctx, taskCallbackResponse, errors.Wrap(err, "failed to build staging result"))
	}

	taskCallbackResponse.Result = stagingResult

	return s.CompleteStaging(ctx, taskCallbackResponse)
}

func (s DockerStaging) respondWithFailure(ctx context.Context, taskCompletedRequest cf.StagingCompletedRequest, err error) error {
	taskCompletedRequest.Failed = true
	taskCompletedRequest.FailureReason = err.Error()

	return s.CompleteStaging(ctx, taskCompletedRequest)
}

func (s DockerStaging) CompleteStaging(ctx context.Context, taskCompletedRequest cf.StagingCompletedRequest) error {
	return s.StagingCompleter.CompleteStaging(ctx, taskCompletedRequest)
}

func (s DockerStaging) getImageConfig(lifecycle *cf.StagingDockerLifecycle) (*v1.ImageConfig, error) {
	dockerRef, err := s.ImageRefParser.Parse(lifecycle.Image)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse image ref")
	}

	imgMetadata, err := s.ImageMetadataFetcher.Fetch(dockerRef, types.SystemContext{
		DockerAuthConfig: &types.DockerAuthConfig{
			Username: lifecycle.RegistryUsername,
			Password: lifecycle.RegistryPassword,
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch image metadata")
	}

	return imgMetadata, nil
}

func parseExposedPorts(imageConfig *v1.ImageConfig) ([]port, error) {
	var (
		portNum  uint
		protocol string
	)

	ports := make([]port, 0, len(imageConfig.ExposedPorts))

	for imagePort := range imageConfig.ExposedPorts {
		_, err := fmt.Sscanf(imagePort, "%d/%s", &portNum, &protocol)
		if err != nil {
			return []port{}, errors.Wrap(err, "")
		}

		ports = append(ports, port{
			Port:     portNum,
			Protocol: protocol,
		})
	}

	return ports, nil
}

func buildStagingResult(image string, ports []port) (string, error) {
	executionMetadataJSON, err := json.Marshal(executionMetadata{
		Cmd:   []string{},
		Ports: ports,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to parse execution metadata")
	}

	payload := StagingResult{
		LifecycleType: "docker",
		LifecycleMetadata: LifecycleMetadata{
			DockerImage: image,
		},
		ProcessTypes:      ProcessTypes{Web: ""},
		ExecutionMetadata: string(executionMetadataJSON),
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", errors.Wrap(err, "failed to build payload json")
	}

	return string(payloadJSON), nil
}
