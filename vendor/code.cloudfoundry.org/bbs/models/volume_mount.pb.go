// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: volume_mount.proto

package models

import (
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	io "io"
	math "math"
	math_bits "math/bits"
	reflect "reflect"
	strings "strings"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type SharedDevice struct {
	VolumeId    string `protobuf:"bytes,1,opt,name=volume_id,json=volumeId,proto3" json:"volume_id"`
	MountConfig string `protobuf:"bytes,2,opt,name=mount_config,json=mountConfig,proto3" json:"mount_config"`
}

func (m *SharedDevice) Reset()      { *m = SharedDevice{} }
func (*SharedDevice) ProtoMessage() {}
func (*SharedDevice) Descriptor() ([]byte, []int) {
	return fileDescriptor_bbde336a4634d84f, []int{0}
}
func (m *SharedDevice) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SharedDevice) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SharedDevice.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SharedDevice) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SharedDevice.Merge(m, src)
}
func (m *SharedDevice) XXX_Size() int {
	return m.Size()
}
func (m *SharedDevice) XXX_DiscardUnknown() {
	xxx_messageInfo_SharedDevice.DiscardUnknown(m)
}

var xxx_messageInfo_SharedDevice proto.InternalMessageInfo

func (m *SharedDevice) GetVolumeId() string {
	if m != nil {
		return m.VolumeId
	}
	return ""
}

func (m *SharedDevice) GetMountConfig() string {
	if m != nil {
		return m.MountConfig
	}
	return ""
}

type VolumeMount struct {
	Driver       string `protobuf:"bytes,1,opt,name=driver,proto3" json:"driver"`
	ContainerDir string `protobuf:"bytes,3,opt,name=container_dir,json=containerDir,proto3" json:"container_dir"`
	Mode         string `protobuf:"bytes,6,opt,name=mode,proto3" json:"mode"`
	// oneof device {
	Shared *SharedDevice `protobuf:"bytes,7,opt,name=shared,proto3" json:"shared"`
}

func (m *VolumeMount) Reset()      { *m = VolumeMount{} }
func (*VolumeMount) ProtoMessage() {}
func (*VolumeMount) Descriptor() ([]byte, []int) {
	return fileDescriptor_bbde336a4634d84f, []int{1}
}
func (m *VolumeMount) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *VolumeMount) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_VolumeMount.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *VolumeMount) XXX_Merge(src proto.Message) {
	xxx_messageInfo_VolumeMount.Merge(m, src)
}
func (m *VolumeMount) XXX_Size() int {
	return m.Size()
}
func (m *VolumeMount) XXX_DiscardUnknown() {
	xxx_messageInfo_VolumeMount.DiscardUnknown(m)
}

var xxx_messageInfo_VolumeMount proto.InternalMessageInfo

func (m *VolumeMount) GetDriver() string {
	if m != nil {
		return m.Driver
	}
	return ""
}

func (m *VolumeMount) GetContainerDir() string {
	if m != nil {
		return m.ContainerDir
	}
	return ""
}

func (m *VolumeMount) GetMode() string {
	if m != nil {
		return m.Mode
	}
	return ""
}

func (m *VolumeMount) GetShared() *SharedDevice {
	if m != nil {
		return m.Shared
	}
	return nil
}

type VolumePlacement struct {
	DriverNames []string `protobuf:"bytes,1,rep,name=driver_names,json=driverNames,proto3" json:"driver_names"`
}

func (m *VolumePlacement) Reset()      { *m = VolumePlacement{} }
func (*VolumePlacement) ProtoMessage() {}
func (*VolumePlacement) Descriptor() ([]byte, []int) {
	return fileDescriptor_bbde336a4634d84f, []int{2}
}
func (m *VolumePlacement) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *VolumePlacement) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_VolumePlacement.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *VolumePlacement) XXX_Merge(src proto.Message) {
	xxx_messageInfo_VolumePlacement.Merge(m, src)
}
func (m *VolumePlacement) XXX_Size() int {
	return m.Size()
}
func (m *VolumePlacement) XXX_DiscardUnknown() {
	xxx_messageInfo_VolumePlacement.DiscardUnknown(m)
}

var xxx_messageInfo_VolumePlacement proto.InternalMessageInfo

func (m *VolumePlacement) GetDriverNames() []string {
	if m != nil {
		return m.DriverNames
	}
	return nil
}

