// TODO: Use https://github.com/src-d/go-git or https://github.com/google/go-github to
// * Get changelog history and publish it on PubSub

package main

import (
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"gopkg.in/ini.v1"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agriardyan/go-zsyncmake/zsync"
	"github.com/probonopd/appimage/internal/helpers"
)

// https://blog.kowalczyk.info/article/vEja/embedding-build-number-in-go-executable.html
// The build script needs to set, e.g.,
// go build -ldflags "-X main.commit=$TRAVIS_BUILD_NUMBER"
var commit string

func main() {

	var version string
	if commit != "" {
		version = commit
	} else {
		version = "unsupported custom build"
	}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, filepath.Base(os.Args[0])+" "+version+"\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Tool to convert an AppDir into an AppImage.\n")
		fmt.Fprintf(os.Stderr, "If it is running on Travis CI, it also uploads the AppImage\nto GitHub Releases, creates update and publishes the information needed\nfor updating the AppImage.\n")
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, filepath.Base(os.Args[0])+" <path to AppDir>\n")
		fmt.Fprintf(os.Stderr, "\n")

		flag.PrintDefaults()
	}
	flag.Parse()

	// Always show version
	fmt.Println(filepath.Base(os.Args[0]), version)

	// Add the location of the executable to the $PATH
	helpers.AddHereToPath()

	// Check for needed files on $PATH
	tools := []string{"file", "mksquashfs", "desktop-file-validate", "uploadtool"}
	for _, t := range tools {
		_, err := exec.LookPath(t)
		if err != nil {
			fmt.Println("Required helper tool", t, "missing")
			os.Exit(1)
		}
	}

	// Check whether we have a sufficient version of mksquashfs for -offset
	if helpers.CheckIfSquashfsVersionSufficient("mksquashfs") == false {
		os.Exit(1)
	}

	// Check if first argument is present, exit otherwise
	if len(os.Args) < 2 {
		os.Stderr.WriteString("Please specify an AppDir to be converted to an AppImage \n")
		os.Exit(1)
	}

	// Check if is directory, then assume we want to convert an AppDir into an AppImage
	firstArg, _ := filepath.EvalSymlinks(os.Args[1])
	if info, err := os.Stat(firstArg); err == nil && info.IsDir() {
		GenerateAppImage(firstArg)
	} else {
		// TODO: If it is a file, then check if it is an AppImage and if yes, extract it
		os.Stderr.WriteString("Supplied argument is not a directory \n")
		os.Stderr.WriteString("To extract an AppImage, run it with --appimage-extract \n")
		os.Exit(1)
	}
}

