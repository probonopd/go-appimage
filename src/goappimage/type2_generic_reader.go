package goappimage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Reader for a non-squashfs type2 AppImage.
// Requires the AppImage to implement --appimage-extract and --appimage-mount
type type2GenericReader struct {
	path string
}

type type2GenericFile struct {
	tmpPath string
	file    *os.File
}

func newType2GenericReader(aiPath string) type2GenericReader {
	return type2GenericReader{aiPath}
}

func (r type2GenericFile) Read(b []byte) (int, error) {
	return r.file.Read(b)
}

func (r type2GenericFile) Close() error {
	r.file.Close()
	os.RemoveAll(r.tmpPath)
	return nil
}

func (r type2GenericReader) FileReader(path string) (io.ReadCloser, error) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "appimaged-")
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(r.path, "--appimage-extract", path)
	cmd.Dir = tmpDir
	err = cmd.Run()
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}
	fil, err := os.Open(filepath.Join(tmpDir, "squashfs-root", path))
	if err != nil {
		fmt.Println("error openning:", err)
		os.RemoveAll(tmpDir)
		return nil, err
	}
	stat, err := fil.Stat()
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}
	if !stat.Mode().IsRegular() {
		os.RemoveAll(tmpDir)
		return nil, errors.New("file not regular")
	}
	return type2GenericFile{tmpPath: tmpDir, file: fil}, nil
}

func (r type2GenericReader) IsDir(path string) bool {
	cmd := exec.Command(r.path, "--appimage-mount")
	cmdOut := new(bytes.Buffer)
	cmd.Stdout = cmdOut
	err := cmd.Start()
	if err != nil {
		return false
	}
	defer cmd.Process.Signal(os.Interrupt)
	// Wait a moment to make sure AppImage is mounted
	time.Sleep(50 * time.Millisecond)
	// Make sure the mount dir is printed to stdout before we proceed
	// Wait a total of 1 second max for this.
	wait := 1
	for i := 1; i < 20 && cmdOut.String() == ""; i++ {
		wait++
		if wait == 20 { // 1 second
			return false
		}
	}
	stat, err := os.Stat(filepath.Join(strings.TrimSpace(cmdOut.String()), path))
	if err != nil {
		return false
	}
	return stat.IsDir()
}

func (r type2GenericReader) ListFiles(path string) []string {
	cmd := exec.Command(r.path, "--appimage-mount")
	cmdOut := new(bytes.Buffer)
	cmd.Stdout = cmdOut
	err := cmd.Start()
	if err != nil {
		fmt.Println(r.path, err)
		return nil
	}
	defer func() {
		cmd.Process.Signal(os.Interrupt)
	}()
	// Wait a moment to make sure AppImage is mounted
	time.Sleep(50 * time.Millisecond)
	// Make sure the mount dir is printed to stdout before we proceed
	// Wait a total of 1 second max for this.
	wait := 1
	for i := 1; i < 20 && cmdOut.String() == ""; i++ {
		wait++
		if wait == 20 { // 1 second
			return nil
		}
	}
	fil, err := os.Open(filepath.Join(strings.TrimSpace(cmdOut.String()), path))
	if err != nil {
		return nil
	}
	names, err := fil.Readdirnames(-1)
	if err != nil {
		return nil
	}
	return names
}

func (r type2GenericReader) ExtractTo(path, destination string, resolveSymlinks bool) error {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "appimaged-")
	if err != nil {
		return err
	}
	cmd := exec.Command(r.path, "--appimage-extract", path)
	cmd.Dir = tmpDir
	err = cmd.Run()
	if err != nil {
		os.RemoveAll(tmpDir)
		return err
	}
	//TODO: do not ignore resolveSymlinks
	return os.Rename(filepath.Join(tmpDir, "squashfs-root", path), filepath.Join(destination, filepath.Base(path)))
}
