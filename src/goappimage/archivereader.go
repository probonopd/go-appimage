package goappimage

import (
	"errors"
	"io"
	"os"
	"path"
	"strings"

	"github.com/CalebQ42/squashfs"
	"github.com/kdomanski/iso9660"
)

//TODO: easy way to interact with both type 1 and type 2 appimages.
type archiveReader interface {
	//rdr is the retuned reader, symlink says if the file at path is a symlink. If this is true, rdr will be nil.
	GetFileAtPath(path string, resolveSymlink bool) (rdr io.ReadCloser, symlink bool, err error)
}

type type2Reader struct {
	reader *squashfs.Reader
}

func newType2Reader(ai *AppImage) (*type2Reader, error) {
	fil, err := os.Open(ai.path)
	if err != nil {
		return nil, err
	}
	stat, _ := fil.Stat()
	rdr, err := squashfs.NewSquashfsReader(io.NewSectionReader(fil, ai.offset, stat.Size()-ai.offset))
	return &type2Reader{
		reader: rdr,
	}, nil
}

func (r *type2Reader) GetFileAtPath(filepath string, resolveSymlink bool) (io.ReadCloser, bool, error) {
	fil := r.reader.GetFileAtPath(filepath)
	if fil == nil {
		return nil, false, errors.New("Not able to find file " + filepath)
	}
	if resolveSymlink {
		sym := fil.GetSymlinkFile()
		if sym == nil {

		}
	}
	return nil, true, nil
}

type type1Reader struct {
	image *iso9660.Image
}

func newType1Reader(ai *AppImage) (*type1Reader, error) {
	fil, err := os.Open(ai.path)
	if err != nil {
		return nil, err
	}
	img, err := iso9660.OpenImage(fil)
	if err != nil {
		return nil, err
	}
	return &type1Reader{
		image: img,
	}, nil
}

func (r *type1Reader) GetFileAtPath(filepath string, resolveSymlink bool) (io.ReadCloser, bool, error) {
	filepath = strings.Trim(path.Clean(filepath), "/")
	dir, err := r.image.RootDir()
	if err != nil {
		return nil, false, err
	}
	children, err := dir.GetChildren()
	if err != nil {
		return nil, false, err
	}
	split := strings.Split(filepath, "/")
	index := 0
	for _, child := range children {
		name := path.Base(child.Name())
		if name == 
	}
}

func 
