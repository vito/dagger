// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: networks.proto

package networks

import (
	context "context"
	fmt "fmt"
	proto "github.com/gogo/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
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

type GetNetworkRequest struct {
	ID string `protobuf:"bytes,1,opt,name=ID,proto3" json:"ID,omitempty"`
}

func (m *GetNetworkRequest) Reset()      { *m = GetNetworkRequest{} }
func (*GetNetworkRequest) ProtoMessage() {}
func (*GetNetworkRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_e43f16ceb713dc88, []int{0}
}
func (m *GetNetworkRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GetNetworkRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GetNetworkRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GetNetworkRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetNetworkRequest.Merge(m, src)
}
func (m *GetNetworkRequest) XXX_Size() int {
	return m.Size()
}
func (m *GetNetworkRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_GetNetworkRequest.DiscardUnknown(m)
}

var xxx_messageInfo_GetNetworkRequest proto.InternalMessageInfo

func (m *GetNetworkRequest) GetID() string {
	if m != nil {
		return m.ID
	}
	return ""
}

type GetNetworkResponse struct {
	Config *Config `protobuf:"bytes,1,opt,name=config,proto3" json:"config,omitempty"`
}

func (m *GetNetworkResponse) Reset()      { *m = GetNetworkResponse{} }
func (*GetNetworkResponse) ProtoMessage() {}
func (*GetNetworkResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_e43f16ceb713dc88, []int{1}
}
func (m *GetNetworkResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GetNetworkResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GetNetworkResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GetNetworkResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetNetworkResponse.Merge(m, src)
}
func (m *GetNetworkResponse) XXX_Size() int {
	return m.Size()
}
func (m *GetNetworkResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_GetNetworkResponse.DiscardUnknown(m)
}

var xxx_messageInfo_GetNetworkResponse proto.InternalMessageInfo

func (m *GetNetworkResponse) GetConfig() *Config {
	if m != nil {
		return m.Config
	}
	return nil
}

type Config struct {
	Dns     *DNSConfig `protobuf:"bytes,1,opt,name=dns,proto3" json:"dns,omitempty"`
	IpHosts []*IPHosts `protobuf:"bytes,2,rep,name=ipHosts,proto3" json:"ipHosts,omitempty"`
}

func (m *Config) Reset()      { *m = Config{} }
func (*Config) ProtoMessage() {}
func (*Config) Descriptor() ([]byte, []int) {
	return fileDescriptor_e43f16ceb713dc88, []int{2}
}
func (m *Config) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Config) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Config.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Config) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Config.Merge(m, src)
}
func (m *Config) XXX_Size() int {
	return m.Size()
}
func (m *Config) XXX_DiscardUnknown() {
	xxx_messageInfo_Config.DiscardUnknown(m)
}

var xxx_messageInfo_Config proto.InternalMessageInfo

func (m *Config) GetDns() *DNSConfig {
	if m != nil {
		return m.Dns
	}
	return nil
}

func (m *Config) GetIpHosts() []*IPHosts {
	if m != nil {
		return m.IpHosts
	}
	return nil
}

type DNSConfig struct {
	Nameservers   []string `protobuf:"bytes,1,rep,name=nameservers,proto3" json:"nameservers,omitempty"`
	Options       []string `protobuf:"bytes,2,rep,name=options,proto3" json:"options,omitempty"`
	SearchDomains []string `protobuf:"bytes,3,rep,name=searchDomains,proto3" json:"searchDomains,omitempty"`
}

func (m *DNSConfig) Reset()      { *m = DNSConfig{} }
func (*DNSConfig) ProtoMessage() {}
func (*DNSConfig) Descriptor() ([]byte, []int) {
	return fileDescriptor_e43f16ceb713dc88, []int{3}
}
func (m *DNSConfig) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *DNSConfig) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_DNSConfig.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *DNSConfig) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DNSConfig.Merge(m, src)
}
func (m *DNSConfig) XXX_Size() int {
	return m.Size()
}
func (m *DNSConfig) XXX_DiscardUnknown() {
	xxx_messageInfo_DNSConfig.DiscardUnknown(m)
}

var xxx_messageInfo_DNSConfig proto.InternalMessageInfo

