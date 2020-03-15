package main

import (
	"github.com/probonopd/go-appimage/internal/helpers"
	"github.com/shuheiktgw/go-travis"
	"io"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestAppDirDeploy(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestAskForConfirmation(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AskForConfirmation(); got != tt.want {
				t.Errorf("AskForConfirmation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContains(t *testing.T) {
	type args struct {
		s []string
		e string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.args.s, tt.args.e); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateAppImage(t *testing.T) {
	type args struct {
		appdir string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestNewLibrary(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want ELF
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewLibrary(tt.args.path); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewLibrary() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPatchFile(t *testing.T) {
	type args struct {
		path    string
		search  string
		replace string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := PatchFile(tt.args.path, tt.args.search, tt.args.replace); (err != nil) != tt.wantErr {
				t.Errorf("PatchFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestScanFile(t *testing.T) {
	type args struct {
		f      io.ReadSeeker
		search []byte
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScanFile(tt.args.f, tt.args.search); got != tt.want {
				t.Errorf("ScanFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetTravisEnv(t *testing.T) {
	type args struct {
		client            *travis.Client
		repoSlug          string
		existingVars      []string
		name              string
		value             string
		travisSettingsURL string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func Test_appendLib(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func Test_constructMQTTPayload(t *testing.T) {
	type args struct {
		name    string
		version string
		FSTime  time.Time
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := constructMQTTPayload(tt.args.name, tt.args.version, tt.args.FSTime)
			if (err != nil) != tt.wantErr {
				t.Errorf("constructMQTTPayload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("constructMQTTPayload() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_containsString(t *testing.T) {
	type args struct {
		slice   []string
		element string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsString(tt.args.slice, tt.args.element); got != tt.want {
				t.Errorf("containsString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_deployGtkDirectory(t *testing.T) {
	type args struct {
		appdir     helpers.AppDir
		gtkVersion int
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func Test_determineELFsInDirTree(t *testing.T) {
	type args struct {
		appdir                    helpers.AppDir
		pathToDirTreeToBeDeployed string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func Test_die(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func Test_exitIfFileExists(t *testing.T) {
	type args struct {
		file        string
		description string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func Test_findAllExecutablesAndLibraries(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findAllExecutablesAndLibraries(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("findAllExecutablesAndLibraries() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findAllExecutablesAndLibraries() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_findLibrary(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findLibrary(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("findLibrary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("findLibrary() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_findWithPrefixInLibraryLocations(t *testing.T) {
	type args struct {
		prefix string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findWithPrefixInLibraryLocations(tt.args.prefix)
			if (err != nil) != tt.wantErr {
				t.Errorf("findWithPrefixInLibraryLocations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findWithPrefixInLibraryLocations() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_generatePassword(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generatePassword(); got != tt.want {
				t.Errorf("generatePassword() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getCopyrightFile(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getCopyrightFile(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("getCopyrightFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getCopyrightFile() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getDeps(t *testing.T) {
	type args struct {
		binaryOrLib string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := getDeps(tt.args.binaryOrLib); (err != nil) != tt.wantErr {
				t.Errorf("getDeps() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getDirsFromSoConf(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getDirsFromSoConf(tt.args.path); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getDirsFromSoConf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getQtPrfxpath(t *testing.T) {
	type args struct {
		f         *os.File
		err       error
		qtVersion int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getQtPrfxpath(tt.args.f, tt.args.err, tt.args.qtVersion); got != tt.want {
				t.Errorf("getQtPrfxpath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_handleQt(t *testing.T) {
	type args struct {
		appdir    helpers.AppDir
		qtVersion int
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func Test_patchRpathsInElf(t *testing.T) {
	type args struct {
		appdir                   helpers.AppDir
		libraryLocationsInAppDir []string
		path                     string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func Test_posString(t *testing.T) {
	type args struct {
		slice   []string
		element string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := posString(tt.args.slice, tt.args.element); got != tt.want {
				t.Errorf("posString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_readRpaths(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readRpaths(tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("readRpaths() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readRpaths() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_setupSigning(t *testing.T) {
	tests := []struct {
		name string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestGenerateAppImage1(t *testing.T) {
	type args struct {
		appdir string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func Test_constructMQTTPayload1(t *testing.T) {
	type args struct {
		name    string
		version string
		FSTime  time.Time
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := constructMQTTPayload(tt.args.name, tt.args.version, tt.args.FSTime)
			if (err != nil) != tt.wantErr {
				t.Errorf("constructMQTTPayload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("constructMQTTPayload() got = %v, want %v", got, tt.want)
			}
		})
	}
}