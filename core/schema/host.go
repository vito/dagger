package schema

import (
	"context"
	"errors"
	"fmt"

	"github.com/containerd/containerd/labels"
	"github.com/dagger/dagger/core"
	"github.com/dagger/dagger/engine/sources/blob"
	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/vito/dagql"
)

type hostSchema struct {
	srv *dagql.Server
}

var _ SchemaResolvers = &hostSchema{}

func (s *hostSchema) Name() string {
	return "host"
}

func (s *hostSchema) Schema() string {
	return Host
}

func (s *hostSchema) Install() {
	dagql.Fields[*core.Query]{
		dagql.Func("host", func(ctx context.Context, parent *core.Query, args struct{}) (*core.Host, error) {
			return parent.NewHost(), nil
		}),
		dagql.Func("blob", func(ctx context.Context, parent *core.Query, args struct {
			Digest       string `doc:"Digest of the blob"`
			Size         int64  `doc:"Size of the blob"`
			MediaType    string `doc:"Media type of the blob"`
			Uncompressed string `doc:"Digest of the uncompressed blob"`
		}) (*core.Directory, error) {
			blobDef, err := blob.LLB(specs.Descriptor{
				MediaType: args.MediaType,
				Digest:    digest.Digest(args.Digest),
				Size:      int64(args.Size),
				Annotations: map[string]string{
					labels.LabelUncompressed: args.Uncompressed, // TODO ???
				},
			}).Marshal(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal blob source: %s", err)
			}
			return core.NewDirectory(parent, blobDef.ToPB(), "", parent.Platform, nil), nil
		}),
	}.Install(s.srv)

	dagql.Fields[*core.Host]{
		dagql.Func("directory", s.directory).Impure(),
		dagql.Func("file", s.file).Impure(),
		dagql.Func("unixSocket", s.socket),
		dagql.Func("setSecretFile", s.setSecretFile),
		dagql.Func("tunnel", s.tunnel),
		dagql.Func("service", s.service),
	}.Install(s.srv)
}

type setSecretFileArgs struct {
	Name string
	Path string
}

func (s *hostSchema) setSecretFile(ctx context.Context, host *core.Host, args setSecretFileArgs) (*core.Secret, error) {
	return host.SetSecretFile(ctx, args.Name, args.Path)
}

type hostDirectoryArgs struct {
	Path string

	core.CopyFilter
}

func (s *hostSchema) directory(ctx context.Context, host *core.Host, args hostDirectoryArgs) (dagql.Instance[*core.Directory], error) {
	return host.Directory(ctx, s.srv, args.Path, "host.directory", args.CopyFilter)
}

type hostSocketArgs struct {
	Path string
}

func (s *hostSchema) socket(ctx context.Context, host *core.Host, args hostSocketArgs) (*core.Socket, error) {
	return host.Socket(args.Path), nil
}

type hostFileArgs struct {
	Path string
}

func (s *hostSchema) file(ctx context.Context, host *core.Host, args hostFileArgs) (dagql.Instance[*core.File], error) {
	return host.File(ctx, s.srv, args.Path)
}

type hostTunnelArgs struct {
	Service core.ServiceID
	Ports   []dagql.InputObject[core.PortForward] `default:"[]"`
	Native  bool                                  `default:"false"`
}

func (s *hostSchema) tunnel(ctx context.Context, parent *core.Host, args hostTunnelArgs) (*core.Service, error) {
	inst, err := args.Service.Load(ctx, s.srv)
	if err != nil {
		return nil, err
	}

	svc := inst.Self

	if svc.Container == nil {
		return nil, errors.New("tunneling to non-Container services is not supported")
	}

	ports := []core.PortForward{}

	if args.Native {
		for _, port := range svc.Container.Ports {
			ports = append(ports, core.PortForward{
				Frontend: port.Port,
				Backend:  port.Port,
				Protocol: port.Protocol,
			})
		}
	}

	if len(args.Ports) > 0 {
		ports = append(ports, collectInputsSlice(args.Ports)...)
	}

	if len(ports) == 0 {
		for _, port := range svc.Container.Ports {
			ports = append(ports, core.PortForward{
				Frontend: 0, // pick a random port on the host
				Backend:  port.Port,
				Protocol: port.Protocol,
			})
		}
	}

	if len(ports) == 0 {
		return nil, errors.New("no ports to forward")
	}

	return parent.Query.NewTunnelService(inst, ports), nil
}

type hostServiceArgs struct {
	Host  string `default:"localhost"`
	Ports []dagql.InputObject[core.PortForward]
}

func (s *hostSchema) service(ctx context.Context, parent *core.Host, args hostServiceArgs) (*core.Service, error) {
	if len(args.Ports) == 0 {
		return nil, errors.New("no ports specified")
	}

	return parent.Query.NewHostService(args.Host, collectInputsSlice(args.Ports)), nil
}