func (m *DNSConfig) GetNameservers() []string {
	if m != nil {
		return m.Nameservers
	}
	return nil
}

func (m *DNSConfig) GetOptions() []string {
	if m != nil {
		return m.Options
	}
	return nil
}

func (m *DNSConfig) GetSearchDomains() []string {
	if m != nil {
		return m.SearchDomains
	}
	return nil
}

type IPHosts struct {
	Ip    string   `protobuf:"bytes,1,opt,name=ip,proto3" json:"ip,omitempty"`
	Hosts []string `protobuf:"bytes,2,rep,name=hosts,proto3" json:"hosts,omitempty"`
}

func (m *IPHosts) Reset()      { *m = IPHosts{} }
func (*IPHosts) ProtoMessage() {}
func (*IPHosts) Descriptor() ([]byte, []int) {
	return fileDescriptor_e43f16ceb713dc88, []int{4}
}
func (m *IPHosts) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *IPHosts) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_IPHosts.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *IPHosts) XXX_Merge(src proto.Message) {
	xxx_messageInfo_IPHosts.Merge(m, src)
}
func (m *IPHosts) XXX_Size() int {
	return m.Size()
}
func (m *IPHosts) XXX_DiscardUnknown() {
	xxx_messageInfo_IPHosts.DiscardUnknown(m)
}

var xxx_messageInfo_IPHosts proto.InternalMessageInfo

func (m *IPHosts) GetIp() string {
	if m != nil {
		return m.Ip
	}
	return ""
}

func (m *IPHosts) GetHosts() []string {
	if m != nil {
		return m.Hosts
	}
	return nil
}

func init() {
	proto.RegisterType((*GetNetworkRequest)(nil), "moby.buildkit.networks.v1.GetNetworkRequest")
	proto.RegisterType((*GetNetworkResponse)(nil), "moby.buildkit.networks.v1.GetNetworkResponse")
	proto.RegisterType((*Config)(nil), "moby.buildkit.networks.v1.Config")
	proto.RegisterType((*DNSConfig)(nil), "moby.buildkit.networks.v1.DNSConfig")
	proto.RegisterType((*IPHosts)(nil), "moby.buildkit.networks.v1.IPHosts")
}

func init() { proto.RegisterFile("networks.proto", fileDescriptor_e43f16ceb713dc88) }

