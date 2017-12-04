package models

import (
	"errors"

	"code.cloudfoundry.org/bbs/format"
)

func (*VolumePlacement) Version() format.Version {
	return format.V1
}

func (*VolumePlacement) Validate() error {
	return nil
}

// to handle old cases, can be removed as soon as the bridge speaks V1
func (v *VolumeMount) VersionUpToV1() *VolumeMount {
	mode := "rw"
	if v.DeprecatedMode == DeprecatedBindMountMode_RO {
		mode = "r"
	}

	return &VolumeMount{
		Driver:       v.Driver,
		ContainerDir: v.ContainerDir,
		Mode:         mode,
		Shared: &SharedDevice{
			VolumeId:    v.DeprecatedVolumeId,
			MountConfig: string(v.DeprecatedConfig),
		},
	}
}

// while volume mounts are experimental, we should never persist a "old" volume
// mount to the db layer, so the handler must convert old data models to the new ones
// when volume mounts are no longer experimental, this validation strategy must be reconsidered
func (v *VolumeMount) Validate() error {
	var ve ValidationError
	if v.DeprecatedConfig != nil {
		ve = ve.Append(errors.New("invalid volume_mount deprecated config"))
	}
	if v.DeprecatedVolumeId != "" {
		ve = ve.Append(errors.New("invalid volume_mount deprecated id"))
	}
	if v.Driver == "" {
		ve = ve.Append(errors.New("invalid volume_mount driver"))
	}
	if !(v.Mode == "r" || v.Mode == "rw") {
		ve = ve.Append(errors.New("invalid volume_mount mode"))
	}
	if v.Shared != nil && v.Shared.VolumeId == "" {
		ve = ve.Append(errors.New("invalid volume_mount volume id"))
	}

	if !ve.Empty() {
		return ve
	}

	return nil
}
