package fuse

import "os"

type foldFS struct {
	*os.File
}
