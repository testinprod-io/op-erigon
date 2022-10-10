// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0-devel
// 	protoc        v3.14.0
// source: sentinel.proto

package lightrpc

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GossipType int32

const (
	// Lightclient gossip
	GossipType_LightClientFinalityUpdateGossipType   GossipType = 0
	GossipType_LightClientOptimisticUpdateGossipType GossipType = 1
	// Legacy gossip
	GossipType_BeaconBlockGossipType GossipType = 2
)

// Enum value maps for GossipType.
var (
	GossipType_name = map[int32]string{
		0: "LightClientFinalityUpdateGossipType",
		1: "LightClientOptimisticUpdateGossipType",
		2: "BeaconBlockGossipType",
	}
	GossipType_value = map[string]int32{
		"LightClientFinalityUpdateGossipType":   0,
		"LightClientOptimisticUpdateGossipType": 1,
		"BeaconBlockGossipType":                 2,
	}
)

func (x GossipType) Enum() *GossipType {
	p := new(GossipType)
	*p = x
	return p
}

func (x GossipType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (GossipType) Descriptor() protoreflect.EnumDescriptor {
	return file_sentinel_proto_enumTypes[0].Descriptor()
}

func (GossipType) Type() protoreflect.EnumType {
	return &file_sentinel_proto_enumTypes[0]
}

func (x GossipType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use GossipType.Descriptor instead.
func (GossipType) EnumDescriptor() ([]byte, []int) {
	return file_sentinel_proto_rawDescGZIP(), []int{0}
}

type GossipRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GossipRequest) Reset() {
	*x = GossipRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sentinel_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GossipRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GossipRequest) ProtoMessage() {}

func (x *GossipRequest) ProtoReflect() protoreflect.Message {
	mi := &file_sentinel_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GossipRequest.ProtoReflect.Descriptor instead.
func (*GossipRequest) Descriptor() ([]byte, []int) {
	return file_sentinel_proto_rawDescGZIP(), []int{0}
}

type GossipData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Data []byte     `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"` // SSZ encoded data
	Type GossipType `protobuf:"varint,2,opt,name=type,proto3,enum=lightrpc.GossipType" json:"type,omitempty"`
}

func (x *GossipData) Reset() {
	*x = GossipData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sentinel_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GossipData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GossipData) ProtoMessage() {}

func (x *GossipData) ProtoReflect() protoreflect.Message {
	mi := &file_sentinel_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GossipData.ProtoReflect.Descriptor instead.
func (*GossipData) Descriptor() ([]byte, []int) {
	return file_sentinel_proto_rawDescGZIP(), []int{1}
}

func (x *GossipData) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *GossipData) GetType() GossipType {
	if x != nil {
		return x.Type
	}
	return GossipType_LightClientFinalityUpdateGossipType
}

type RequestData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Data  []byte `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"` // SSZ encoded data
	Topic string `protobuf:"bytes,2,opt,name=topic,proto3" json:"topic,omitempty"`
}

func (x *RequestData) Reset() {
	*x = RequestData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sentinel_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RequestData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RequestData) ProtoMessage() {}

func (x *RequestData) ProtoReflect() protoreflect.Message {
	mi := &file_sentinel_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RequestData.ProtoReflect.Descriptor instead.
func (*RequestData) Descriptor() ([]byte, []int) {
	return file_sentinel_proto_rawDescGZIP(), []int{2}
}

func (x *RequestData) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *RequestData) GetTopic() string {
	if x != nil {
		return x.Topic
	}
	return ""
}

type ResponseData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Data  []byte `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`    // prefix-stripped SSZ encoded data
	Error bool   `protobuf:"varint,2,opt,name=error,proto3" json:"error,omitempty"` // did the peer encounter an error
}

func (x *ResponseData) Reset() {
	*x = ResponseData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_sentinel_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ResponseData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ResponseData) ProtoMessage() {}

func (x *ResponseData) ProtoReflect() protoreflect.Message {
	mi := &file_sentinel_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ResponseData.ProtoReflect.Descriptor instead.
func (*ResponseData) Descriptor() ([]byte, []int) {
	return file_sentinel_proto_rawDescGZIP(), []int{3}
}

func (x *ResponseData) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *ResponseData) GetError() bool {
	if x != nil {
		return x.Error
	}
	return false
}

var File_sentinel_proto protoreflect.FileDescriptor

