// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: modification_tag.proto

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

type ModificationTag struct {
	Epoch string `protobuf:"bytes,1,opt,name=epoch,proto3" json:"epoch"`
	Index uint32 `protobuf:"varint,2,opt,name=index,proto3" json:"index"`
}

func (m *ModificationTag) Reset()      { *m = ModificationTag{} }
func (*ModificationTag) ProtoMessage() {}
func (*ModificationTag) Descriptor() ([]byte, []int) {
	return fileDescriptor_b84c9c806e96b4e3, []int{0}
}
func (m *ModificationTag) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ModificationTag) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ModificationTag.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ModificationTag) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ModificationTag.Merge(m, src)
}
func (m *ModificationTag) XXX_Size() int {
	return m.Size()
}
func (m *ModificationTag) XXX_DiscardUnknown() {
	xxx_messageInfo_ModificationTag.DiscardUnknown(m)
}

var xxx_messageInfo_ModificationTag proto.InternalMessageInfo

func (m *ModificationTag) GetEpoch() string {
	if m != nil {
		return m.Epoch
	}
	return ""
}

func (m *ModificationTag) GetIndex() uint32 {
	if m != nil {
		return m.Index
	}
	return 0
}

func init() {
	proto.RegisterType((*ModificationTag)(nil), "models.ModificationTag")
}

func init() { proto.RegisterFile("modification_tag.proto", fileDescriptor_b84c9c806e96b4e3) }

var fileDescriptor_b84c9c806e96b4e3 = []byte{
	// 203 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0xcb, 0xcd, 0x4f, 0xc9,
	0x4c, 0xcb, 0x4c, 0x4e, 0x2c, 0xc9, 0xcc, 0xcf, 0x8b, 0x2f, 0x49, 0x4c, 0xd7, 0x2b, 0x28, 0xca,
	0x2f, 0xc9, 0x17, 0x62, 0xcb, 0xcd, 0x4f, 0x49, 0xcd, 0x29, 0x96, 0xd2, 0x4d, 0xcf, 0x2c, 0xc9,
	0x28, 0x4d, 0xd2, 0x4b, 0xce, 0xcf, 0xd5, 0x4f, 0xcf, 0x4f, 0xcf, 0xd7, 0x07, 0x4b, 0x27, 0x95,
	0xa6, 0x81, 0x79, 0x60, 0x0e, 0x98, 0x05, 0xd1, 0xa6, 0x14, 0xcc, 0xc5, 0xef, 0x8b, 0x64, 0x60,
	0x48, 0x62, 0xba, 0x90, 0x3c, 0x17, 0x6b, 0x6a, 0x41, 0x7e, 0x72, 0x86, 0x04, 0xa3, 0x02, 0xa3,
	0x06, 0xa7, 0x13, 0xe7, 0xab, 0x7b, 0xf2, 0x10, 0x81, 0x20, 0x08, 0x05, 0x52, 0x90, 0x99, 0x97,
	0x92, 0x5a, 0x21, 0xc1, 0xa4, 0xc0, 0xa8, 0xc1, 0x0b, 0x51, 0x00, 0x16, 0x08, 0x82, 0x50, 0x4e,
	0x26, 0x17, 0x1e, 0xca, 0x31, 0xdc, 0x78, 0x28, 0xc7, 0xf0, 0xe1, 0xa1, 0x1c, 0x63, 0xc3, 0x23,
	0x39, 0xc6, 0x15, 0x8f, 0xe4, 0x18, 0x4f, 0x3c, 0x92, 0x63, 0xbc, 0xf0, 0x48, 0x8e, 0xf1, 0xc1,
	0x23, 0x39, 0xc6, 0x17, 0x8f, 0xe4, 0x18, 0x3e, 0x3c, 0x92, 0x63, 0x9c, 0xf0, 0x58, 0x8e, 0xe1,
	0xc2, 0x63, 0x39, 0x86, 0x1b, 0x8f, 0xe5, 0x18, 0x92, 0xd8, 0xc0, 0x2e, 0x32, 0x06, 0x04, 0x00,
	0x00, 0xff, 0xff, 0x12, 0xa6, 0xaa, 0xa3, 0xe2, 0x00, 0x00, 0x00,
}

func (this *ModificationTag) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*ModificationTag)
	if !ok {
		that2, ok := that.(ModificationTag)
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
	if this.Epoch != that1.Epoch {
		return false
	}
	if this.Index != that1.Index {
		return false
	}
	return true
}
func (this *ModificationTag) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 6)
	s = append(s, "&models.ModificationTag{")
	s = append(s, "Epoch: "+fmt.Sprintf("%#v", this.Epoch)+",\n")
	s = append(s, "Index: "+fmt.Sprintf("%#v", this.Index)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func valueToGoStringModificationTag(v interface{}, typ string) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("func(v %v) *%v { return &v } ( %#v )", typ, typ, pv)
}
func (m *ModificationTag) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ModificationTag) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ModificationTag) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Index != 0 {
		i = encodeVarintModificationTag(dAtA, i, uint64(m.Index))
		i--
		dAtA[i] = 0x10
	}
	if len(m.Epoch) > 0 {
		i -= len(m.Epoch)
		copy(dAtA[i:], m.Epoch)
		i = encodeVarintModificationTag(dAtA, i, uint64(len(m.Epoch)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintModificationTag(dAtA []byte, offset int, v uint64) int {
	offset -= sovModificationTag(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *ModificationTag) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Epoch)
	if l > 0 {
		n += 1 + l + sovModificationTag(uint64(l))
	}
	if m.Index != 0 {
		n += 1 + sovModificationTag(uint64(m.Index))
	}
	return n
}

func sovModificationTag(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozModificationTag(x uint64) (n int) {
	return sovModificationTag(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *ModificationTag) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&ModificationTag{`,
		`Epoch:` + fmt.Sprintf("%v", this.Epoch) + `,`,
		`Index:` + fmt.Sprintf("%v", this.Index) + `,`,
		`}`,
	}, "")
	return s
}
func valueToStringModificationTag(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
func (m *ModificationTag) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowModificationTag
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
			return fmt.Errorf("proto: ModificationTag: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ModificationTag: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Epoch", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowModificationTag
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
				return ErrInvalidLengthModificationTag
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthModificationTag
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Epoch = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Index", wireType)
			}
			m.Index = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowModificationTag
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Index |= uint32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipModificationTag(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthModificationTag
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
func skipModificationTag(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowModificationTag
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
					return 0, ErrIntOverflowModificationTag
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
					return 0, ErrIntOverflowModificationTag
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
				return 0, ErrInvalidLengthModificationTag
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupModificationTag
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthModificationTag
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthModificationTag        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowModificationTag          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupModificationTag = fmt.Errorf("proto: unexpected end of group")
)