func init() {
	proto.RegisterType((*SharedDevice)(nil), "models.SharedDevice")
	proto.RegisterType((*VolumeMount)(nil), "models.VolumeMount")
	proto.RegisterType((*VolumePlacement)(nil), "models.VolumePlacement")
}

func init() { proto.RegisterFile("volume_mount.proto", fileDescriptor_bbde336a4634d84f) }

var fileDescriptor_bbde336a4634d84f = []byte{
	// 381 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x54, 0x91, 0xc1, 0x6a, 0xa3, 0x40,
	0x18, 0xc7, 0x9d, 0xc4, 0xb8, 0x66, 0x4c, 0x58, 0x77, 0xd8, 0x83, 0x2c, 0xcb, 0x18, 0x3c, 0x85,
	0x85, 0x35, 0xd0, 0x94, 0xd2, 0x73, 0x1a, 0x0a, 0x0d, 0xb4, 0x14, 0x0b, 0xbd, 0x8a, 0xd1, 0x89,
	0x19, 0x88, 0x4e, 0x31, 0x9a, 0x73, 0x1f, 0xa1, 0x8f, 0xd1, 0x47, 0xe9, 0x31, 0xd0, 0x4b, 0x4e,
	0xd2, 0x98, 0x4b, 0xf1, 0x94, 0x47, 0x28, 0xce, 0xd8, 0x36, 0xb9, 0x38, 0xf3, 0xfb, 0x7f, 0x7f,
	0x3f, 0xbf, 0xef, 0x2f, 0x44, 0x2b, 0xb6, 0xc8, 0x22, 0xe2, 0x46, 0x2c, 0x8b, 0x53, 0xfb, 0x21,
	0x61, 0x29, 0x43, 0x4a, 0xc4, 0x02, 0xb2, 0x58, 0xfe, 0xf9, 0x1f, 0xd2, 0x74, 0x9e, 0x4d, 0x6d,
	0x9f, 0x45, 0x83, 0x90, 0x85, 0x6c, 0xc0, 0xcb, 0xd3, 0x6c, 0xc6, 0x89, 0x03, 0xbf, 0x89, 0xd7,
	0x2c, 0x06, 0x3b, 0x77, 0x73, 0x2f, 0x21, 0xc1, 0x98, 0xac, 0xa8, 0x4f, 0xd0, 0x3f, 0xd8, 0xae,
	0x9b, 0xd3, 0xc0, 0x00, 0x3d, 0xd0, 0x6f, 0x8f, 0xba, 0x65, 0x6e, 0x7e, 0x8b, 0x8e, 0x2a, 0xae,
	0x57, 0x01, 0x1a, 0xc2, 0x0e, 0x9f, 0xc0, 0xf5, 0x59, 0x3c, 0xa3, 0xa1, 0xd1, 0xe0, 0x76, 0xbd,
	0xcc, 0xcd, 0x23, 0xdd, 0xd1, 0x38, 0x5d, 0x70, 0xb0, 0x5e, 0x01, 0xd4, 0xee, 0x79, 0x87, 0xeb,
	0x4a, 0x45, 0x16, 0x54, 0x82, 0x84, 0xae, 0x48, 0x52, 0x7f, 0x0d, 0x96, 0xb9, 0x59, 0x2b, 0x4e,
	0x7d, 0xa2, 0x33, 0xd8, 0xf5, 0x59, 0x9c, 0x7a, 0x34, 0x26, 0x89, 0x1b, 0xd0, 0xc4, 0x68, 0x72,
	0xeb, 0xaf, 0x32, 0x37, 0x8f, 0x0b, 0x4e, 0xe7, 0x0b, 0xc7, 0x34, 0x41, 0x7f, 0xa1, 0x5c, 0xa5,
	0x62, 0x28, 0xdc, 0xae, 0x96, 0xb9, 0xc9, 0xd9, 0xe1, 0x4f, 0x74, 0x0e, 0x95, 0x25, 0x5f, 0xdd,
	0xf8, 0xd1, 0x03, 0x7d, 0xed, 0xe4, 0xb7, 0x2d, 0x22, 0xb4, 0x0f, 0x03, 0x11, 0xf3, 0x08, 0x9f,
	0x53, 0x9f, 0x13, 0x59, 0x6d, 0xe8, 0xcd, 0x89, 0xac, 0xca, 0x7a, 0x6b, 0x22, 0xab, 0x2d, 0x5d,
	0xb1, 0x2e, 0xe1, 0x4f, 0xb1, 0xd4, 0xed, 0xc2, 0xf3, 0x49, 0x44, 0xe2, 0xb4, 0x4a, 0x47, 0x8c,
	0xef, 0xc6, 0x5e, 0x44, 0x96, 0x06, 0xe8, 0x35, 0x3f, 0xd3, 0x39, 0xd4, 0x1d, 0x4d, 0xd0, 0x4d,
	0x05, 0xa3, 0xd3, 0xf5, 0x16, 0x83, 0xcd, 0x16, 0x4b, 0xfb, 0x2d, 0x06, 0x8f, 0x05, 0x06, 0xcf,
	0x05, 0x06, 0x2f, 0x05, 0x06, 0xeb, 0x02, 0x83, 0xb7, 0x02, 0x83, 0xf7, 0x02, 0x4b, 0xfb, 0x02,
	0x83, 0xa7, 0x1d, 0x96, 0xd6, 0x3b, 0x2c, 0x6d, 0x76, 0x58, 0x9a, 0x2a, 0xfc, 0x5f, 0x0e, 0x3f,
	0x02, 0x00, 0x00, 0xff, 0xff, 0x1a, 0x23, 0x60, 0xde, 0x18, 0x02, 0x00, 0x00,
}