// GenerateAppImage converts an AppDir into an AppImage
func GenerateAppImage(appdir string) {
	if _, err := os.Stat(appdir + "/AppRun"); os.IsNotExist(err) {
		os.Stderr.WriteString("AppRun is missing \n")
		os.Exit(1)
	}
	// Guess update information
	// Check if $VERSION is empty and git is on the path, if yes "git rev-parse --short HEAD"
	// TODO: Use native git in Go, and on Travis use Travis build numbers (too)
	version := ""
	version = os.Getenv("VERSION")
	_, err := exec.LookPath("git")
	if version == "" && err == nil {
		version, err := exec.Command("git", "rev-parse", "--short", "HEAD", appdir).Output()
		if err != nil {
			os.Stderr.WriteString("Could not determine version automatically, please supply the application version as $VERSION " + filepath.Base(os.Args[0]) + " ... \n")
			os.Exit(1)
		} else {
			fmt.Println("NOTE: Using", version, "from 'git rev-parse --short HEAD' as the version")
			fmt.Println("      Please set the $VERSION environment variable if this is not intended")
		}
	}

	// If no desktop file found, exit
	n := len(helpers.FilesWithSuffixInDirectory(appdir, ".desktop"))
	if n < 1 {
		os.Stderr.WriteString("No top-level desktop file found in " + appdir + ", aborting\n")
		os.Exit(1)
	}

	// If more than one desktop files found, exit
	if n > 1 {
		os.Stderr.WriteString("Multiple top-level desktop files found in" + appdir + ", aborting\n")
		os.Exit(1)
	}

	desktopfile := helpers.FilesWithSuffixInDirectory(appdir, ".desktop")[0]

	err = helpers.ValidateDesktopFile(desktopfile)
	helpers.PrintError("ValidateDesktopFile", err)
	if err != nil {
		os.Exit(1)
	}

	// Read information from .desktop file

	// Check for presence of required keys and abort otherwise
	d, err := ini.Load(desktopfile)
	helpers.PrintError("ini.load", err)
	neededKeys := []string{"Categories", "Name", "Exec", "Type", "Icon"}
	for _, k := range neededKeys {
		if d.Section("Desktop Entry").HasKey(k) == false {
			os.Stderr.WriteString(".desktop file is missing a '" + k + "'= key\n")
			os.Exit(1)
		}
	}

	val, _ := d.Section("Desktop Entry").GetKey("Icon")
	iconname := val.String()
	if strings.Contains(iconname, "/") {
		os.Stderr.WriteString("Desktop file contains Icon= entry with a path, aborting\n")
		os.Exit(1)
	}

	if strings.Contains(filepath.Base(iconname), ".") {
		os.Stderr.WriteString("Desktop file contains Icon= entry with '.', aborting\n")
		os.Exit(1)
	}

	// Read "Name=" key and convert spaces into underscores
	val, _ = d.Section("Desktop Entry").GetKey("Name")
	name := val.String()
	nameWithUnderscores := strings.Replace(name, " ", "_", -1)

	fmt.Println(nameWithUnderscores)

	// Determine the architecture
	// If no $ARCH variable is set check all .so that we can find to determine the architecture
	var archs []string
	if os.Getenv("ARCH") == "" {
		res, err := helpers.GetElfArchitecture(appdir + "/AppRun")
		if err == nil {
			archs = helpers.AppendIfMissing(archs, res)
			fmt.Println("Architecture from AppRun:", res)
		} else {
			err := filepath.Walk(appdir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					helpers.PrintError("Determine architecture", err)
				} else if info.IsDir() == false && strings.Contains(info.Name(), ".so.") {
					arch, err := helpers.GetElfArchitecture(path)
					helpers.PrintError("Determine architecture", err)
					fmt.Println("Architecture of", info.Name(), arch)
					archs = helpers.AppendIfMissing(archs, arch)
				}
				return nil
			})
			helpers.PrintError("Determine architecture", err)
		}
	} else {
		archs = helpers.AppendIfMissing(archs, os.Getenv("ARCH"))
		fmt.Println("Architecture from $ARCH:", os.Getenv("ARCH"))
	}

	if len(archs) != 1 {
		os.Stderr.WriteString("Could not determine architecture automatically, please supply it as $ARCH " + filepath.Base(os.Args[0]) + " ... \n")
		os.Exit(1)
	}
	arch := archs[0]

	// Set VERSION in desktop file and save it
	d, err = ini.Load(desktopfile)
	ini.PrettyFormat = false
	helpers.PrintError("ini.load", err)
	d.Section("Desktop Entry").Key("X-AppImage-Version").SetValue(version)
	err = d.SaveTo(desktopfile)
	helpers.PrintError("Save desktop file", err)

	// Construct target AppImage filename
	target := nameWithUnderscores + "-" + version + "-" + arch + ".AppImage"
	fmt.Println(target)

	var iconfile string

	// Check if we find a png matching the Icon= key in the top-level directory of the AppDir
	// We insist on a png because otherwise we need to costly convert it to png at integration time
	// since thumbails need to be in png format
	if helpers.CheckIfFileExists(appdir+"/"+iconname+".png") == true {
		iconfile = appdir + "/" + iconname + ".png"
	} else if helpers.CheckIfFileExists(appdir + "/usr/share/icons/hicolor/256x256/" + iconname + ".png") {
		// Search in usr/share/icons/hicolor/256x256 and copy from there
		input, err := ioutil.ReadFile(appdir + "/usr/share/icons/hicolor/256x256/" + iconname + ".png")
		if err != nil {
			fmt.Println(err)
			return
		}
		err = ioutil.WriteFile(appdir+".DirIcon", input, 0644)
		if err != nil {
			fmt.Println("Error copying ticon to", appdir+".DirIcon")
			fmt.Println(err)
			return
		}
	} else {
		os.Stderr.WriteString("Could not find icon file at " + appdir + "/" + iconname + ".png" + ", exiting\n")
		os.Exit(1)
	}
	fmt.Println(iconfile)

	fmt.Println("TODO: Check validity and size of png")

	// "Deleting pre-existing .DirIcon"
	if helpers.CheckIfFileExists(appdir+"/.DirIcon") == true {
		fmt.Println("Deleting pre-existing .DirIcon")
		os.Remove(appdir + "/.DirIcon")
	}

	// "Copying .DirIcon in place based on information from desktop file"
	err = helpers.CopyFile(iconfile, appdir+"/.DirIcon")
	if err != nil {
		helpers.PrintError("Copy .DirIcon", err)
		os.Exit(1)
	}

	// Check if AppStream upstream metadata is present in source AppDir
	// If yes, use ximion's appstreamcli to make sure that desktop file and appdata match together and are valid
	appstreamfile := appdir + "/usr/share/metainfo/" + filepath.Base(desktopfile) + ".appdata.xml"
	if helpers.CheckIfFileExists(appstreamfile) == false {
		fmt.Println("WARNING: AppStream upstream metadata is missing, please consider creating it in")
		fmt.Println("         " + appdir + "/usr/share/metainfo/" + filepath.Base(desktopfile) + ".appdata.xml")
		fmt.Println("         Please see https://www.freedesktop.org/software/appstream/docs/chap-Quickstart.html#sect-Quickstart-DesktopApps")
		fmt.Println("         for more information or use the generator at")
		fmt.Println("         http://output.jsbin.com/qoqukof")
	} else {
		fmt.Println("Trying to validate AppStream information with the appstreamcli tool")
		_, err := exec.LookPath("appstreamcli")
		if err != nil {
			fmt.Println("Required helper tool appstreamcli missing")
			os.Exit(1)
		}
		err = helpers.ValidateAppStreamMetainfoFile(appdir)
		if err != nil {
			fmt.Println("In case of questions regarding the validation, please refer to https://github.com/ximion/appstream")
			os.Exit(1)
		}
	}

	runtimedir := filepath.Clean(helpers.Here() + "/../share/AppImageKit/runtime/")
	if _, err := os.Stat(runtimedir); os.IsNotExist(err) {
		runtimedir = helpers.Here()
	}
	runtimefilepath := runtimedir + "/runtime-" + arch
	if helpers.CheckIfFileExists(runtimefilepath) == false {
		os.Stderr.WriteString("Cannot find " + runtimefilepath + ", exiting\n")
		fmt.Println("It should have been bundled, but you can get it from https://github.com/AppImage/AppImageKit/releases/continuous")
		// TODO: Download it from there?
		os.Exit(1)
	}

	// Find out the size of the binary runtime
	fi, err := os.Stat(runtimefilepath)
	if err != nil {
		helpers.PrintError("runtime", err)
		os.Exit(1)
	}
	offset := fi.Size()

	// We supply our own fstime rather than letting mksquashfs determine it
	// so that we know its value for being able to publish it
	FSTime := time.Now()
	fstime := strconv.FormatInt(FSTime.Unix(), 10) // Seconds since epoch.  Default to current time

	// Turns out that using time.Now() is more precise than a Unix timestamp (seconds precision).
	// Hence we convert back from the Unix timestamp to be consistent.
	if n, err := strconv.Atoi(fstime); err == nil {
		FSTime = time.Unix(int64(n), 0)
	} else {
		fmt.Println("Time conversion error:", fstime, "is not an integer.")
		FSTime = time.Unix(0, 0)
	}

	// "mksquashfs", source, destination, "-offset", offset, "-comp", "gzip", "-root-owned", "-noappend"
	cmd := exec.Command("mksquashfs", appdir, target, "-offset", strconv.FormatInt(offset, 10), "-fstime", fstime, "-comp", "gzip", "-root-owned", "-noappend")
	fmt.Println(cmd.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		helpers.PrintError("mksquashfs", err)
		fmt.Printf("%s", string(out))
		os.Exit(1)
	}

	// Embed the binary runtime into the squashfs
	fmt.Println("Embedding ELF...")

	err = helpers.WriteFileIntoOtherFileAtOffset(runtimefilepath, target, 0)
	if err != nil {
		helpers.PrintError("Embedding runtime", err)
		fmt.Printf("%s", string(out))
		os.Exit(1)
	}

	fmt.Println("Marking the AppImage as executable...")
	os.Chmod(target, 0755)

	// Get the filesize in bytes of the resulting AppImage
	fi, err = os.Stat(target)
	if err != nil {
		helpers.PrintError("Could not get size of AppImage", err)
		os.Exit(1)
	}

	// Construct update information
	var updateinformation string

	// If we know this is a GitLab CI build
	// do nothing at the moment but print some nice message
	// https://docs.gitlab.com/ee/ci/variables/#predefined-variables-environment-variables
	// CI_PROJECT_URL
	// "CI_COMMIT_REF_NAME"); The branch or tag name for which project is built
	// "CI_JOB_NAME"); The name of the job as defined in .gitlab-ci.yml
	if os.Getenv("CI_COMMIT_REF_NAME") != "" {
		fmt.Println("Running on GitLab CI")
		fmt.Println("Will not calculate update information for GitLab because GitLab does not support HTTP range requests yet")
	}

	// If we know this is a Travis CI build,
	// then fill in update information based on TRAVIS_REPO_SLUG
	//     https://docs.travis-ci.com/user/environment-variables/#Default-Environment-Variables
	//     TRAVIS_COMMIT: The commit that the current build is testing.
	//     TRAVIS_REPO_SLUG: The slug (in form: owner_name/repo_name) of the repository currently being built.
	//     TRAVIS_TAG: If the current build is for a git tag, this variable is set to the tag’s name.
	//     TRAVIS_PULL_REQUEST
	if os.Getenv("TRAVIS_REPO_SLUG") != "" {
		fmt.Println("Running on Travis CI")
		if os.Getenv("TRAVIS_PULL_REQUEST") != "false" {
			fmt.Println("Will not calculate update information for GitHub because this is a pull request")
		} else if os.Getenv("GITHUB_TOKEN") == "" {
			fmt.Println("Will not calculate update information for GitHub because $GITHUB_TOKEN is missing")
			fmt.Println("please set it in the Travis CI Repository Settings for this project.")
			fmt.Println("You can get one from https://github.com/settings/tokens")
		} else {
			parts := strings.Split(os.Getenv("TRAVIS_REPO_SLUG"), "/")
			var channel string
			if os.Getenv("TRAVIS_TAG") != "" && os.Getenv("TRAVIS_TAG") != "continuous" {
				channel = "latest"
			} else {
				channel = "continuous"
			}
			updateinformation = "gh-releases-zsync|" + parts[0] + "|" + parts[1] + "|" + channel + "|" + nameWithUnderscores + "-" + "*-" + arch + ".AppImage.zsync"
			fmt.Println("Calculated updateinformation:", updateinformation)
		}
	}

	if updateinformation != "" {

		err = helpers.ValidateUpdateInformation(updateinformation)
		if err != nil {
			helpers.PrintError("VerifyUpdateInformation", err)
			os.Exit(1)
		}

		embedStringInSegment(target, ".upd_info", updateinformation)
	}

	// TODO: calculate and embed MD5 digest
	// https://github.com/AppImage/AppImageKit/blob/801e789390d0e6848aef4a5802cd52da7f4abafb/src/appimagetool.c#L961
	// Blocked by https://github.com/AppImage/AppImageSpec/issues/29
	// This is actually very useful for PubSub notifications, to find out whether we have already have
	// an identical AppImage on the local machine or not

	// Calculate AppImage MD5 digest according to
	// https://github.com/AppImage/libappimage/blob/4d6f5f3d5b6c8c01c39b8ce0364b74cd6e4043c7/src/libappimage_shared/digest.c
	// The ELF sections
	// .digest_md5
	// .sha256_sig
	// .sig_key
	// need to be skipped, although I think
	// .upd_info
	// ought to be skipped, too
	md5.New()

	// TODO: Signing.

	// Embed public key into .sig_key section
	// FIXME: Find out where the git top-level directory is and read the pubkey from there...
	buf, err := ioutil.ReadFile(helpers.PubkeyFileName)
	if err != nil {
		fmt.Println("TODO: Find out where the git top-level directory is and read the pubkey from there...")
		helpers.PrintError("Could not read pubkey", err)
		os.Exit(0)
	}
	embedStringInSegment(target, ".sig_key", string(buf))

	if updateinformation == "" {
		// No updateinformation was provided nor calculated, so the following steps make no sense
		fmt.Println("Almost a success")
		fmt.Println("")
		fmt.Println("The AppImage was created, but is lacking update information.")
		fmt.Println("Possibly it was built on a local developer machine.")
		fmt.Println("Such an AppImage is fine for local use but should not be distributed.")
		fmt.Println("Please build on one of the supported CI systems like Travis CI")
		fmt.Println("if you want your AppImage to be updatable\nand have update notifications published.")
		os.Exit(0)
	}

	// If updateinformation was provided, then we also generate the zsync file (after having signed the AppImage)
	if updateinformation != "" {
		opts := zsync.Options{0, "", filepath.Base(target)}
		zsync.ZsyncMake(target, opts)

		// Check if the zsync file is really there
		fi, err = os.Stat(target + ".zsync")
		if err != nil {
			helpers.PrintError("zsync file not generated", err)
			os.Exit(1)
		}
	}

	// Create the payload the publishing
	pl, _ := constructMQTTPayload(name, version, FSTime)
	fmt.Println(pl)

	// Upload and publish if we know this is a Travis CI build
	// https://github.com/probonopd/uploadtool says
	// Note that UPLOADTOOL* variables will be used in bash script to form a JSON request,
	// that means some characters like double quotes and new lines need to be escaped
	// TODO: Instead of trying to somehow force this into uploadtool, do it properly in Go.
	body, err := helpers.GetCommitMessageForThisCommitOnTravis()
	fmt.Println("Commit message for this commit:", body)

	if os.Getenv("TRAVIS_REPO_SLUG") != "" {
		cmd := exec.Command("uploadtool", target, target+".zsync")
		fmt.Println(cmd.String())
		out, err := cmd.CombinedOutput()
		fmt.Printf("%s", string(out))
		if err != nil {
			helpers.PrintError("uploadtool", err)
			os.Exit(1)
		}

		// If upload succeeded, publish MQTT message
		// TODO: Message AppImageHub instead, which in turn messages the clients

		helpers.PublishMQTTMessage(updateinformation, pl)
	}

	fmt.Println("Success")
	fmt.Println("")
	fmt.Println("Please consider submitting your AppImage to AppImageHub, the crowd-sourced")
	fmt.Println("central directory of available AppImages, by opening a pull request")
	fmt.Println("at https://github.com/AppImage/appimage.github.io")
}

