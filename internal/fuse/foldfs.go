package fuse

import (
	"context"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"

	"github.com/CalebQ42/fuse"
	"github.com/CalebQ42/fuse/fs"
)

type fileRoot struct {
	*os.File
}

func (f fileRoot) Root() (fs.Node, error) {
	return &fileNode{
		File:  f.File,
		inode: 1,
	}, nil
}

type fileNode struct {
	*os.File

	inode uint64
}

func (f *fileNode) ReadAll(ctx context.Context) ([]byte, error) {
	return io.ReadAll(f.File)
}

func (f *fileNode) ReadDirAll(ctx context.Context) (out []fuse.Dirent, err error) {
	ents, err := f.ReadDir(-1)
	if err != nil {
		return nil, fuse.ENOTDIR
	}
	var typ iofs.FileMode
	var t fuse.DirentType
	for i := range ents {
		typ = ents[i].Type()
		switch {
		case typ|iofs.ModeDir == iofs.ModeDir:
			t = fuse.DT_Dir
		case typ|iofs.ModeSymlink == iofs.ModeSymlink:
			t = fuse.DT_Link
		case typ.IsRegular():
			t = fuse.DT_File
		default:
			t = fuse.DT_Unknown
		}
		out = append(out, fuse.Dirent{
			Type:  t,
			Inode: fs.GenerateDynamicInode(f.inode, ents[i].Name()),
			Name:  ents[i].Name(),
		})
	}
	return
}

func (f *fileNode) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	resp.Data = make([]byte, req.Size)
	n, err := f.ReadAt(resp.Data, req.Offset)
	if err == io.EOF {
		resp.Data = resp.Data[:n]
	} else if err != nil {
		return fuse.ENODATA
	}
	return nil
}

func (f *fileNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) (err error) {
	resp.Size, err = f.WriteAt(req.Data, req.Offset)
	return
}

func (f *fileNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	stat, _ := f.Stat()
	attr.Inode = f.inode
	attr.Mode = stat.Mode()
	attr.Size = uint64(stat.Size())
	attr.Mtime = stat.ModTime()
	return nil
}

func (f *fileNode) Id() uint64 {
	return f.inode
}

func (f *fileNode) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
	return os.Readlink(f.Name())
}

func (f *fileNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	return os.Remove(filepath.Join(f.Name(), req.Name))
}
