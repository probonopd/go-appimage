package goappimage

import "io"

//TODO: easy way to interact with both type 1 and type 2 appimages.
type archiveReader interface {
	GetFileAtPath(path string, resolveSymlink bool) (io.ReadCloser, error)
}
