package goappimage

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"

	"github.com/CalebQ42/squashfs"
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

func (ai *AppImage) populateReader(allowFallback, forceFallback bool) (err error) {
	if ai.imageType == 1 {
		ai.reader, err = newType1Reader(ai.Path)
		return err
	} else if ai.imageType == 2 {
		ai.reader, err = newType2Reader(ai)
		return err
	}
	return errors.New("invalid AppImage type")
}

// TODO: Implement command based fallback here.
type type2Reader struct {
	rdr *squashfs.Reader
}

func newType2Reader(ai *AppImage) (*type2Reader, error) {
	aiFil, err := os.Open(ai.Path)
	if err != nil {
		return nil, err
	}
	stat, _ := aiFil.Stat()
	aiRdr := io.NewSectionReader(aiFil, ai.offset, stat.Size()-ai.offset)
	squashRdr, err := squashfs.NewReader(aiRdr)
	if err != nil {
		return nil, err
	}
	return &type2Reader{
		rdr: squashRdr,
	}, nil
}

// type anonymousCloser struct {
// 	close func() error
// }

// func (a anonymousCloser) Close() error {
// 	return a.close()
// }

func (r *type2Reader) FileReader(filepath string) (io.ReadCloser, error) {
	//TODO: command fallback
	fsFil, err := r.rdr.Open(filepath)
	if err != nil {
		return nil, err
	}
	fil := fsFil.(*squashfs.File)
	for fil.IsSymlink() {
		fil = fil.GetSymlinkFile()
		if fil == nil {
			return nil, errors.New("Can't resolve symlink at: " + filepath)
		}
	}
	if fil.IsDir() {
		return nil, errors.New("Path is a directory: " + filepath)
	}
	return fil, nil
}

func (r *type2Reader) IsDir(filepath string) bool {
	fsFil, err := r.rdr.Open(filepath)
	if err != nil {
		return false
	}
	fil := fsFil.(*squashfs.File)
	if fil.IsSymlink() {
		fil = fil.GetSymlinkFile()
		if fil == nil {
			return false
		}
	}
	return fil.IsDir()
}

