// Code generated by protoc-gen-go.
// source: bulletin.proto
// DO NOT EDIT!

/*
Package bulletin is a generated protocol buffer package.

It is generated from these files:
	bulletin.proto

It has these top-level messages:
	WireBulletin
*/
package main

import proto "code.google.com/p/goprotobuf/proto"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = math.Inf

type WireBulletin struct {
	Version          *uint32 `protobuf:"varint,1,req,name=version" json:"version,omitempty"`
	Topic            *string `protobuf:"bytes,2,opt,name=topic" json:"topic,omitempty"`
	Message          *string `protobuf:"bytes,3,opt,name=message" json:"message,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *WireBulletin) Reset()         { *m = WireBulletin{} }
func (m *WireBulletin) String() string { return proto.CompactTextString(m) }
func (*WireBulletin) ProtoMessage()    {}

func (m *WireBulletin) GetVersion() uint32 {
	if m != nil && m.Version != nil {
		return *m.Version
	}
	return 0
}

func (m *WireBulletin) GetTopic() string {
	if m != nil && m.Topic != nil {
		return *m.Topic
	}
	return ""
}

func (m *WireBulletin) GetMessage() string {
	if m != nil && m.Message != nil {
		return *m.Message
	}
	return ""
}

func init() {
}
