package goappimage

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
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
	//SymlinkPath returns where the symlink at path is pointing.
	//If the given path is not a symlink, just returns the given path.
	SymlinkPath(path string) string
	//SymlinkPath is similiar to SymlinkPath, but will recursively try
	//to get the symlink's path. If the location is outside the archive,
	//the initial path is returned.
	SymlinkPathRecursive(path string) string
	//Contains returns if the given path is contained in the archive.
	Contains(path string) bool
	//ListFiles returns a list of filenames at the given directory.
	//Returns nil if the given path is a symlink, file, or isn't contained.
	ListFiles(path string) []string
	//ExtractTo extracts the file/folder at path to the folder at destination.
	ExtractTo(path, destination string, resolveSymlinks bool) error
}

func (ai *AppImage) populateReader() (err error) {
	if ai.imageType == 1 {
		ai.reader, err = newType1Reader(ai.path)
	} else if ai.imageType == 2 {
		ai.reader, err = newType2Reader(ai)
		return err
	}
	return errors.New("HIII")
}

type type2Reader struct {
	rdr *squashfs.Reader
}

func newType2Reader(ai *AppImage) (*type2Reader, error) {
	aiFil, err := os.Open(ai.path)
	if err != nil {
		return nil, err
	}
	stat, _ := aiFil.Stat()
	aiRdr := io.NewSectionReader(aiFil, ai.offset, stat.Size()-ai.offset)
	squashRdr, err := squashfs.NewSquashfsReader(aiRdr)
	if err != nil {
		return nil, err
	}
	return &type2Reader{
		rdr: squashRdr,
	}, nil
}

func (r *type2Reader) FileReader(path string) (io.ReadCloser, error) {
	fil := r.rdr.GetFileAtPath(path)
	if fil == nil {
		return nil, errors.New("Can't find file at: " + path)
	}
	if fil.IsSymlink() {
		fil = fil.GetSymlinkFileRecursive()
		if fil == nil {
			return nil, errors.New("Can't resolve symlink at: " + path)
		}
	}
	if fil.IsDir() {
		return nil, errors.New("Path is a directory: " + path)
	}
	return fil, nil
}

func (r *type2Reader) IsDir(path string) bool {
	fil := r.rdr.GetFileAtPath(path)
	if fil == nil {
		return false
	}
	if fil.IsSymlink() {
		fil = fil.GetSymlinkFileRecursive()
		if fil == nil {
			return false
		}
	}
	return fil.IsDir()
}

func (r *type2Reader) SymlinkPath(path string) string {
	fil := r.rdr.GetFileAtPath(path)
	if fil == nil {
		return path
	}
	if fil.IsSymlink() {
		return fil.SymlinkPath()
	}
	return path
}

func (r *type2Reader) SymlinkPathRecursive(path string) string {
	fil := r.rdr.GetFileAtPath(path)
	if fil == nil {
		return path
	}
	if fil.IsSymlink() {
		tmpLoc := r.SymlinkPathRecursive(fil.Path())
		if tmpLoc != fil.Path() {
			return tmpLoc
		}
	}
	return path
}

func (r *type2Reader) Contains(path string) bool {
	fil := r.rdr.GetFileAtPath(path)
	return fil != nil
}

func (r *type2Reader) ListFiles(path string) []string {
	fil := r.rdr.GetFileAtPath(path)
	if fil == nil {
		return nil
	}
	if fil.IsSymlink() {
		fil = fil.GetSymlinkFileRecursive()
		if fil == nil {
			return nil
		}
	}
	if !fil.IsDir() {
		return nil
	}
	children, err := fil.GetChildren()
	if err != nil {
		return nil
	}
	out := make([]string, 0)
	for _, child := range children {
		out = append(out, child.Name())
	}
	return out
}

