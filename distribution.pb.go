// Code generated by protoc-gen-go.
// source: distribution.proto
// DO NOT EDIT!

/*
Package libphonelab is a generated protocol buffer package.

syntax = "proto3";

It is generated from these files:
	distribution.proto

It has these top-level messages:
	DistributionMsg
	DistributionComparison
*/
package libphonelab

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type DistributionMsg struct {
	DeviceId         *string `protobuf:"bytes,1,req,name=DeviceId" json:"DeviceId,omitempty"`
	BootId           *string `protobuf:"bytes,2,req,name=BootId" json:"BootId,omitempty"`
	Temps            []int32 `protobuf:"varint,3,rep,name=Temps" json:"Temps,omitempty"`
	Timestamps       []int64 `protobuf:"varint,4,rep,name=Timestamps" json:"Timestamps,omitempty"`
	Period           *int64  `protobuf:"varint,5,req,name=Period" json:"Period,omitempty"`
	IsFull           *bool   `protobuf:"varint,6,req,name=IsFull" json:"IsFull,omitempty"`
	NumEntries       *int64  `protobuf:"varint,7,req,name=NumEntries" json:"NumEntries,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *DistributionMsg) Reset()                    { *m = DistributionMsg{} }
func (m *DistributionMsg) String() string            { return proto.CompactTextString(m) }
func (*DistributionMsg) ProtoMessage()               {}
func (*DistributionMsg) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *DistributionMsg) GetDeviceId() string {
	if m != nil && m.DeviceId != nil {
		return *m.DeviceId
	}
	return ""
}

func (m *DistributionMsg) GetBootId() string {
	if m != nil && m.BootId != nil {
		return *m.BootId
	}
	return ""
}

func (m *DistributionMsg) GetTemps() []int32 {
	if m != nil {
		return m.Temps
	}
	return nil
}

func (m *DistributionMsg) GetTimestamps() []int64 {
	if m != nil {
		return m.Timestamps
	}
	return nil
}

func (m *DistributionMsg) GetPeriod() int64 {
	if m != nil && m.Period != nil {
		return *m.Period
	}
	return 0
}

func (m *DistributionMsg) GetIsFull() bool {
	if m != nil && m.IsFull != nil {
		return *m.IsFull
	}
	return false
}

func (m *DistributionMsg) GetNumEntries() int64 {
	if m != nil && m.NumEntries != nil {
		return *m.NumEntries
	}
	return 0
}

type DistributionComparison struct {
	Dist1            *DistributionMsg `protobuf:"bytes,1,req,name=dist1" json:"dist1,omitempty"`
	Dist2            *DistributionMsg `protobuf:"bytes,2,req,name=dist2" json:"dist2,omitempty"`
	XXX_unrecognized []byte           `json:"-"`
}

func (m *DistributionComparison) Reset()                    { *m = DistributionComparison{} }
func (m *DistributionComparison) String() string            { return proto.CompactTextString(m) }
func (*DistributionComparison) ProtoMessage()               {}
func (*DistributionComparison) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *DistributionComparison) GetDist1() *DistributionMsg {
	if m != nil {
		return m.Dist1
	}
	return nil
}

func (m *DistributionComparison) GetDist2() *DistributionMsg {
	if m != nil {
		return m.Dist2
	}
	return nil
}

func init() {
	proto.RegisterType((*DistributionMsg)(nil), "libphonelab.DistributionMsg")
	proto.RegisterType((*DistributionComparison)(nil), "libphonelab.DistributionComparison")
}

func init() { proto.RegisterFile("distribution.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 212 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0xe2, 0x12, 0x4a, 0xc9, 0x2c, 0x2e,
	0x29, 0xca, 0x4c, 0x2a, 0x2d, 0xc9, 0xcc, 0xcf, 0xd3, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2,
	0xce, 0xc9, 0x4c, 0x2a, 0xc8, 0xc8, 0xcf, 0x4b, 0xcd, 0x49, 0x4c, 0x52, 0xea, 0x62, 0xe4, 0xe2,
	0x77, 0x41, 0x52, 0xe3, 0x5b, 0x9c, 0x2e, 0x24, 0xc0, 0xc5, 0xe1, 0x92, 0x5a, 0x96, 0x99, 0x9c,
	0xea, 0x99, 0x22, 0xc1, 0xa8, 0xc0, 0xa4, 0xc1, 0x29, 0xc4, 0xc7, 0xc5, 0xe6, 0x94, 0x9f, 0x5f,
	0x02, 0xe4, 0x33, 0x81, 0xf9, 0xbc, 0x5c, 0xac, 0x21, 0xa9, 0xb9, 0x05, 0xc5, 0x12, 0xcc, 0x0a,
	0xcc, 0x1a, 0xac, 0x42, 0x42, 0x5c, 0x5c, 0x21, 0x99, 0xb9, 0xa9, 0xc5, 0x25, 0x89, 0x20, 0x31,
	0x16, 0xa0, 0x18, 0x33, 0x48, 0x4b, 0x40, 0x6a, 0x51, 0x66, 0x7e, 0x8a, 0x04, 0x2b, 0x50, 0x0b,
	0x98, 0xef, 0x59, 0xec, 0x56, 0x9a, 0x93, 0x23, 0xc1, 0x06, 0xe4, 0x73, 0x80, 0xf4, 0xf8, 0x95,
	0xe6, 0xba, 0xe6, 0x01, 0x6d, 0x4e, 0x2d, 0x96, 0x60, 0x07, 0xa9, 0x51, 0x2a, 0xe2, 0x12, 0x43,
	0x76, 0x8b, 0x73, 0x7e, 0x6e, 0x41, 0x62, 0x51, 0x66, 0x71, 0x7e, 0x9e, 0x90, 0x36, 0x17, 0x2b,
	0xc8, 0x27, 0x86, 0x60, 0xf7, 0x70, 0x1b, 0xc9, 0xe8, 0x21, 0xf9, 0x41, 0x0f, 0xdd, 0xfd, 0x50,
	0xc5, 0x46, 0x60, 0xc7, 0x12, 0x50, 0x0c, 0x08, 0x00, 0x00, 0xff, 0xff, 0xfe, 0x2b, 0x45, 0xd4,
	0x22, 0x01, 0x00, 0x00,
}