var fileDescriptor_e43f16ceb713dc88 = []byte{
	// 368 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x52, 0xbd, 0x4e, 0x32, 0x41,
	0x14, 0xdd, 0x61, 0xf3, 0x01, 0x7b, 0xc9, 0x47, 0xf2, 0x4d, 0xbe, 0x62, 0xb5, 0x98, 0xe0, 0x4a,
	0x41, 0xa1, 0x6b, 0xc4, 0xc4, 0xc4, 0xc4, 0x58, 0xe8, 0x26, 0x4a, 0x83, 0x66, 0xed, 0xec, 0xf8,
	0x19, 0x65, 0x82, 0x3b, 0xb3, 0xee, 0x0c, 0x18, 0x1b, 0xe3, 0x23, 0xf8, 0x18, 0x3e, 0x8a, 0x25,
	0x25, 0xa5, 0x0c, 0x8d, 0x25, 0x8f, 0x60, 0xd8, 0x1f, 0xc4, 0x18, 0x89, 0xe5, 0x9c, 0x7b, 0xce,
	0xb9, 0xf7, 0x9e, 0x3b, 0x50, 0xe6, 0x54, 0xdd, 0x8b, 0xa8, 0x2f, 0xdd, 0x30, 0x12, 0x4a, 0xe0,
	0xb5, 0x40, 0xb4, 0x1f, 0xdc, 0xf6, 0x80, 0xdd, 0x76, 0xfb, 0x4c, 0xb9, 0x8b, 0xea, 0x70, 0xd7,
	0xd9, 0x84, 0x7f, 0xa7, 0x54, 0x35, 0x13, 0xc4, 0xa7, 0x77, 0x03, 0x2a, 0x15, 0x2e, 0x43, 0xae,
	0xe1, 0xd9, 0xa8, 0x82, 0x6a, 0x96, 0x9f, 0x6b, 0x78, 0xce, 0x39, 0xe0, 0x65, 0x92, 0x0c, 0x05,
	0x97, 0x14, 0x1f, 0x40, 0xbe, 0x23, 0xf8, 0x35, 0xbb, 0x89, 0x99, 0xa5, 0xfa, 0x86, 0xfb, 0x63,
	0x1b, 0xf7, 0x24, 0x26, 0xfa, 0xa9, 0xc0, 0x79, 0x84, 0x7c, 0x82, 0xe0, 0x7d, 0x30, 0xbb, 0x5c,
	0xa6, 0x0e, 0xd5, 0x15, 0x0e, 0x5e, 0xf3, 0x32, 0x35, 0x99, 0x0b, 0xf0, 0x21, 0x14, 0x58, 0x78,
	0x26, 0xa4, 0x92, 0x76, 0xae, 0x62, 0xd6, 0x4a, 0x75, 0x67, 0x85, 0xb6, 0x71, 0x11, 0x33, 0xfd,
	0x4c, 0xe2, 0x04, 0x60, 0x2d, 0xfc, 0x70, 0x05, 0x4a, 0xbc, 0x15, 0x50, 0x49, 0xa3, 0x21, 0x8d,
	0xe6, 0xa3, 0x98, 0x35, 0xcb, 0x5f, 0x86, 0xb0, 0x0d, 0x05, 0x11, 0x2a, 0x26, 0x78, 0xd2, 0xcc,
	0xf2, 0xb3, 0x27, 0xae, 0xc2, 0x5f, 0x49, 0x5b, 0x51, 0xa7, 0xe7, 0x89, 0xa0, 0xc5, 0xb8, 0xb4,
	0xcd, 0xb8, 0xfe, 0x15, 0x74, 0x76, 0xa0, 0x90, 0x8e, 0x30, 0x8f, 0x96, 0x85, 0x59, 0xb4, 0x2c,
	0xc4, 0xff, 0xe1, 0x4f, 0x6f, 0xb1, 0x85, 0xe5, 0x27, 0x8f, 0xfa, 0x00, 0x8a, 0x69, 0xda, 0x12,
	0x33, 0x80, 0xcf, 0xf0, 0xf1, 0xd6, 0x8a, 0x35, 0xbf, 0x1d, 0x72, 0x7d, 0xfb, 0x97, 0xec, 0xe4,
	0xa2, 0xc7, 0x47, 0xa3, 0x09, 0x31, 0xc6, 0x13, 0x62, 0xcc, 0x26, 0x04, 0x3d, 0x69, 0x82, 0x5e,
	0x34, 0x41, 0xaf, 0x9a, 0xa0, 0x91, 0x26, 0xe8, 0x4d, 0x13, 0xf4, 0xae, 0x89, 0x31, 0xd3, 0x04,
	0x3d, 0x4f, 0x89, 0x31, 0x9a, 0x12, 0x63, 0x3c, 0x25, 0xc6, 0x55, 0x31, 0x73, 0x6d, 0xe7, 0xe3,
	0xef, 0xb6, 0xf7, 0x11, 0x00, 0x00, 0xff, 0xff, 0x7a, 0x5d, 0xaa, 0x1a, 0x80, 0x02, 0x00, 0x00,
}