func embedStringInSegment(path string, segment string, s string) {
	// Find offset and length of segment
	uidata, err := helpers.GetSectionData(path, segment)
	helpers.PrintError("GetSectionData for '"+segment+"'", err)
	if err != nil {
		os.Stderr.WriteString("Could not find section " + segment + " in runtime, exiting\n")
		os.Exit(1)
	}
	fmt.Println("Embedded " + segment + " section before embedding:")
	fmt.Println(uidata)
	uioffset, uilength, err := helpers.GetSectionOffsetAndLength(path, ".upd_info")
	helpers.PrintError("GetSectionData for '"+segment+"'", err)
	if err != nil {
		os.Stderr.WriteString("Could not determine offset and length of " + segment + " in runtime, exiting\n")
		os.Exit(1)
	}
	fmt.Println("Embedded "+segment+" section offset:", uioffset)
	fmt.Println("Embedded "+segment+" section length:", uilength)
	// Exit if data exceeds available space in segment
	if len(s) > len(uidata) {
		os.Stderr.WriteString("does not fit into " + segment + " segment, exiting\n")
		os.Exit(1)
	}
	fmt.Println("Writing into "+segment+" segment...", uilength)
	// Seek file to ui_offset and write it there
	helpers.WriteStringIntoOtherFileAtOffset(s, path, uioffset)
	helpers.PrintError("GetSectionData for '"+segment+"'", err)
	if err != nil {
		os.Stderr.WriteString("Could write into " + segment + " segment, exiting\n")
		os.Exit(1)
	}
	uidata, err = helpers.GetSectionData(path, segment)
	helpers.PrintError("GetSectionData for '"+segment+"'", err)
	if err != nil {
		os.Stderr.WriteString("Could not find section " + segment + " in runtime, exiting\n")
		os.Exit(1)
	}
	fmt.Println("Embedded " + segment + " section now contains:")
	fmt.Println(string(uidata))
}

func constructMQTTPayload(name string, version string, FSTime time.Time) (string, error) {

	psd := helpers.PubSubData{
		Name:    name,
		Version: version,
		FSTime:  FSTime,
		// Size:    size,
		// Fruit:   []string{"Apple", "Banana", "Orange"},
		// Id:      999,
		// private: "Unexported field",
		// Created: time.Now(),
	}

	var jsonData []byte
	jsonData, err := json.Marshal(psd)
	if err != nil {
		return "", err
	}
	// Print it in a nice readable form, unlike the one that actually gets returned
	var jsonDataReadable []byte
	jsonDataReadable, err = json.MarshalIndent(psd, "", "    ")
	if err != nil {
		return "", err
	}
	fmt.Println(string(jsonDataReadable))

	return string(jsonData), nil
}