var file_sentinel_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x73, 0x65, 0x6e, 0x74, 0x69, 0x6e, 0x65, 0x6c, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x08, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x72, 0x70, 0x63, 0x1a, 0x12, 0x62, 0x65, 0x61, 0x63,
	0x6f, 0x6e, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x0f,
	0x0a, 0x0d, 0x47, 0x6f, 0x73, 0x73, 0x69, 0x70, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22,
	0x4a, 0x0a, 0x0a, 0x47, 0x6f, 0x73, 0x73, 0x69, 0x70, 0x44, 0x61, 0x74, 0x61, 0x12, 0x12, 0x0a,
	0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74,
	0x61, 0x12, 0x28, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32,
	0x14, 0x2e, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x72, 0x70, 0x63, 0x2e, 0x47, 0x6f, 0x73, 0x73, 0x69,
	0x70, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x22, 0x37, 0x0a, 0x0b, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x44, 0x61, 0x74, 0x61, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61,
	0x74, 0x61, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12, 0x14,
	0x0a, 0x05, 0x74, 0x6f, 0x70, 0x69, 0x63, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74,
	0x6f, 0x70, 0x69, 0x63, 0x22, 0x38, 0x0a, 0x0c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x44, 0x61, 0x74, 0x61, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x12, 0x14, 0x0a, 0x05, 0x65, 0x72, 0x72, 0x6f,
	0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x2a, 0x7b,
	0x0a, 0x0a, 0x47, 0x6f, 0x73, 0x73, 0x69, 0x70, 0x54, 0x79, 0x70, 0x65, 0x12, 0x27, 0x0a, 0x23,
	0x4c, 0x69, 0x67, 0x68, 0x74, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x46, 0x69, 0x6e, 0x61, 0x6c,
	0x69, 0x74, 0x79, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x47, 0x6f, 0x73, 0x73, 0x69, 0x70, 0x54,
	0x79, 0x70, 0x65, 0x10, 0x00, 0x12, 0x29, 0x0a, 0x25, 0x4c, 0x69, 0x67, 0x68, 0x74, 0x43, 0x6c,
	0x69, 0x65, 0x6e, 0x74, 0x4f, 0x70, 0x74, 0x69, 0x6d, 0x69, 0x73, 0x74, 0x69, 0x63, 0x55, 0x70,
	0x64, 0x61, 0x74, 0x65, 0x47, 0x6f, 0x73, 0x73, 0x69, 0x70, 0x54, 0x79, 0x70, 0x65, 0x10, 0x01,
	0x12, 0x19, 0x0a, 0x15, 0x42, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x47,
	0x6f, 0x73, 0x73, 0x69, 0x70, 0x54, 0x79, 0x70, 0x65, 0x10, 0x02, 0x32, 0x8c, 0x01, 0x0a, 0x08,
	0x53, 0x65, 0x6e, 0x74, 0x69, 0x6e, 0x65, 0x6c, 0x12, 0x42, 0x0a, 0x0f, 0x53, 0x75, 0x62, 0x73,
	0x63, 0x72, 0x69, 0x62, 0x65, 0x47, 0x6f, 0x73, 0x73, 0x69, 0x70, 0x12, 0x17, 0x2e, 0x6c, 0x69,
	0x67, 0x68, 0x74, 0x72, 0x70, 0x63, 0x2e, 0x47, 0x6f, 0x73, 0x73, 0x69, 0x70, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x14, 0x2e, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x72, 0x70, 0x63, 0x2e,
	0x47, 0x6f, 0x73, 0x73, 0x69, 0x70, 0x44, 0x61, 0x74, 0x61, 0x30, 0x01, 0x12, 0x3c, 0x0a, 0x0b,
	0x53, 0x65, 0x6e, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x15, 0x2e, 0x6c, 0x69,
	0x67, 0x68, 0x74, 0x72, 0x70, 0x63, 0x2e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x44, 0x61,
	0x74, 0x61, 0x1a, 0x16, 0x2e, 0x6c, 0x69, 0x67, 0x68, 0x74, 0x72, 0x70, 0x63, 0x2e, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x44, 0x61, 0x74, 0x61, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_sentinel_proto_rawDescOnce sync.Once
	file_sentinel_proto_rawDescData = file_sentinel_proto_rawDesc
)

func file_sentinel_proto_rawDescGZIP() []byte {
	file_sentinel_proto_rawDescOnce.Do(func() {
		file_sentinel_proto_rawDescData = protoimpl.X.CompressGZIP(file_sentinel_proto_rawDescData)
	})
	return file_sentinel_proto_rawDescData
}

var file_sentinel_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_sentinel_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_sentinel_proto_goTypes = []interface{}{
	(GossipType)(0),       // 0: lightrpc.GossipType
	(*GossipRequest)(nil), // 1: lightrpc.GossipRequest
	(*GossipData)(nil),    // 2: lightrpc.GossipData
	(*RequestData)(nil),   // 3: lightrpc.RequestData
	(*ResponseData)(nil),  // 4: lightrpc.ResponseData
}
var file_sentinel_proto_depIdxs = []int32{
	0, // 0: lightrpc.GossipData.type:type_name -> lightrpc.GossipType
	1, // 1: lightrpc.Sentinel.SubscribeGossip:input_type -> lightrpc.GossipRequest
	3, // 2: lightrpc.Sentinel.SendRequest:input_type -> lightrpc.RequestData
	2, // 3: lightrpc.Sentinel.SubscribeGossip:output_type -> lightrpc.GossipData
	4, // 4: lightrpc.Sentinel.SendRequest:output_type -> lightrpc.ResponseData
	3, // [3:5] is the sub-list for method output_type
	1, // [1:3] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_sentinel_proto_init() }
func file_sentinel_proto_init() {
	if File_sentinel_proto != nil {
		return
	}
	file_beacon_block_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_sentinel_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GossipRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_sentinel_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GossipData); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_sentinel_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RequestData); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_sentinel_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ResponseData); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_sentinel_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_sentinel_proto_goTypes,
		DependencyIndexes: file_sentinel_proto_depIdxs,
		EnumInfos:         file_sentinel_proto_enumTypes,
		MessageInfos:      file_sentinel_proto_msgTypes,
	}.Build()
	File_sentinel_proto = out.File
	file_sentinel_proto_rawDesc = nil
	file_sentinel_proto_goTypes = nil
	file_sentinel_proto_depIdxs = nil
}