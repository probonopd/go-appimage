package goappimage

import (
	"io"
)

type archiveReader interface {
	//FileReader returns an io.ReadCloser for a file at the given path.
	//If the given path is a symlink, it will return the link's reader.
	//If the symlink is to an absolute path, an error is returned.
	//Returns an error if the given path is a directory.
	FileReader(path string) (io.ReadCloser, error)
	//IsDir returns if the given path points to a directory.
	IsDir(path string) bool
	//ListFiles returns a list of filenames at the given directory.
	//Returns nil if the given path is a symlink, file, or isn't contained.
	ListFiles(path string) []string
	//ExtractTo extracts the file/folder at path to the folder at destination.
	ExtractTo(path, destination string, resolveSymlinks bool) error
}
