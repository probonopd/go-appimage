package fusemount

import (
	"context"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"

	"github.com/seaweedfs/fuse"
	"github.com/seaweedfs/fuse/fs"
)

func newFuse2FileNode(filename string) *fuse2FileNode {
	return &fuse2FileNode{name: filename}
}

type fuse2FileNode struct {
	// This doesn't work for some reason. The file pointer seems to die after it's mounted.
	// fil *os.File
	name string
}

func (f *fuse2FileNode) Root() (fs.Node, error) {
	return f, nil
}

func (f *fuse2FileNode) ReadAll(ctx context.Context) ([]byte, error) {
	return os.ReadFile(f.name)
}

func (f *fuse2FileNode) ReadDirAll(ctx context.Context) (out []fuse.Dirent, err error) {
	fil, _ := os.Open(f.name)
	ents, err := fil.Readdirnames(-1)
	if err != nil {
		return nil, fuse.ENOTDIR
	}
	var typ iofs.FileMode
	var t fuse.DirentType
	var stat iofs.FileInfo
	for i := range ents {
		stat, err = os.Stat(filepath.Join(f.name, ents[i]))
		if err != nil {
			continue
		}
		typ = stat.Mode().Type()
		switch {
		case typ&iofs.ModeDir == iofs.ModeDir:
			t = fuse.DT_Dir
		case typ&iofs.ModeSymlink == iofs.ModeSymlink:
			t = fuse.DT_Link
		case typ.IsRegular():
			t = fuse.DT_File
		default:
			t = fuse.DT_Unknown
		}
		out = append(out, fuse.Dirent{
			Type: t,
			Name: ents[i],
		})
	}
	return
}

func (f *fuse2FileNode) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	fil, err := os.Open(f.name)
	if err != nil {
		return fuse.ENODATA
	}
	resp.Data = make([]byte, req.Size)
	n, err := fil.ReadAt(resp.Data, req.Offset)
	if err == io.EOF {
		resp.Data = resp.Data[:n]
	} else if err != nil {
		return fuse.ENODATA
	}
	return nil
}

func (f *fuse2FileNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	stat, _ := os.Stat(f.name)
	attr.Mode = stat.Mode()
	attr.Size = uint64(stat.Size())
	attr.Mtime = stat.ModTime()
	return nil
}

func (f *fuse2FileNode) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
	return os.Readlink(f.name)
}

func (f *fuse2FileNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	return os.Remove(filepath.Join(f.name, req.Name))
}

func (f *fuse2FileNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	ents, err := os.ReadDir(f.name)
	if err != nil {
		return nil, fuse.ENOENT
	}
	for i := range ents {
		if ents[i].Name() == name {
			return newFuse2FileNode(filepath.Join(f.name, ents[i].Name())), nil
		}
	}
	return nil, fuse.ENOENT
}
