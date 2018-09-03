package recipe

import (
	"encoding/json"
	"fmt"

	bap "code.cloudfoundry.org/buildpackapplifecycle"
	"code.cloudfoundry.org/runtimeschema/cc_messages"
)

type BuildpacksKeyModifier struct {
	CCBuildpacksJSON string
}

func (m *BuildpacksKeyModifier) Modify(result bap.StagingResult) (bap.StagingResult, error) {
	buildpacks, err := m.getProvidedBuildpacks()
	if err != nil {
		return bap.StagingResult{}, err
	}

	if err := m.modifyBuildpackKey(&result, buildpacks); err != nil {
		return bap.StagingResult{}, err
	}

	if err := m.modifyBuildpacks(&result, buildpacks); err != nil {
		return bap.StagingResult{}, err
	}

	return result, nil
}

func (m *BuildpacksKeyModifier) modifyBuildpackKey(result *bap.StagingResult, buildpacks []cc_messages.Buildpack) error {
	name := result.LifecycleMetadata.BuildpackKey
	key, err := m.getBuildpackKey(name, buildpacks)
	if err != nil {
		return err
	}
	result.LifecycleMetadata.BuildpackKey = key

	return nil
}

func (m *BuildpacksKeyModifier) modifyBuildpacks(result *bap.StagingResult, buildpacks []cc_messages.Buildpack) error {
	for i, b := range result.LifecycleMetadata.Buildpacks {
		modified, err := m.modifyBuildpackMetadata(b, buildpacks)
		if err != nil {
			return err
		}

		result.LifecycleMetadata.Buildpacks[i] = modified
	}

	return nil
}

func (m *BuildpacksKeyModifier) modifyBuildpackMetadata(b bap.BuildpackMetadata, buildpacks []cc_messages.Buildpack) (bap.BuildpackMetadata, error) {
	name := b.Key
	key, err := m.getBuildpackKey(name, buildpacks)
	if err != nil {
		return bap.BuildpackMetadata{}, err
	}
	b.Key = key
	return b, nil
}

func (m *BuildpacksKeyModifier) getProvidedBuildpacks() ([]cc_messages.Buildpack, error) {
	var providedBuildpacks []cc_messages.Buildpack
	err := json.Unmarshal([]byte(m.CCBuildpacksJSON), &providedBuildpacks)
	if err != nil {
		return []cc_messages.Buildpack{}, err
	}

	return providedBuildpacks, nil
}

func (m *BuildpacksKeyModifier) getBuildpackKey(name string, providedBuildpacks []cc_messages.Buildpack) (string, error) {
	for _, b := range providedBuildpacks {
		if b.Name == name {
			return b.Key, nil
		}
	}

	return "", fmt.Errorf("could not find buildpack with name: %s", name)
}