func (this *SharedDevice) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*SharedDevice)
	if !ok {
		that2, ok := that.(SharedDevice)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.VolumeId != that1.VolumeId {
		return false
	}
	if this.MountConfig != that1.MountConfig {
		return false
	}
	return true
}
func (this *VolumeMount) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*VolumeMount)
	if !ok {
		that2, ok := that.(VolumeMount)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.Driver != that1.Driver {
		return false
	}
	if this.ContainerDir != that1.ContainerDir {
		return false
	}
	if this.Mode != that1.Mode {
		return false
	}
	if !this.Shared.Equal(that1.Shared) {
		return false
	}
	return true
}
func (this *VolumePlacement) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*VolumePlacement)
	if !ok {
		that2, ok := that.(VolumePlacement)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if len(this.DriverNames) != len(that1.DriverNames) {
		return false
	}
	for i := range this.DriverNames {
		if this.DriverNames[i] != that1.DriverNames[i] {
			return false
		}
	}
	return true
}
func (this *SharedDevice) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 6)
	s = append(s, "&models.SharedDevice{")
	s = append(s, "VolumeId: "+fmt.Sprintf("%#v", this.VolumeId)+",\n")
	s = append(s, "MountConfig: "+fmt.Sprintf("%#v", this.MountConfig)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *VolumeMount) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 8)
	s = append(s, "&models.VolumeMount{")
	s = append(s, "Driver: "+fmt.Sprintf("%#v", this.Driver)+",\n")
	s = append(s, "ContainerDir: "+fmt.Sprintf("%#v", this.ContainerDir)+",\n")
	s = append(s, "Mode: "+fmt.Sprintf("%#v", this.Mode)+",\n")
	if this.Shared != nil {
		s = append(s, "Shared: "+fmt.Sprintf("%#v", this.Shared)+",\n")
	}
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *VolumePlacement) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 5)
	s = append(s, "&models.VolumePlacement{")
	s = append(s, "DriverNames: "+fmt.Sprintf("%#v", this.DriverNames)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func valueToGoStringVolumeMount(v interface{}, typ string) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("func(v %v) *%v { return &v } ( %#v )", typ, typ, pv)
}
func (m *SharedDevice) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SharedDevice) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SharedDevice) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.MountConfig) > 0 {
		i -= len(m.MountConfig)
		copy(dAtA[i:], m.MountConfig)
		i = encodeVarintVolumeMount(dAtA, i, uint64(len(m.MountConfig)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.VolumeId) > 0 {
		i -= len(m.VolumeId)
		copy(dAtA[i:], m.VolumeId)
		i = encodeVarintVolumeMount(dAtA, i, uint64(len(m.VolumeId)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *VolumeMount) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *VolumeMount) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *VolumeMount) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Shared != nil {
		{
			size, err := m.Shared.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintVolumeMount(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x3a
	}
	if len(m.Mode) > 0 {
		i -= len(m.Mode)
		copy(dAtA[i:], m.Mode)
		i = encodeVarintVolumeMount(dAtA, i, uint64(len(m.Mode)))
		i--
		dAtA[i] = 0x32
	}
	if len(m.ContainerDir) > 0 {
		i -= len(m.ContainerDir)
		copy(dAtA[i:], m.ContainerDir)
		i = encodeVarintVolumeMount(dAtA, i, uint64(len(m.ContainerDir)))
		i--
		dAtA[i] = 0x1a
	}
	if len(m.Driver) > 0 {
		i -= len(m.Driver)
		copy(dAtA[i:], m.Driver)
		i = encodeVarintVolumeMount(dAtA, i, uint64(len(m.Driver)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *VolumePlacement) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *VolumePlacement) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *VolumePlacement) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.DriverNames) > 0 {
		for iNdEx := len(m.DriverNames) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.DriverNames[iNdEx])
			copy(dAtA[i:], m.DriverNames[iNdEx])
			i = encodeVarintVolumeMount(dAtA, i, uint64(len(m.DriverNames[iNdEx])))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func encodeVarintVolumeMount(dAtA []byte, offset int, v uint64) int {
	offset -= sovVolumeMount(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *SharedDevice) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.VolumeId)
	if l > 0 {
		n += 1 + l + sovVolumeMount(uint64(l))
	}
	l = len(m.MountConfig)
	if l > 0 {
		n += 1 + l + sovVolumeMount(uint64(l))
	}
	return n
}

func (m *VolumeMount) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Driver)
	if l > 0 {
		n += 1 + l + sovVolumeMount(uint64(l))
	}
	l = len(m.ContainerDir)
	if l > 0 {
		n += 1 + l + sovVolumeMount(uint64(l))
	}
	l = len(m.Mode)
	if l > 0 {
		n += 1 + l + sovVolumeMount(uint64(l))
	}
	if m.Shared != nil {
		l = m.Shared.Size()
		n += 1 + l + sovVolumeMount(uint64(l))
	}
	return n
}

