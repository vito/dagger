package core

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"time"

	containerdfs "github.com/containerd/continuity/fs"
	bkcache "github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/client/llb"
	bkgw "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/solver/pb"
	"github.com/opencontainers/go-digest"
	fstypes "github.com/tonistiigi/fsutil/types"
	"github.com/vektah/gqlparser/v2/ast"

	"dagger.io/dagger/telemetry"
	"github.com/dagger/dagger/core/reffs"
	"github.com/dagger/dagger/dagql"
	"github.com/dagger/dagger/dagql/call"
	"github.com/dagger/dagger/engine/buildkit"
)

// File is a content-addressed file.
type File struct {
	RawLLB *pb.Definition
	Result bkcache.ImmutableRef // only valid when returned by dagop

	File     string
	Platform Platform

	// Services necessary to provision the file.
	Services ServiceBindings
}

func (file *File) LLB(ctx context.Context) (*pb.Definition, error) {
	if file.RawLLB != nil {
		return file.RawLLB, nil
	}
	if file.Result != nil {
		op, err := newDagOpLLB(ctx,
			&ImmutableRefDagOp{
				Ref: file.Result.ID(),
			},
			call.New().Append(
				&ast.Type{
					NamedType: "Directory",
					NonNull:   true,
				},
				"__immutableRef",
				"",
				nil,
				0,
				"",
				// NB: doesnt actually matter
				call.NewArgument("ref", call.NewLiteralString(file.Result.ID()), false),
			),
			nil, // TODO: no inputs? or, use layer chain??
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create file LLB: %w", err)
		}
		def, err := op.Marshal(ctx, llb.Platform(file.Platform.Spec()))
		if err != nil {
			return nil, fmt.Errorf("failed to marshal file LLB: %w", err)
		}
		return def.ToPB(), nil
	}
	return nil, nil
}

func (*File) Type() *ast.Type {
	return &ast.Type{
		NamedType: "File",
		NonNull:   true,
	}
}

func (*File) TypeDescription() string {
	return "A file."
}

func (dir *File) getResult() bkcache.ImmutableRef {
	return dir.Result
}
func (dir *File) setResult(ref bkcache.ImmutableRef) {
	dir.Result = ref
}

var _ HasPBDefinitions = (*File)(nil)

func (dir *File) PBDefinitions(ctx context.Context) ([]*pb.Definition, error) {
	var defs []*pb.Definition
	def, err := dir.LLB(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get file LLB: %w", err)
	}
	if def != nil {
		defs = append(defs, def)
	}
	for _, bnd := range dir.Services {
		ctr := bnd.Service.Self().Container
		if ctr == nil {
			continue
		}
		ctrDefs, err := ctr.PBDefinitions(ctx)
		if err != nil {
			return nil, err
		}
		defs = append(defs, ctrDefs...)
	}
	return defs, nil
}

var _ dagql.OnReleaser = (*File)(nil)

func (dir *File) OnRelease(ctx context.Context) error {
	if dir.Result != nil {
		return dir.Result.Release(ctx)
	}
	return nil
}

func NewFile(def *pb.Definition, file string, platform Platform, services ServiceBindings) *File {
	return &File{
		RawLLB:   def,
		File:     file,
		Platform: platform,
		Services: services,
	}
}

func NewFileWithContents(
	ctx context.Context,
	name string,
	content []byte,
	permissions fs.FileMode,
	ownership *Ownership,
	platform Platform,
) (*File, error) {
	if dir, _ := filepath.Split(name); dir != "" {
		return nil, fmt.Errorf("file name %q must not contain a directory", name)
	}
	dir, err := NewScratchDirectory(ctx, platform)
	if err != nil {
		return nil, err
	}
	dir, err = dir.WithNewFile(ctx, name, content, permissions, ownership)
	if err != nil {
		return nil, err
	}
	return dir.File(ctx, name)
}

func NewFileSt(ctx context.Context, st llb.State, file string, platform Platform, services ServiceBindings) (*File, error) {
	def, err := st.Marshal(ctx, llb.Platform(platform.Spec()))
	if err != nil {
		return nil, err
	}

	return NewFile(def.ToPB(), file, platform, services), nil
}

// Clone returns a deep copy of the container suitable for modifying in a
// WithXXX method.
func (dir *File) Clone() *File {
	cp := *dir
	cp.Services = slices.Clone(cp.Services)
	return &cp
}

func (dir *File) State(ctx context.Context) (llb.State, error) {
	def, err := dir.LLB(ctx)
	if err != nil {
		return llb.State{}, fmt.Errorf("failed to get file LLB: %w", err)
	}
	return defToState(def)
}

func (dir *File) Evaluate(ctx context.Context) (*buildkit.Result, error) {
	query, err := CurrentQuery(ctx)
	if err != nil {
		return nil, err
	}
	llb, err := dir.LLB(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get file LLB: %w", err)
	}
	bk, err := query.Buildkit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get buildkit client: %w", err)
	}

	return bk.Solve(ctx, bkgw.SolveRequest{
		Evaluate:   true,
		Definition: llb,
	})
}

