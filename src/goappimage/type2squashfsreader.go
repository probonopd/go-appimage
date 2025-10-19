package goappimage

import (
	"errors"
	"io"
	"os"

	"github.com/CalebQ42/squashfs"
)

type type2SquashfsReader struct {
	rdr *squashfs.Reader
}

func newType2SquashfsReader(ai *AppImage) (*type2SquashfsReader, error) {
	aiFil, err := os.Open(ai.Path)
	if err != nil {
		return nil, err
	}
	squashRdr, err := squashfs.NewReaderAtOffset(aiFil, ai.offset)
	if err != nil {
		return nil, err
	}
	return &type2SquashfsReader{
		rdr: &squashRdr,
	}, nil
}

// type anonymousCloser struct {
// 	close func() error
// }

// func (a anonymousCloser) Close() error {
// 	return a.close()
// }

func (r *type2SquashfsReader) FileReader(filepath string) (io.ReadCloser, error) {
	//TODO: command fallback
	fsFil, err := r.rdr.Open(filepath)
	if err != nil {
		return nil, err
	}
	fil := fsFil.(*squashfs.File)
	for fil.IsSymlink() {
		symFil := fil.GetSymlinkFile()
		if symFil == nil {
			return nil, errors.New("Can't resolve symlink at: " + filepath)
		}
		fil = symFil.(*squashfs.File)
	}
	if fil.IsDir() {
		return nil, errors.New("Path is a directory: " + filepath)
	}
	return fil, nil
}

func (r *type2SquashfsReader) IsDir(filepath string) bool {
	fsFil, err := r.rdr.Open(filepath)
	if err != nil {
		return false
	}
	fil := fsFil.(*squashfs.File)
	for fil.IsSymlink() {
		symFil := fil.GetSymlinkFile()
		if symFil == nil {
			return false
		}
		fil = symFil.(*squashfs.File)
	}
	return fil.IsDir()
}

func (r *type2SquashfsReader) ListFiles(path string) []string {
	fsFil, err := r.rdr.Open(path)
	if err != nil {
		return nil
	}
	fil := fsFil.(*squashfs.File)
	for fil.IsSymlink() {
		symFil := fil.GetSymlinkFile()
		if symFil == nil {
			return nil
		}
		fil = symFil.(*squashfs.File)
	}
	if !fil.IsDir() {
		return nil
	}
	children, err := fil.ReadDir(0)
	if err != nil {
		return nil
	}
	out := make([]string, len(children))
	for _, child := range children {
		out = append(out, child.Name())
	}
	return out
}

func (r *type2SquashfsReader) ExtractTo(filepath, destination string, resolveSymlinks bool) error {
	fsFil, err := r.rdr.Open(filepath)
	if err != nil {
		return err
	}
	options := squashfs.DefaultOptions()
	options.DereferenceSymlink = resolveSymlinks
	err = fsFil.(*squashfs.File).ExtractWithOptions(destination, options)
	return err
}