func (r *type2Reader) ExtractTo(path, destination string, resolveSymlinks bool) error {
	fil := r.rdr.GetFileAtPath(path)
	if fil == nil {
		return nil
	}
	if fil.IsSymlink() && resolveSymlinks {
		tmp := fil.GetSymlinkFileRecursive()
		if tmp != nil {
			errs := tmp.ExtractTo(destination)
			if len(errs) > 0 {
				return errs[0]
			}
			return nil
		}
	}
	errs := fil.ExtractTo(destination)
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
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
	containedFiles := strings.Split(string(wrt.Bytes()), "\n")
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
	return nil, nil
}

func (r *type1Reader) FileReader(filepath string) (io.ReadCloser, error) {
	//I need to make sure that, if there is wildcards, that we only get ONE file. If we get more then one, ALL matching files will be in Stdout.
	//Probably a bit spagetti code...
	filepath = strings.TrimPrefix(filepath, "/")
	filepath = path.Clean(filepath)
	var filepathDir string
	if strings.Contains(filepath, "/") {
		for _, dir := range r.folders {
			match, _ := path.Match(dir, path.Dir(filepath))
			if match {
				filepathDir = dir
				break
			}
		}
	} else {
		filepathDir = "/"
	}
	if filepathDir == "" {
		return nil, errors.New("File not found in the archive")
	}
	var filepathName string
	for _, fil := range r.structure[filepathDir] {
		match, _ := path.Match(fil, path.Base(filepath))
		if match {
			filepathName = fil
			break
		}
	}
	if filepathName == "" {
		return nil, errors.New("File not found in the archive")
	}
	if filepathDir == "/" {
		filepath = filepathName
	} else {
		filepath = filepathDir + "/" + filepathName
	}
	//We're finally sure we have just ONE file, lol
	cmd := exec.Command("bsdtar", "-f", r.path, "-xO", filepath)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	return ioutil.NopCloser(&out), nil
}

func (r *type1Reader) IsDir(filepath string) bool {
	filepath = strings.TrimPrefix(filepath, "/")
	filepath = path.Clean(filepath)
	if filepath == "" {
		return true //this means they're asking if root is a dir.....
	}
	return r.structure[filepath] != nil
}

func (r *type1Reader) SymlinkPath(filepath string) string {
	cmd := exec.Command("bsdtar", "-f", r.path, "-tv", filepath)
	wrt, err := runCommand(cmd)
	if err != nil {
		return filepath
	}
	output := strings.TrimSuffix(string(wrt.Bytes()), "\n")
	output = strings.Split(output, "\n")[0]                //Make sure we are only getting the first value that matches
	if index := strings.Index(output, "->"); index != -1 { //signifies symlink
		return output[index+3:]
	}
	return filepath
}

func (r *type1Reader) SymlinkPathRecursive(filepath string) string {
	cmd := exec.Command("bsdtar", "-f", r.path, "-tv", filepath)
	wrt, err := runCommand(cmd)
	if err != nil {
		return filepath
	}
	output := strings.TrimSuffix(string(wrt.Bytes()), "\n")
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
	filepath = strings.TrimPrefix(filepath, "/")
	filepath = path.Clean(filepath)
	dir := path.Dir(filepath)
	name := path.Base(filepath)
	if dir == "" {
		dir = "/"
	}
	return sort.SearchStrings(r.structure[dir], name) != len(r.structure[dir])
}
func (r *type1Reader) ListFiles(filepath string) []string {
	filepath = strings.TrimPrefix(filepath, "/")
	filepath = path.Clean(filepath)
	if filepath == "" {
		return r.structure["/"]
	}
	return r.structure[filepath]
}

func (r *type1Reader) ExtractTo(filepath, destination string, resolveSymlinks bool) error {
	err := os.MkdirAll(destination, os.ModePerm)
	if err != nil {
		return err
	}
	name := path.Base(filepath)
	filepath = strings.TrimPrefix(filepath, "/")
	destination = strings.TrimSuffix(destination, "/")
	tmpDir := destination + "/" + ".temp"
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