func (m *VolumePlacement) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.DriverNames) > 0 {
		for _, s := range m.DriverNames {
			l = len(s)
			n += 1 + l + sovVolumeMount(uint64(l))
		}
	}
	return n
}

func sovVolumeMount(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozVolumeMount(x uint64) (n int) {
	return sovVolumeMount(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *SharedDevice) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&SharedDevice{`,
		`VolumeId:` + fmt.Sprintf("%v", this.VolumeId) + `,`,
		`MountConfig:` + fmt.Sprintf("%v", this.MountConfig) + `,`,
		`}`,
	}, "")
	return s
}
func (this *VolumeMount) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&VolumeMount{`,
		`Driver:` + fmt.Sprintf("%v", this.Driver) + `,`,
		`ContainerDir:` + fmt.Sprintf("%v", this.ContainerDir) + `,`,
		`Mode:` + fmt.Sprintf("%v", this.Mode) + `,`,
		`Shared:` + strings.Replace(this.Shared.String(), "SharedDevice", "SharedDevice", 1) + `,`,
		`}`,
	}, "")
	return s
}
func (this *VolumePlacement) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&VolumePlacement{`,
		`DriverNames:` + fmt.Sprintf("%v", this.DriverNames) + `,`,
		`}`,
	}, "")
	return s
}
func valueToStringVolumeMount(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
func (m *SharedDevice) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowVolumeMount
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: SharedDevice: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: SharedDevice: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field VolumeId", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVolumeMount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthVolumeMount
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.VolumeId = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field MountConfig", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVolumeMount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthVolumeMount
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.MountConfig = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipVolumeMount(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *VolumeMount) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowVolumeMount
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: VolumeMount: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: VolumeMount: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Driver", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVolumeMount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthVolumeMount
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Driver = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ContainerDir", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVolumeMount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthVolumeMount
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ContainerDir = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 6:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Mode", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVolumeMount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthVolumeMount
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Mode = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 7:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Shared", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVolumeMount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthVolumeMount
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Shared == nil {
				m.Shared = &SharedDevice{}
			}
			if err := m.Shared.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipVolumeMount(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *VolumePlacement) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowVolumeMount
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: VolumePlacement: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: VolumePlacement: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field DriverNames", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowVolumeMount
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthVolumeMount
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.DriverNames = append(m.DriverNames, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipVolumeMount(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthVolumeMount
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipVolumeMount(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowVolumeMount
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowVolumeMount
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowVolumeMount
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthVolumeMount
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupVolumeMount
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthVolumeMount
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthVolumeMount        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowVolumeMount          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupVolumeMount = fmt.Errorf("proto: unexpected end of group")
)
