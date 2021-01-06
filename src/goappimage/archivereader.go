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
	"strconv"
	"strings"

	"github.com/CalebQ42/squashfs"
	ioutilextra "gopkg.in/src-d/go-git.v4/utils/ioutil"
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

func (ai *AppImage) populateReader(allowFallback, forceFallback bool) (err error) {
	if ai.imageType == 1 {
		ai.reader, err = newType1Reader(ai.Path)
		return err
	} else if ai.imageType == 2 {
		ai.reader, err = newType2Reader(ai, allowFallback, forceFallback)
		return err
	}
	return errors.New("Invalid AppImage type")
}

//TODO: Implement command based fallback here.
type type2Reader struct {
	rdr             *squashfs.Reader
	structure       map[string][]string
	path            string
	folders         []string
	offset          int
	fallbackAllowed bool
	forceFallback   bool
}

func newType2Reader(ai *AppImage, fallbackAllowed, forceFallback bool) (*type2Reader, error) {
	if forceFallback || ai == nil {
		out := &type2Reader{
			forceFallback: true,
		}
		err := out.setupCommandFallback(ai)
		if err != nil {
			return nil, err
		}
		return out, nil
	}
	aiFil, err := os.Open(ai.Path)
	if err != nil {
		return nil, err
	}
	stat, _ := aiFil.Stat()
	aiRdr := io.NewSectionReader(aiFil, ai.offset, stat.Size()-ai.offset)
	squashRdr, err := squashfs.NewSquashfsReader(aiRdr)
	if err != nil {
		if fallbackAllowed {
			//If there are errors, we force the use of unsquashfs.
			out := &type2Reader{
				forceFallback: true,
			}
			err := out.setupCommandFallback(ai)
			if err != nil {
				return nil, err
			}
			return out, nil
		}
		return nil, err
	}
	return &type2Reader{
		rdr:             squashRdr,
		fallbackAllowed: fallbackAllowed,
	}, nil
}

func (r *type2Reader) setupCommandFallback(ai *AppImage) error {
	r.structure = make(map[string][]string)
	r.folders = make([]string, 0)
	r.offset = int(ai.offset)
	r.path = ai.Path
	cmd := exec.Command("unsquashfs", "-o", strconv.FormatInt(ai.offset, 10), "-l", ai.Path)
	out, err := runCommand(cmd)
	if err != nil {
		return err
	}
	allFiles := strings.Split(string(out.Bytes()), "\n")
	for _, filepath := range allFiles {
		if filepath == "" {
			continue
		}
		filepath = path.Clean(strings.TrimPrefix(filepath, "squashfs-root/"))
		if filepath == "." {
			continue
		}
		dir := path.Dir(filepath)
		name := path.Base(filepath)
		if dir == "." {
			dir = "/"
		}
		if r.structure[dir] == nil {
			if dir != "/" {
				r.folders = append(r.folders, dir)
			}
			r.structure[dir] = make([]string, 0)
		}
		r.structure[dir] = append(r.structure[dir], name)
	}
	sort.Strings(r.folders)
	for dir := range r.structure {
		sort.Strings(r.structure[dir])
	}
	return nil
}

//makes sure that the path is nice and only points to ONE file, which is needed if there are wildcards.
//If you were to search for *.desktop, you will get both blender.desktop AND /usr/bin/blender.desktop.
//This could cause issues, especially for FileReader
//
//Probably a bit spagetti and can be cleaned up. Maybe add a rawPaths variable to type1reader to make
//it easier to find a match with wildcards.
func (r *type2Reader) cleanPath(filepath string) (string, error) {
	filepath = strings.TrimPrefix(filepath, "/")
	filepath = path.Clean(filepath)
	if filepath == "." {
		return "/", nil
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
		return "", errors.New("File not found in the archive")
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
		return "", errors.New("File not found in the archive")
	}
	if filepathDir == "/" {
		filepath = filepathName
	} else {
		filepath = filepathDir + "/" + filepathName
	}
	return filepath, nil
}

type anonymousCloser struct {
	close func() error
}

func (a anonymousCloser) Close() error {
	return a.close()
}

func (r *type2Reader) FileReader(filepath string) (io.ReadCloser, error) {
	//TODO: command fallback
	if !r.forceFallback {
		fil := r.rdr.GetFileAtPath(filepath)
		if fil == nil {
			return nil, errors.New("Can't find file at: " + filepath)
		}
		if fil.IsSymlink() {
			fil = fil.GetSymlinkFileRecursive()
			if fil == nil {
				return nil, errors.New("Can't resolve symlink at: " + filepath)
			}
		}
		if fil.IsDir() {
			return nil, errors.New("Path is a directory: " + filepath)
		}
		return ioutil.NopCloser(fil), nil
	}
	filepath, err := r.cleanPath(filepath)
	filepath = r.SymlinkPathRecursive(filepath)
	if filepath != r.SymlinkPath(filepath) {
		return nil, errors.New("Can't resolve symlink at: " + filepath)
	}
	if r.IsDir(filepath) {
		return nil, errors.New("Path is a directory: " + filepath)
	}
	tmpDir, err := ioutil.TempDir("", filepath)
	if err != nil {
		return nil, errors.New("Cannot make the temp directory")
	}
	err = r.ExtractTo(filepath, tmpDir, true)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}
	tmpFil, err := os.Open(tmpDir + "/" + path.Base(filepath))
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}
	closer := anonymousCloser{
		close: func() error {
			tmpFil.Close()
			return os.RemoveAll(tmpDir)
		},
	}
	return ioutilextra.NewReadCloser(tmpFil, closer), nil
}