// Contents handles file content retrieval
func (dir *File) Contents(ctx context.Context) ([]byte, error) {
	query, err := CurrentQuery(ctx)
	if err != nil {
		return nil, err
	}
	bk, err := query.Buildkit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get buildkit client: %w", err)
	}

	llbSt, err := dir.State(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get file state: %w", err)
	}
	def, err := llbSt.Marshal(ctx, llb.Platform(dir.Platform.Spec()))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal file state: %w", err)
	}
	ref, err := bkRef(ctx, bk, def.ToPB())
	if err != nil {
		return nil, err
	}

	// Stat the file and preallocate file contents buffer:
	st, err := dir.Stat(ctx)
	if err != nil {
		return nil, err
	}

	// Error on files that exceed MaxFileContentsSize:
	fileSize := int(st.GetSize_())
	if fileSize > buildkit.MaxFileContentsSize {
		// TODO: move to proper error structure
		return nil, fmt.Errorf("file size %d exceeds limit %d", fileSize, buildkit.MaxFileContentsSize)
	}

	// Allocate buffer with the given file size:
	contents := make([]byte, fileSize)

	// Use a chunked reader to overcome issues when
	// the input file exceeds MaxFileContentsChunkSize:
	var offset int
	for offset < fileSize {
		chunk, err := ref.ReadFile(ctx, bkgw.ReadRequest{
			Filename: dir.File,
			Range: &bkgw.FileRange{
				Offset: offset,
				Length: buildkit.MaxFileContentsChunkSize,
			},
		})
		if err != nil {
			return nil, err
		}

		// Copy the chunk and increment offset for subsequent reads:
		copy(contents[offset:], chunk)
		offset += len(chunk)
	}
	return contents, nil
}

func (dir *File) Digest(ctx context.Context, excludeMetadata bool) (string, error) {
	// If metadata are included, directly compute the digest of the file
	if !excludeMetadata {
		result, err := dir.Evaluate(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to evaluate file: %w", err)
		}

		digest, err := result.Ref.Digest(ctx, dir.File)
		if err != nil {
			return "", fmt.Errorf("failed to compute digest: %w", err)
		}

		return digest.String(), nil
	}

	// If metadata are excluded, compute the digest of the file from its content.
	reader, err := dir.Open(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to open file to compute digest: %w", err)
	}

	defer reader.Close()

	h := sha256.New()
	if _, err := io.Copy(h, reader); err != nil {
		return "", fmt.Errorf("failed to copy file content into hasher: %w", err)
	}

	return digest.FromBytes(h.Sum(nil)).String(), nil
}

func (file *File) Stat(ctx context.Context) (*fstypes.Stat, error) {
	query, err := CurrentQuery(ctx)
	if err != nil {
		return nil, err
	}
	bk, err := query.Buildkit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get buildkit client: %w", err)
	}

	def, err := file.LLB(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get file LLB: %w", err)
	}
	ref, err := bkRef(ctx, bk, def)
	if err != nil {
		return nil, err
	}

	return ref.StatFile(ctx, bkgw.StatRequest{
		Path: file.File,
	})
}

func (dir *File) WithName(ctx context.Context, filename string) (*File, error) {
	// Clone the file
	dir = dir.Clone()

	st, err := dir.State(ctx)
	if err != nil {
		return nil, err
	}

	// Create a new file with the new name
	newFile := llb.Scratch().File(llb.Copy(st, dir.File, path.Base(filename)))

	def, err := newFile.Marshal(ctx, llb.Platform(dir.Platform.Spec()))
	if err != nil {
		return nil, err
	}

	dir.RawLLB = def.ToPB()
	dir.File = path.Base(filename)

	return dir, nil
}

func (dir *File) WithTimestamps(ctx context.Context, unix int) (*File, error) {
	dir = dir.Clone()
	return execInMount(ctx, dir, func(root string) error {
		fullPath, err := RootPathWithoutFinalSymlink(root, dir.File)
		if err != nil {
			return err
		}
		t := time.Unix(int64(unix), 0)
		err = os.Chtimes(fullPath, t, t)
		if err != nil {
			return err
		}
		return nil
	}, withSavedSnapshot("withTimestamps %d", unix))
}

func (dir *File) Open(ctx context.Context) (io.ReadCloser, error) {
	query, err := CurrentQuery(ctx)
	if err != nil {
		return nil, err
	}
	bk, err := query.Buildkit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get buildkit client: %w", err)
	}

	def, err := dir.LLB(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get file LLB: %w", err)
	}
	fs, err := reffs.OpenDef(ctx, bk, def)
	if err != nil {
		return nil, err
	}

	return fs.Open(dir.File)
}

func (dir *File) Export(ctx context.Context, dest string, allowParentDirPath bool) (rerr error) {
	query, err := CurrentQuery(ctx)
	if err != nil {
		return err
	}
	bk, err := query.Buildkit(ctx)
	if err != nil {
		return fmt.Errorf("failed to get buildkit client: %w", err)
	}

	src, err := dir.State(ctx)
	if err != nil {
		return err
	}
	def, err := src.Marshal(ctx, llb.Platform(dir.Platform.Spec()))
	if err != nil {
		return err
	}

	ctx, vtx := Tracer(ctx).Start(ctx, fmt.Sprintf("export file %s to host %s", dir.File, dest))
	defer telemetry.End(vtx, func() error { return rerr })

	return bk.LocalFileExport(ctx, def.ToPB(), dest, dir.File, allowParentDirPath)
}

func (dir *File) Mount(ctx context.Context, f func(string) error) error {
	def, err := dir.LLB(ctx)
	if err != nil {
		return fmt.Errorf("failed to get file LLB: %w", err)
	}
	return mountLLB(ctx, def, func(root string) error {
		src, err := containerdfs.RootPath(root, dir.File)
		if err != nil {
			return err
		}
		return f(src)
	})
}

// bkRef returns the buildkit reference from the solved def.
func bkRef(ctx context.Context, bk *buildkit.Client, def *pb.Definition) (bkgw.Reference, error) {
	res, err := bk.Solve(ctx, bkgw.SolveRequest{
		Definition: def,
	})
	if err != nil {
		return nil, err
	}

	ref, err := res.SingleRef()
	if err != nil {
		return nil, err
	}

	if ref == nil {
		// empty file, i.e. llb.Scratch()
		return nil, fmt.Errorf("empty reference")
	}

	return ref, nil
}
