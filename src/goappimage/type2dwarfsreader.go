package goappimage

import (
	"io"
	"os"
	"os/exec"
	"time"
)

type type2DwarfsReader struct {
	mountPoint string
	unmount    *time.Timer
}

func newType2DwarfsReader(ai AppImage) (*type2DwarfsReader, error) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "appimaged-")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("dwarfs", ai.Path, tmpDir, "-o offset=auto")
	err = cmd.Run()
	if err != nil {
		exec.Command("umount", tmpDir)
		return nil, err
	}
	out := &type2DwarfsReader{
		mountPoint: tmpDir,
	}
	out.unmount = time.AfterFunc(5*time.Second, func() {
		exec.Command("umount", tmpDir)
		out.mountPoint = ""
	})
	return out, nil
}

func (r *type2DwarfsReader) mountOrPauseUnmount() {
	if r.mountPoint != "" {
		r.unmount.Stop()
		return
	}
}

func (r *type2DwarfsReader) resumeTimer() {
	r.unmount = time.AfterFunc(5*time.Second, func() {
		exec.Command("umount", r.mountPoint)
		r.mountPoint = ""
	})
}

func (r *type2DwarfsReader) FileReader(path string) (io.ReadCloser, error)

func (r *type2DwarfsReader) IsDir(path string) bool

func (r *type2DwarfsReader) ListFiles(path string) []string

func (r *type2DwarfsReader) ExtractTo(path, destination string, resolveSymlinks bool) error