func (r *type2Reader) IsDir(filepath string) bool {
	if !r.forceFallback {
		fil := r.rdr.GetFileAtPath(filepath)
		if fil == nil {
			//TODO: make squashfs differenciate between a not found file, and compression extraction issues.
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
	// commandFallback: TODO: when i can differenciate the above errors
	filepath, err := r.cleanPath(filepath)
	if err != nil {
		return false
	}
	return r.structure[filepath] != nil
}

func (r *type2Reader) SymlinkPath(filepath string) string {
	if !r.forceFallback {
		fil := r.rdr.GetFileAtPath(filepath)
		if fil == nil {
			return filepath
		}
		if fil.IsSymlink() {
			return fil.SymlinkPath()
		}
		return filepath
	}
	//fallingback to commands.
	//TODO: add a way fro the above to fallback down here.
	filepath, err := r.cleanPath(filepath)
	if err != nil {
		return filepath
	}
	cmd := exec.Command("unsquashfs", "-ll", "-o", strconv.Itoa(r.offset), r.path, filepath)
	out, err := runCommand(cmd)
	if err != nil {
		return filepath
	}
	tmpOutput := strings.Split(strings.TrimSuffix(string(out.Bytes()), "\n"), "\n")
	neededLine := tmpOutput[len(tmpOutput)-1]
	if strings.Contains(neededLine, "->") {
		neededLine = neededLine[strings.Index(neededLine, "->")+3:]
		return path.Dir(filepath) + "/" + neededLine
	}
	return filepath
}

func (r *type2Reader) SymlinkPathRecursive(filepath string) string {
	if !r.forceFallback {
		fil := r.rdr.GetFileAtPath(filepath)
		if fil == nil {
			return filepath
		}
		tmp := fil.GetSymlinkFileRecursive()
		if tmp == nil {
			return filepath
		}
		return tmp.Path()
	}
	//Command fallback
	//TODO: allow command fallback from above
	filepath, err := r.cleanPath(filepath)
	if err != nil {
		return filepath
	}
	symlinkedFile := r.SymlinkPath(filepath)
	if symlinkedFile == filepath {
		return filepath
	}
	if strings.HasPrefix(symlinkedFile, "/") {
		return filepath //we can't help with absolute symlinks...
	}
	tmp := r.SymlinkPathRecursive(path.Dir(filepath) + "/" + symlinkedFile)
	if tmp != path.Dir(filepath)+"/"+symlinkedFile {
		return tmp
	}
	return filepath
}

func (r *type2Reader) Contains(path string) bool {
	if !r.forceFallback {
		fil := r.rdr.GetFileAtPath(path)
		return fil != nil
	}
	path, err := r.cleanPath(path)
	return err == nil
}

func (r *type2Reader) ListFiles(path string) []string {
	if !r.forceFallback {
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
	path, err := r.cleanPath(path)
	if err != nil {
		return nil
	}
	return r.structure[path]
}

func (r *type2Reader) ExtractTo(filepath, destination string, resolveSymlinks bool) error {
	if !r.forceFallback {
		fil := r.rdr.GetFileAtPath(filepath)
		if fil == nil {
			return nil
		}
		var errs []error
		if resolveSymlinks {
			errs = fil.ExtractSymlink(filepath)
		} else {
			errs = fil.ExtractTo(destination)
		}
		if len(errs) > 0 {
			return errs[0]
		}
		return nil
	}
	filepath, err := r.cleanPath(filepath)
	if err != nil {
		return err
	}
	var origName string
	if resolveSymlinks {
		origName = path.Base(filepath)
		filepath = r.SymlinkPathRecursive(filepath)
	}
	var tmp string
	for i := -1; ; i++ { //let's make sure we aren't going to coincidentally extracting to a directoyr that already has a temp directory in it...
		if i == -1 {
			tmp = destination + "/.tmp"
		} else {
			tmp = destination + "/.tmp" + strconv.Itoa(i)
		}
		_, err = os.Open(tmp)
		if os.IsNotExist(err) {
			break
		} else if err != nil {
			return err //make sure other issues aren't going to cause this loop to run forever.
		}
	}
	defer os.RemoveAll(tmp)
	cmd := exec.Command("unsquashfs", "-o", strconv.Itoa(r.offset), "-d", tmp, r.path, filepath)
	err = cmd.Run()
	if err != nil {
		return err
	}
	name := path.Base(filepath)
	if origName != "" {
		name = origName
	}
	err = os.Rename(tmp+"/"+filepath, destination+"/"+name)
	if err != nil {
		return err
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
	return &rdr, nil
}

//makes sure that the path is nice and only points to ONE file, which is needed if there are wildcards.
//If you were to search for *.desktop, you will get both blender.desktop AND /usr/bin/blender.desktop.
//This could cause issues, especially for FileReader
//
//Probably a bit spagetti and can be cleaned up. Maybe add a rawPaths variable to type1reader to make
//it easier to find a match with wildcards.
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
		return "", errors.New("File not found in the archive")
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
		return "", errors.New("File not found in the archive")
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
	return ioutil.NopCloser(&out), nil
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
	output := strings.TrimSuffix(string(wrt.Bytes()), "\n")
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
