package fusemount

import (
	"context"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"

	"github.com/CalebQ42/fuse"
	"github.com/CalebQ42/fuse/fs"
)

func newFileNode(filename string) *fileNode {
	return &fileNode{name: filename}
}

type fileNode struct {
	// This doesn't work for some reason. The file pointer seems to die after it's mounted.
	// fil *os.File
	name string
}

func (f *fileNode) Root() (fs.Node, error) {
	return f, nil
}

func (f *fileNode) ReadAll(ctx context.Context) ([]byte, error) {
	return os.ReadFile(f.name)
}

func (f *fileNode) ReadDirAll(ctx context.Context) (out []fuse.Dirent, err error) {
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

func (f *fileNode) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
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

func (f *fileNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	stat, _ := os.Stat(f.name)
	attr.Mode = stat.Mode()
	attr.Size = uint64(stat.Size())
	attr.Mtime = stat.ModTime()
	return nil
}

func (f *fileNode) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
	return os.Readlink(f.name)
}

func (f *fileNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	return os.Remove(filepath.Join(f.name, req.Name))
}

func (f *fileNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	ents, err := os.ReadDir(f.name)
	if err != nil {
		return nil, fuse.ENOENT
	}
	for i := range ents {
		if ents[i].Name() == name {
			return newFileNode(filepath.Join(f.name, ents[i].Name())), nil
		}
	}
	return nil, fuse.ENOENT
}