func (this *GetNetworkRequest) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*GetNetworkRequest)
	if !ok {
		that2, ok := that.(GetNetworkRequest)
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
	if this.ID != that1.ID {
		return false
	}
	return true
}
func (this *GetNetworkResponse) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*GetNetworkResponse)
	if !ok {
		that2, ok := that.(GetNetworkResponse)
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
	if !this.Config.Equal(that1.Config) {
		return false
	}
	return true
}
func (this *Config) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Config)
	if !ok {
		that2, ok := that.(Config)
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
	if !this.Dns.Equal(that1.Dns) {
		return false
	}
	if len(this.IpHosts) != len(that1.IpHosts) {
		return false
	}
	for i := range this.IpHosts {
		if !this.IpHosts[i].Equal(that1.IpHosts[i]) {
			return false
		}
	}
	return true
}
func (this *DNSConfig) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*DNSConfig)
	if !ok {
		that2, ok := that.(DNSConfig)
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
	if len(this.Nameservers) != len(that1.Nameservers) {
		return false
	}
	for i := range this.Nameservers {
		if this.Nameservers[i] != that1.Nameservers[i] {
			return false
		}
	}
	if len(this.Options) != len(that1.Options) {
		return false
	}
	for i := range this.Options {
		if this.Options[i] != that1.Options[i] {
			return false
		}
	}
	if len(this.SearchDomains) != len(that1.SearchDomains) {
		return false
	}
	for i := range this.SearchDomains {
		if this.SearchDomains[i] != that1.SearchDomains[i] {
			return false
		}
	}
	return true
}
func (this *IPHosts) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*IPHosts)
	if !ok {
		that2, ok := that.(IPHosts)
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
	if this.Ip != that1.Ip {
		return false
	}
	if len(this.Hosts) != len(that1.Hosts) {
		return false
	}
	for i := range this.Hosts {
		if this.Hosts[i] != that1.Hosts[i] {
			return false
		}
	}
	return true
}
func (this *GetNetworkRequest) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 5)
	s = append(s, "&networks.GetNetworkRequest{")
	s = append(s, "ID: "+fmt.Sprintf("%#v", this.ID)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *GetNetworkResponse) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 5)
	s = append(s, "&networks.GetNetworkResponse{")
	if this.Config != nil {
		s = append(s, "Config: "+fmt.Sprintf("%#v", this.Config)+",\n")
	}
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *Config) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 6)
	s = append(s, "&networks.Config{")
	if this.Dns != nil {
		s = append(s, "Dns: "+fmt.Sprintf("%#v", this.Dns)+",\n")
	}
	if this.IpHosts != nil {
		s = append(s, "IpHosts: "+fmt.Sprintf("%#v", this.IpHosts)+",\n")
	}
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *DNSConfig) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 7)
	s = append(s, "&networks.DNSConfig{")
	s = append(s, "Nameservers: "+fmt.Sprintf("%#v", this.Nameservers)+",\n")
	s = append(s, "Options: "+fmt.Sprintf("%#v", this.Options)+",\n")
	s = append(s, "SearchDomains: "+fmt.Sprintf("%#v", this.SearchDomains)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *IPHosts) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 6)
	s = append(s, "&networks.IPHosts{")
	s = append(s, "Ip: "+fmt.Sprintf("%#v", this.Ip)+",\n")
	s = append(s, "Hosts: "+fmt.Sprintf("%#v", this.Hosts)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func valueToGoStringNetworks(v interface{}, typ string) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("func(v %v) *%v { return &v } ( %#v )", typ, typ, pv)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// NetworksClient is the client API for Networks service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type NetworksClient interface {
	GetNetwork(ctx context.Context, in *GetNetworkRequest, opts ...grpc.CallOption) (*GetNetworkResponse, error)
}

type networksClient struct {
	cc *grpc.ClientConn
}

func NewNetworksClient(cc *grpc.ClientConn) NetworksClient {
	return &networksClient{cc}
}

