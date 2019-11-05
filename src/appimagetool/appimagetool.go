package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/probonopd/appimage/internal/helpers"
)

// https://blog.kowalczyk.info/article/vEja/embedding-build-number-in-go-executable.html
// The build script needs to set those using
// now=$(date +'%Y-%m-%d_%T')
// go build -ldflags "-X main.commit=$(git rev-parse HEAD) -X main.buildTime=$now"
var (
	commit    string // sha1 revision used to build the program
	buildTime string // when the executable was built
)

var flgVersion bool

func main() {

	// Parse command line arguments
	flag.BoolVar(&flgVersion, "version", false, "Show version number")
	flag.Parse()

	// Always show version, but exit immediately if only the version number was requested
	if commit != "" {
		fmt.Printf("Build on %s from sha1 %s\n", buildTime, commit)
	} else {
		fmt.Println("Unsupported local developer build")
	}
	if flgVersion {
		os.Exit(0)
	}

	// Check for needed files on $PATH
	tools := []string{"file", "mksquashfs", "desktop-file-validate", "appstreamcli"}
	for _, t := range tools {
		if helpers.IsCommandAvailable(t) == false {
			fmt.Println("Required helper tool", t, "missing")
			os.Exit(1)
		}
	}

	// Check if first argument is present, exit otherwise
	if len(os.Args) < 2 {
		os.Stderr.WriteString("Please specify an AppDir to be converted to an AppImage")
		os.Exit(1)
	}

	// Check if is directory, then assume we want to convert an AppDir into an AppImage
	firstArg, _ := filepath.EvalSymlinks(os.Args[1])
	if info, err := os.Stat(firstArg); err == nil && info.IsDir() {
		GenerateAppImage(firstArg)
	} else {
		// TODO: If it is a file, then check if it is an AppImage and if yes, extract it
		os.Stderr.WriteString("Supplied argument is not a directory")
		os.Exit(1)
	}
}

