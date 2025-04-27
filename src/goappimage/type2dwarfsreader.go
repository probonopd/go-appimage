package goappimage

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type type2DwarfsReader struct {
	mountPoint  string
	aiPath      string
	fileReadNum int
	unmount     *time.Timer
}

func newType2DwarfsReader(ai *AppImage) (*type2DwarfsReader, error) {
	pth, err := exec.LookPath("dwarfs")
	if pth == "" {
		return nil, errors.New("dwarfs not found in PATH, cannot read dwarfs AppImage")
	}
	pth, err = exec.LookPath("dwarfsextract")
	if pth == "" {
		return nil, errors.New("dwarfsextract not found in PATH, cannot read dwarfs AppImage")
	}
	tmpDir, err := os.MkdirTemp(os.TempDir(), "appimaged-")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("dwarfs", ai.Path, tmpDir, "-o", "offset=auto")
	err = cmd.Run()
	if err != nil {
		exec.Command("umount", tmpDir).Run()
		return nil, err
	}
	out := &type2DwarfsReader{
		mountPoint: tmpDir,
		aiPath:     ai.Path,
	}
	out.unmount = time.AfterFunc(5*time.Second, func() {
		exec.Command("umount", tmpDir).Run()
		os.RemoveAll(tmpDir)
		out.mountPoint = ""
	})
	return out, nil
}

func (r *type2DwarfsReader) mountOrPauseUnmount() error {
	if r.mountPoint != "" {
		r.unmount.Stop()
		return nil
	}
	tmpDir, err := os.MkdirTemp(os.TempDir(), "appimaged-")
	if err != nil {
		return err
	}
	cmd := exec.Command("dwarfs", r.aiPath, tmpDir, "-o", "offset=auto")
	err = cmd.Run()
	if err != nil {
		exec.Command("umount", tmpDir).Run()
		return err
	}
	return nil
}

func (r *type2DwarfsReader) resumeTimer() {
	if r.fileReadNum == 0 {
		r.unmount = time.AfterFunc(5*time.Second, func() {
			exec.Command("umount", r.mountPoint).Run()
			os.RemoveAll(r.mountPoint)
			r.mountPoint = ""
		})
	}
}

type type2DwarfsFile struct {
	fil *os.File
	rdr *type2DwarfsReader
}

func (f type2DwarfsFile) Read(b []byte) (int, error) {
	return f.fil.Read(b)
}

func (f type2DwarfsFile) Close() error {
	f.fil.Close()
	f.rdr.fileReadNum--
	if f.rdr.fileReadNum == 0 {
		f.rdr.resumeTimer()
	}
	return nil
}

func (r *type2DwarfsReader) FileReader(path string) (io.ReadCloser, error) {
	err := r.mountOrPauseUnmount()
	if err != nil {
		return nil, err
	}
	fil, err := os.Open(filepath.Join(r.mountPoint, path))
	if err != nil {
		r.resumeTimer()
		return nil, err
	}
	if stat, _ := fil.Stat(); stat.Mode()&os.ModeSymlink == os.ModeSymlink {
		var link string
		link, err = os.Readlink(filepath.Join(r.mountPoint, path))
		if err != nil {
			return nil, errors.New("Can't resolve symlink at: " + path)
		}
		fil, err = os.Open(filepath.Join(r.mountPoint, path, link))
		if err != nil {
			return nil, errors.New("Can't resolve symlink at: " + path)
		}
	}
	if stat, _ := fil.Stat(); !stat.Mode().IsRegular() {
		return nil, errors.New("Path is a directory: " + path)
	}
	r.fileReadNum++
	return type2DwarfsFile{
		fil: fil,
		rdr: r,
	}, nil
}

func (r *type2DwarfsReader) IsDir(path string) bool {
	err := r.mountOrPauseUnmount()
	if err != nil {
		return false
	}
	defer r.resumeTimer()
	stat, err := os.Stat(filepath.Join(r.mountPoint, path))
	if err != nil {
		return false
	}
	return stat.IsDir()
}

func (r *type2DwarfsReader) ListFiles(path string) []string {
	err := r.mountOrPauseUnmount()
	if err != nil {
		return nil
	}
	defer r.resumeTimer()
	fil, err := os.Open(filepath.Join(r.mountPoint, path))
	if err != nil {
		return nil
	}
	names, _ := fil.Readdirnames(-1)
	return names
}

func (r *type2DwarfsReader) ExtractTo(path, destination string, resolveSymlinks bool) error {
	if resolveSymlinks {
		err := r.mountOrPauseUnmount()
		if err != nil {
			return nil
		}
		defer r.resumeTimer()
		stat, err := os.Stat(filepath.Join(r.mountPoint, path))
		if err != nil {
			return err
		}
		if stat.Mode()&os.ModeSymlink == os.ModeSymlink {
			var symPath string
			symPath, err = os.Readlink(filepath.Join(r.mountPoint, path))
			if err != nil {
				return err
			}
			path = filepath.Join(path, symPath)
		}
	}
	cmd := exec.Command("dwarfsextract",
		"-i", r.aiPath,
		"--pattern=\""+path+"\"",
		"-O", "auto",
		"-o", destination)
	return cmd.Run()
}