func (c *networksClient) GetNetwork(ctx context.Context, in *GetNetworkRequest, opts ...grpc.CallOption) (*GetNetworkResponse, error) {
	out := new(GetNetworkResponse)
	err := c.cc.Invoke(ctx, "/moby.buildkit.networks.v1.Networks/GetNetwork", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// NetworksServer is the server API for Networks service.
type NetworksServer interface {
	GetNetwork(context.Context, *GetNetworkRequest) (*GetNetworkResponse, error)
}

// UnimplementedNetworksServer can be embedded to have forward compatible implementations.
type UnimplementedNetworksServer struct {
}

func (*UnimplementedNetworksServer) GetNetwork(ctx context.Context, req *GetNetworkRequest) (*GetNetworkResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetNetwork not implemented")
}

func RegisterNetworksServer(s *grpc.Server, srv NetworksServer) {
	s.RegisterService(&_Networks_serviceDesc, srv)
}

func _Networks_GetNetwork_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetNetworkRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetworksServer).GetNetwork(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/moby.buildkit.networks.v1.Networks/GetNetwork",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetworksServer).GetNetwork(ctx, req.(*GetNetworkRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Networks_serviceDesc = grpc.ServiceDesc{
	ServiceName: "moby.buildkit.networks.v1.Networks",
	HandlerType: (*NetworksServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetNetwork",
			Handler:    _Networks_GetNetwork_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "networks.proto",
}

func (m *GetNetworkRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GetNetworkRequest) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GetNetworkRequest) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.ID) > 0 {
		i -= len(m.ID)
		copy(dAtA[i:], m.ID)
		i = encodeVarintNetworks(dAtA, i, uint64(len(m.ID)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *GetNetworkResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GetNetworkResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GetNetworkResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Config != nil {
		{
			size, err := m.Config.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintNetworks(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *Config) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Config) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Config) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.IpHosts) > 0 {
		for iNdEx := len(m.IpHosts) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.IpHosts[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintNetworks(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x12
		}
	}
	if m.Dns != nil {
		{
			size, err := m.Dns.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintNetworks(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *DNSConfig) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *DNSConfig) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *DNSConfig) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.SearchDomains) > 0 {
		for iNdEx := len(m.SearchDomains) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.SearchDomains[iNdEx])
			copy(dAtA[i:], m.SearchDomains[iNdEx])
			i = encodeVarintNetworks(dAtA, i, uint64(len(m.SearchDomains[iNdEx])))
			i--
			dAtA[i] = 0x1a
		}
	}
	if len(m.Options) > 0 {
		for iNdEx := len(m.Options) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.Options[iNdEx])
			copy(dAtA[i:], m.Options[iNdEx])
			i = encodeVarintNetworks(dAtA, i, uint64(len(m.Options[iNdEx])))
			i--
			dAtA[i] = 0x12
		}
	}
	if len(m.Nameservers) > 0 {
		for iNdEx := len(m.Nameservers) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.Nameservers[iNdEx])
			copy(dAtA[i:], m.Nameservers[iNdEx])
			i = encodeVarintNetworks(dAtA, i, uint64(len(m.Nameservers[iNdEx])))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *IPHosts) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *IPHosts) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *IPHosts) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Hosts) > 0 {
		for iNdEx := len(m.Hosts) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.Hosts[iNdEx])
			copy(dAtA[i:], m.Hosts[iNdEx])
			i = encodeVarintNetworks(dAtA, i, uint64(len(m.Hosts[iNdEx])))
			i--
			dAtA[i] = 0x12
		}
	}
	if len(m.Ip) > 0 {
		i -= len(m.Ip)
		copy(dAtA[i:], m.Ip)
		i = encodeVarintNetworks(dAtA, i, uint64(len(m.Ip)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintNetworks(dAtA []byte, offset int, v uint64) int {
	offset -= sovNetworks(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *GetNetworkRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.ID)
	if l > 0 {
		n += 1 + l + sovNetworks(uint64(l))
	}
	return n
}

func (m *GetNetworkResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Config != nil {
		l = m.Config.Size()
		n += 1 + l + sovNetworks(uint64(l))
	}
	return n
}

func (m *Config) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Dns != nil {
		l = m.Dns.Size()
		n += 1 + l + sovNetworks(uint64(l))
	}
	if len(m.IpHosts) > 0 {
		for _, e := range m.IpHosts {
			l = e.Size()
			n += 1 + l + sovNetworks(uint64(l))
		}
	}
	return n
}

func (m *DNSConfig) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Nameservers) > 0 {
		for _, s := range m.Nameservers {
			l = len(s)
			n += 1 + l + sovNetworks(uint64(l))
		}
	}
	if len(m.Options) > 0 {
		for _, s := range m.Options {
			l = len(s)
			n += 1 + l + sovNetworks(uint64(l))
		}
	}
	if len(m.SearchDomains) > 0 {
		for _, s := range m.SearchDomains {
			l = len(s)
			n += 1 + l + sovNetworks(uint64(l))
		}
	}
	return n
}

func (m *IPHosts) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Ip)
	if l > 0 {
		n += 1 + l + sovNetworks(uint64(l))
	}
	if len(m.Hosts) > 0 {
		for _, s := range m.Hosts {
			l = len(s)
			n += 1 + l + sovNetworks(uint64(l))
		}
	}
	return n
}