// GenerateAppImage converts an AppDir into an AppImage
func GenerateAppImage(appdir string) {

	// Get VERSION environment variable
	// os.Getenv("VERSION")

	// Guess update information

	// Check if $VERSION is empty and git is on the path, if yes "git rev-parse --short HEAD"
	version := ""
	version = os.Getenv("VERSION")
	if version == "" && helpers.IsCommandAvailable("git") == true {
		version, err := exec.Command("git", "rev-parse", "--short", "HEAD", appdir).Output()
		helpers.PrintError("git rev-parse", err)
		if err == nil {
			fmt.Println("NOTE: Using", version, "from 'git rev-parse --short HEAD' as the version")
			fmt.Println("      Please set the $VERSION environment variable if this is not intended")
		}
	}

	// Check if *.desktop file is present in source AppDir
	// find_first_matching_file_nonrecursive(source, "*.desktop");

	// If no desktop file found, exit
	// "Desktop file not found, aborting"

	// if(g_find_program_in_path ("desktop-file-validate")) {
	//    if(validate_desktop_file(desktop_file) != 0){
	//        fprintf(stderr, "ERROR: Desktop file contains errors. Please fix them. Please see\n");
	//        fprintf(stderr, "       https://standards.freedesktop.org/desktop-entry-spec/1.0/n");
	//        die("       for more information.");
	//    }
	// }

	// /Read information from .desktop file

	// ".desktop file is missing a Categories= key"

	// Read "Name");
	// replace " ", "_");

	// Determine the architecture
	// getenv("ARCH")

	// If no $ARCH variable is set check all .so that we can find to determine the architecture
	// find_arch(source, "*.so.*", archs);
	// "Unable to guess the architecture of the AppDir source directory"
	// or
	// "More than one architectures were found of the AppDir source directory"
	// "A valid architecture with the ARCH environmental variable should be provided\ne.g. ARCH=x86_64 %s", argv[0]),

	// set VERSION in desktop file and save it
	// g_key_file_set_string(kf, G_KEY_FILE_DESKTOP_GROUP, "X-AppImage-Version", version_env);

	// "Could not save modified desktop file"

	// if (version_env != NULL) {
	// sprintf(dest_path, "%s-%s-%s.AppImage", app_name_for_filename, version_env, arch);

	// Read Icon= and find pngs with that name
	// in top-level directory
	// and as a fallback elsewhere, check their sizes, prefer 256x256

	// "Deleting pre-existing .DirIcon"

	// "Copying .DirIcon in place based on information from desktop file"

	// /Check if AppStream upstream metadata is present in source AppDir
	// "/usr/share/metainfo/" + replacestr(".desktop", ".appdata.xml");

	// "WARNING: AppStream upstream metadata is missing, please consider creating it\n");
	// "         in usr/share/metainfo/%s\n", application_id);
	// "         Please see https://www.freedesktop.org/software/appstream/docs/chap-Quickstart.html#sect-Quickstart-DesktopApps\n");
	// "         for more information or use the generator at http://output.jsbin.com/qoqukof.\n");

	// /Use ximion's appstreamcli to make sure that desktop file and appdata match together
	// "Trying to validate AppStream information with the appstreamcli tool"
	// "In case of issues, please refer to https://github.com/ximion/appstream"
	// "appstreamcli validate-tree %s"

	// Find out the size of the binary runtime
	// offset =

	// "mksquashfs", source, destination, "-offset", offset, "-comp", "gzip", "-root-owned", "-noappend"

	// Embed the binary runtime into the squashfs
	// "Embedding ELF..."

	// "Marking the AppImage as executable...

	// Construct update information

	// If the user has not provided update information but we know this is a Travis CI build,
	// then fill in update information based on TRAVIS_REPO_SLUG
	//     https://docs.travis-ci.com/user/environment-variables/#Default-Environment-Variables
	//     TRAVIS_COMMIT: The commit that the current build is testing.
	//     TRAVIS_REPO_SLUG: The slug (in form: owner_name/repo_name) of the repository currently being built.
	//     TRAVIS_TAG: If the current build is for a git tag, this variable is set to the tag’s name.
	//     TRAVIS_PULL_REQUEST

	// $GITHUB_TOKEN missing
	// "Will not guess update information since $GITHUB_TOKEN is missing"

	// If the user has not provided update information but we know this is a GitLab CI build
	// do nothing at the moment but print some nice message
	// https://docs.gitlab.com/ee/ci/variables/#predefined-variables-environment-variables
	// CI_PROJECT_URL
	// "CI_COMMIT_REF_NAME"); The branch or tag name for which project is built
	// "CI_JOB_NAME"); The name of the job as defined in .gitlab-ci.yml

	// If updateinformation was provided, then we check and embed it

	// if(!g_str_has_prefix(updateinformation,"zsync|"))
	// if(!g_str_has_prefix(updateinformation,"bintray-zsync|"))
	// if(!g_str_has_prefix(updateinformation,"gh-releases-zsync|"))
	// die("The provided updateinformation is not in a recognized format");

	// Find offset and length of updateinformation

	// Section  ".upd_info"
	// unsigned long ui_offset =
	// unsigned long ui_length =
	// "Could not find section .upd_info in runtime"
	// "Could not determine offset for updateinformation"

	// Exit if updateinformation exceeds available space
	// "updateinformation does not fit into segment, aborting"

	// Seek file to ui_offset and write it there

	// TODO: calculate and embed MD5 digest
	// https://github.com/AppImage/AppImageKit/blob/801e789390d0e6848aef4a5802cd52da7f4abafb/src/appimagetool.c#L961
	// Blocked by https://github.com/AppImage/AppImageSpec/issues/29

	// TODO: Signing. It is pretty convoluted and hardly anyone is using it. Drop it?

	// If updateinformation was provided, then we also generate the zsync file (after having signed the AppImage)

	// "Success"
	// ""
	// "Please consider submitting your AppImage to AppImageHub, the crowd-sourced"
	// "central directory of available AppImages, by opening a pull request"
	// "at https://github.com/AppImage/appimage.github.io"

	fmt.Println("Nothing implemented yet")
}