func (r *type2Reader) ListFiles(path string) []string {
	fsFil, err := r.rdr.Open(path)
	if err != nil {
		return nil
	}
	fil := fsFil.(*squashfs.File)
	if fil.IsSymlink() {
		fil = fil.GetSymlinkFile()
		if fil == nil {
			return nil
		}
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

func (r *type2Reader) ExtractTo(filepath, destination string, resolveSymlinks bool) error {
	fsFil, err := r.rdr.Open(filepath)
	if err != nil {
		return err
	}
	options := squashfs.DefaultOptions()
	options.DereferenceSymlink = true
	err = fsFil.(*squashfs.File).ExtractWithOptions(destination, options)
	return err
}

type type1Reader struct {
	structure map[string][]string //[folder]File
	path      string
	folders   []string
}

func newType1Reader(filepath string) (*type1Reader, error) {
	cmd := exec.Command("bsdtar", "-f", filepath, "-t")
	wrt, err := runCommand(cmd)
	if err != nil {
		return nil, err
	}
	containedFiles := strings.Split(wrt.String(), "\n")
	var rdr type1Reader
	rdr.path = filepath
	rdr.structure = make(map[string][]string)
	for _, contained := range containedFiles {
		contained = path.Clean(contained)
		if contained == "" || contained == "." {
			continue
		}
		fileName := path.Base(contained)
		dir := path.Dir(contained)
		dir = path.Clean(dir)
		if !strings.Contains(contained, "/") {
			dir = "/"
		}
		if rdr.structure[dir] == nil && dir != "/" {
			rdr.folders = append(rdr.folders, dir)
		}
		rdr.structure[dir] = append(rdr.structure[dir], fileName)
	}
	sort.Strings(rdr.folders)
	for folds := range rdr.structure {
		sort.Strings(rdr.structure[folds])
	}
	return &rdr, nil
}

// makes sure that the path is nice and only points to ONE file, which is needed if there are wildcards.
// If you were to search for *.desktop, you will get both blender.desktop AND /usr/bin/blender.desktop.
// This could cause issues, especially for FileReader
//
// Probably a bit spagetti and can be cleaned up. Maybe add a rawPaths variable to type1reader to make
// it easier to find a match with wildcards.
func (r *type1Reader) cleanPath(filepath string) (string, error) {
	filepath = strings.TrimPrefix(filepath, "/")
	filepath = path.Clean(filepath)
	if filepath == "" {
		return "", nil
	}
	filepathDir := path.Dir(filepath)
	if filepathDir != "." {
		for _, dir := range r.folders {
			match, _ := path.Match(filepathDir, dir)
			if match {
				filepathDir = dir
				break
			}
		}
	} else {
		filepathDir = "/"
	}
	if filepathDir == "" {
		return "", errors.New("file not found in the archive")
	}
	filepathName := path.Base(filepath)
	for _, fil := range r.structure[filepathDir] {
		match, _ := path.Match(filepathName, fil)
		if match {
			filepathName = fil
			break
		}
	}
	if filepathName == "" {
		return "", errors.New("file not found in the archive")
	}
	if filepathDir == "/" {
		filepath = filepathName
	} else {
		filepath = filepathDir + "/" + filepathName
	}
	return filepath, nil
}

func (r *type1Reader) FileReader(filepath string) (io.ReadCloser, error) {
	//TODO: check size of file and if it's large, extract to a temp directory, read that, and delete it on close.
	//This would make sure a huge file isn't completely held in memory via the byte buffer.
	filepath, err := r.cleanPath(filepath)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("bsdtar", "-f", r.path, "-xO", filepath)
	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return nil, err
	}
	return io.NopCloser(&out), nil
}

func (r *type1Reader) IsDir(filepath string) bool {
	filepath, err := r.cleanPath(filepath)
	if err != nil {
		return false
	}
	return r.structure[filepath] != nil
}

func (r *type1Reader) SymlinkPath(filepath string) string {
	filepath, err := r.cleanPath(filepath)
	if err != nil {
		return filepath
	}
	cmd := exec.Command("bsdtar", "-f", r.path, "-tv", filepath)
	wrt, err := runCommand(cmd)
	if err != nil {
		return filepath
	}
	output := strings.TrimSuffix(wrt.String(), "\n")
	output = strings.Split(output, "\n")[0]                //Make sure we are only getting the first value that matches
	if index := strings.Index(output, "->"); index != -1 { //signifies symlink
		return output[index+3:]
	}
	return filepath
}

func (r *type1Reader) SymlinkPathRecursive(filepath string) string {
	filepath, err := r.cleanPath(filepath)
	if err != nil {
		return filepath
	}
	cmd := exec.Command("bsdtar", "-f", r.path, "-tv", filepath)
	wrt, err := runCommand(cmd)
	if err != nil {
		return filepath
	}
	output := strings.TrimSuffix(wrt.String(), "\n")
	if index := strings.Index(output, "->"); index != -1 { //signifies symlink
		symlinkedFile := output[index+3:]
		if strings.HasPrefix(symlinkedFile, "/") {
			return filepath //we can't help with absolute symlinks...
		}
		tmp := r.SymlinkPathRecursive(path.Dir(filepath) + "/" + symlinkedFile)
		if tmp != path.Dir(filepath)+"/"+symlinkedFile {
			return tmp
		}
	}
	return filepath
}

func (r *type1Reader) Contains(filepath string) bool {
	filepath, err := r.cleanPath(filepath)
	if err != nil {
		return false
	}
	dir := path.Dir(filepath)
	name := path.Base(filepath)
	if dir == "" {
		dir = "/"
	}
	return sort.SearchStrings(r.structure[dir], name) != len(r.structure[dir])
}
func (r *type1Reader) ListFiles(filepath string) []string {
	filepath, err := r.cleanPath(filepath)
	if err != nil {
		return nil
	}
	if filepath == "" {
		return r.structure["/"]
	}
	if r.IsDir(filepath) {
		return r.structure[filepath]
	}
	return nil
}

func (r *type1Reader) ExtractTo(filepath, destination string, resolveSymlinks bool) error {
	filepath, err := r.cleanPath(filepath)
	if err != nil {
		return err
	}
	err = os.MkdirAll(destination, os.ModePerm)
	if err != nil {
		return err
	}
	name := path.Base(filepath)
	destination = strings.TrimSuffix(destination, "/")
	tmpDir := destination + "/" + ".tmp"
	err = os.Mkdir(tmpDir, os.ModePerm)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	if resolveSymlinks {
		filepath = r.SymlinkPathRecursive(filepath)
	}
	cmd := exec.Command("bsdtar", "-C", tmpDir, "-f", r.path, "-x", filepath)
	_, err = runCommand(cmd)
	if err != nil {
		return err
	}
	err = os.Rename(tmpDir+"/"+filepath, destination+"/"+name)
	if err != nil {
		return err
	}
	return err
}