func sovNetworks(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozNetworks(x uint64) (n int) {
	return sovNetworks(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *GetNetworkRequest) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&GetNetworkRequest{`,
		`ID:` + fmt.Sprintf("%v", this.ID) + `,`,
		`}`,
	}, "")
	return s
}
func (this *GetNetworkResponse) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&GetNetworkResponse{`,
		`Config:` + strings.Replace(this.Config.String(), "Config", "Config", 1) + `,`,
		`}`,
	}, "")
	return s
}
func (this *Config) String() string {
	if this == nil {
		return "nil"
	}
	repeatedStringForIpHosts := "[]*IPHosts{"
	for _, f := range this.IpHosts {
		repeatedStringForIpHosts += strings.Replace(f.String(), "IPHosts", "IPHosts", 1) + ","
	}
	repeatedStringForIpHosts += "}"
	s := strings.Join([]string{`&Config{`,
		`Dns:` + strings.Replace(this.Dns.String(), "DNSConfig", "DNSConfig", 1) + `,`,
		`IpHosts:` + repeatedStringForIpHosts + `,`,
		`}`,
	}, "")
	return s
}
func (this *DNSConfig) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&DNSConfig{`,
		`Nameservers:` + fmt.Sprintf("%v", this.Nameservers) + `,`,
		`Options:` + fmt.Sprintf("%v", this.Options) + `,`,
		`SearchDomains:` + fmt.Sprintf("%v", this.SearchDomains) + `,`,
		`}`,
	}, "")
	return s
}
func (this *IPHosts) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&IPHosts{`,
		`Ip:` + fmt.Sprintf("%v", this.Ip) + `,`,
		`Hosts:` + fmt.Sprintf("%v", this.Hosts) + `,`,
		`}`,
	}, "")
	return s
}
func valueToStringNetworks(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
func (m *GetNetworkRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowNetworks
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
			return fmt.Errorf("proto: GetNetworkRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GetNetworkRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ID", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowNetworks
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
				return ErrInvalidLengthNetworks
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthNetworks
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ID = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipNetworks(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthNetworks
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
func (m *GetNetworkResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowNetworks
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
			return fmt.Errorf("proto: GetNetworkResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GetNetworkResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Config", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowNetworks
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
				return ErrInvalidLengthNetworks
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthNetworks
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Config == nil {
				m.Config = &Config{}
			}
			if err := m.Config.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipNetworks(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthNetworks
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
func (m *Config) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowNetworks
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
			return fmt.Errorf("proto: Config: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Config: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Dns", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowNetworks
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
				return ErrInvalidLengthNetworks
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthNetworks
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Dns == nil {
				m.Dns = &DNSConfig{}
			}
			if err := m.Dns.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field IpHosts", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowNetworks
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
				return ErrInvalidLengthNetworks
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthNetworks
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.IpHosts = append(m.IpHosts, &IPHosts{})
			if err := m.IpHosts[len(m.IpHosts)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipNetworks(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthNetworks
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
func (m *DNSConfig) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowNetworks
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
			return fmt.Errorf("proto: DNSConfig: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: DNSConfig: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Nameservers", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowNetworks
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
				return ErrInvalidLengthNetworks
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthNetworks
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Nameservers = append(m.Nameservers, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Options", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowNetworks
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
				return ErrInvalidLengthNetworks
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthNetworks
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Options = append(m.Options, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field SearchDomains", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowNetworks
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
				return ErrInvalidLengthNetworks
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthNetworks
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.SearchDomains = append(m.SearchDomains, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipNetworks(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthNetworks
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
func (m *IPHosts) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowNetworks
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
			return fmt.Errorf("proto: IPHosts: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: IPHosts: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Ip", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowNetworks
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
				return ErrInvalidLengthNetworks
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthNetworks
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Ip = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Hosts", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowNetworks
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
				return ErrInvalidLengthNetworks
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthNetworks
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Hosts = append(m.Hosts, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipNetworks(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthNetworks
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
func skipNetworks(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowNetworks
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
					return 0, ErrIntOverflowNetworks
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
					return 0, ErrIntOverflowNetworks
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
				return 0, ErrInvalidLengthNetworks
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupNetworks
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthNetworks
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthNetworks        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowNetworks          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupNetworks = fmt.Errorf("proto: unexpected end of group")
)
