package goappimage

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mgord9518/imgconv"
	"github.com/probonopd/go-zsyncmake/zsync"
	"gopkg.in/ini.v1"

	"github.com/probonopd/go-appimage/internal/helpers"
)

// constructMQTTPayload TODO: Add documentation
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

// getMeta returns the version of the AppImage, and git repository, if any.
func getMeta() (string, string) {
	// TODO: Append 7-digit commit sha after the build number

	var version string
	version = os.Getenv("VERSION")
	travisBuildNumber := os.Getenv("TRAVIS_BUILD_NUMBER")
	// On Travis use $TRAVIS_BUILD_NUMBER
	if version == "" && travisBuildNumber != "" {
		log.Println("NOTE: Using", travisBuildNumber, "from $TRAVIS_BUILD_NUMBER as the version")
		log.Println("      Please set the $VERSION environment variable if this is not intended")
		version = travisBuildNumber
	}

	githubRunNumber := os.Getenv("GITHUB_RUN_NUMBER")
	// On GitHub Actions use $GITHUB_RUN_NUMBER
	if version == "" && githubRunNumber != "" {
		log.Println("NOTE: Using", githubRunNumber, "from $GITHUB_RUN_NUMBER as the version")
		log.Println("      Please set the $VERSION environment variable if this is not intended")
		version = githubRunNumber
	}

	gitRoot := ""
	gitRepo, err := helpers.GetGitRepository()
	if err != nil {
		log.Println("Apparently not in a git repository")
		if version == "" {
			log.Panic("Version not found, aborting. Set it with VERSION=... " + os.Args[0] + "\n")
		}
	} else {
		gitWt, err := gitRepo.Worktree()
		if err == nil {
			gitRoot = gitWt.Filesystem.Root()
			log.Println("git root:", gitRoot)
			if version == "" {
				gitHead, err := gitRepo.Head()
				if err != nil {
					log.Panic("Could not determine version automatically, " +
						"please supply the application version as $VERSION " +
						filepath.Base(os.Args[0]) + " ... \n")
				} else {
					version = gitHead.Hash().String()[:7] // This equals 'git rev-parse --short HEAD'
					log.Println("NOTE: Using", version, "from 'git rev-parse --short HEAD' as the version")
					log.Println("      Please set the $VERSION environment variable if this is not intended")
				}
			}
		} else {
			fmt.Println("Could not get root of git repository")
		}
	}

	return version, gitRoot
}

// GenerateAppImage converts an AppDir into an AppImage
func GenerateAppImage(
	appdir string,
	destination string,
	generateUpdateInformation bool,
	runtimeFile string,
	squashfsCompressionType string,
	checkAppStreamMetadata bool,
	updateInformation string,
	source string,
) error {
	var err error
	// does the file exist? if not early-exit
	if !helpers.CheckIfFileOrFolderExists(appdir) {
		return fmt.Errorf("the specified directory does not exist")
	}

	if _, err = os.Stat(appdir + "/AppRun"); os.IsNotExist(err) {
		return fmt.Errorf("AppRun is missing: %w", err)
	}

	desktopFiles := helpers.FilesWithSuffixInDirectory(appdir, ".desktop")
	switch n := len(desktopFiles); {
	case n < 1: // If no desktop file found, exit
		return fmt.Errorf("No top-level desktop file found in " + appdir + ", aborting\n")
	case n > 1: // If more than one desktop files found, exit
		return fmt.Errorf("Multiple top-level desktop files found in " + appdir + ", aborting\n")
	}
	desktopFile := desktopFiles[0]

	err = helpers.ValidateDesktopFile(desktopFile)
	if err != nil {
		return fmt.Errorf("ValidateDesktopFile %w", err)
	}

	// Read information from .desktop file
	err = helpers.CheckDesktopFile(desktopFile)
	if err != nil {
		return fmt.Errorf("CheckDesktopFile %w", err)
	}

	// Read "Name=" key and convert spaces into underscores
	d, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, // Do not cripple lines hat contain ";"
		desktopFile)
	if err != nil {
		return fmt.Errorf("ini.load %w", err)
	}
	val, _ := d.Section("Desktop Entry").GetKey("Name")
	name := val.String()
	nameWithUnderscores := strings.Replace(name, " ", "_", -1)
	fmt.Println(nameWithUnderscores)

	// Get the name of the icon
	val, _ = d.Section("Desktop Entry").GetKey("Icon")
	iconname := val.String()

	// Determine the architecture
	// If no $ARCH variable is set check all .so that we can find to determine the architecture
	var archs []string
	if os.Getenv("ARCH") == "" {
		res, err := helpers.GetElfArchitecture(appdir + "/AppRun")
		if err == nil {
			archs = helpers.AppendIfMissing(archs, res)
			log.Println("Architecture from AppRun:", res)
		} else {
			err := filepath.Walk(appdir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return fmt.Errorf("determine architecture %w", err)
				} else if !info.IsDir() && strings.Contains(info.Name(), ".so.") {
					arch, err := helpers.GetElfArchitecture(path)
					if err != nil {
						// we received an error when analyzing the arch
						return fmt.Errorf("determine architecture %w", err)
					} else if !helpers.SliceContains(archs, arch) {
						log.Println("Architecture of", info.Name()+":", arch)
						archs = helpers.AppendIfMissing(archs, arch)
					} else {
						// FIXME: we found some data, but still it was not a part of the
						// known architectures
						errArchNotKnown := errors.New("could not detect a valid architecture")
						return errArchNotKnown
					}
				}
				return nil
			})
			return fmt.Errorf("determine architecture %w", err)
		}
	} else {
		archs = helpers.AppendIfMissing(archs, os.Getenv("ARCH"))
		fmt.Println("Architecture from $ARCH:", os.Getenv("ARCH"))
	}

	if len(archs) != 1 {
		return fmt.Errorf("could not determine architecture automatically, please supply it as $ARCH %s ... ", filepath.Base(os.Args[0]))
	}
	arch := archs[0]

	version, gitRoot := getMeta()

	// If no version found, exit
	if version == "" && source != "mkappimage" {
		// version is not required for mkappimage
		return fmt.Errorf("version not found, aborting. Set it with VERSION=... %s", os.Args[0])
	} else if version != "" {
		// Set VERSION in desktop file and save it
		d, err = ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, // Do not cripple lines hat contain ";"
			desktopFile)
		ini.PrettyFormat = false
		if err != nil {
			return fmt.Errorf("ini.load %w", err)
		}
		d.Section("Desktop Entry").Key("X-AppImage-Version").SetValue(version)
		if err = d.SaveTo(desktopFile); err != nil {
			return fmt.Errorf("save desktop file %w", err)
		}
	}

	// Construct target AppImage filename
	// make sure the output directory exists before continuing
	target := destination
	if target == "" {
		// no destination directory was specified.
		// write the file to the current directory
		target = nameWithUnderscores + "-" + version + "-" + arch + ".AppImage"
	} else {
		targetFileInfo, err := os.Stat(target)
		if os.IsNotExist(err) {
			// the file does not exist.
			// check the parent directory exists
			targetDir := filepath.Dir(destination)
			if !helpers.CheckIfFolderExists(targetDir) {
				return fmt.Errorf("%s does not exist", targetDir)
			}
			// the parent directory exists. Make a fullpath to the destination appimage
			// with the basename filename following appimage conventions
		} else if err != nil {
			// we faced some other random error. Possibly messing around with symlinks or permissionError
			// log it and quit.
			return err
		} else {
			// the file or folder exists
			// check if it's a file or a folder.
			if targetFileInfo.IsDir() {
				// the user provided path is a directory
				target = filepath.Join(destination, nameWithUnderscores+"-"+version+"-"+arch+".AppImage")
			}
		}
	}

	log.Println("Target AppImage filename:", target)

	var iconfile string

	// Check if we find a png matching the Icon= key in the top-level directory of the AppDir
	// or at usr/share/icons/hicolor/256x256/apps/ in the AppDir
	// We insist on a png because otherwise we need to costly convert it to png at integration time
	// since thumbails need to be in png format
	supportedIconExtensions := []string{".png", ".xpm", ".svg"}
	for i := range supportedIconExtensions {
		if helpers.CheckIfFileExists(appdir+"/"+iconname+supportedIconExtensions[i]) == true {
			iconfile = appdir + "/" + iconname + supportedIconExtensions[i]
			break
		} else if helpers.CheckIfFileExists(appdir + "/usr/share/icons/hicolor/256x256/apps/" + iconname + supportedIconExtensions[i]) {
			iconfile = appdir + "/usr/share/icons/hicolor/256x256/apps/" + iconname + supportedIconExtensions[i]
			break
		}
	}
	if iconfile == "" {
		return fmt.Errorf("Could not find icon file at " + appdir + "/" + iconname + "{.png, .svg, .xpm}" + "\n" +
			"nor at " + appdir + "/usr/share/icons/hicolor/256x256/apps/" + iconname + "{.png, .svg, .xpm}" + ", exiting")
	}
	log.Println("Icon file:", iconfile)

	// TODO: Check validity and size of png"

	// Deleting pre-existing .DirIcon
	if helpers.CheckIfFileExists(appdir+"/.DirIcon") == true {
		log.Println("Deleting pre-existing .DirIcon")
		if err = os.Remove(appdir + "/.DirIcon"); err != nil {
			log.Printf("Error deleting .DirIcon: %s, try to keep going\n", err)
		}
	}

	// If the icon is a svg, attempt to convert it to a png
	// If that fails, just copy over the original icon
	iconext := iconfile[len(iconfile)-3:]
	if iconext == "svg" {
		err = imgconv.ConvertFileWithAspect(iconfile, appdir+"/.DirIcon", 256, "png")
		if err != nil {
			err = helpers.CopyFile(iconfile, appdir+"/.DirIcon")
		}
	} else {
		err = helpers.CopyFile(iconfile, appdir+"/.DirIcon")
	}

	if err != nil {
		return fmt.Errorf("on copy .DirIcon %w", err)
	}

	// Check if AppStream upstream metadata is present in source AppDir
	// If yes, use ximion's appstreamcli to make sure that desktop file and appdata match together and are valid
	appstreamfile := appdir + "/usr/share/metainfo/" + strings.ReplaceAll(filepath.Base(desktopFile), ".desktop", ".appdata.xml")
	if !checkAppStreamMetadata {
		log.Println("WARNING: Skipping AppStream metadata check...")
	} else if !helpers.CheckIfFileExists(appstreamfile) {
		log.Println("WARNING: AppStream upstream metadata is missing, please consider creating it in")
		fmt.Println("         " + appstreamfile)
		fmt.Println("         Please see https://www.freedesktop.org/software/appstream/docs/chap-Quickstart.html#sect-Quickstart-DesktopApps")
		fmt.Println("         for more information or use the generator at")
		fmt.Println("         https://appimagecommunity.github.io/simple-appstream-generator/")
	} else {
		fmt.Println("Trying to validate AppStream information with the appstreamcli tool")
		_, err := exec.LookPath("appstreamcli")
		if err != nil {
			return fmt.Errorf("required helper tool appstreamcli missing")
		}
		err = helpers.ValidateAppStreamMetainfoFile(appdir)
		if err != nil {
			return fmt.Errorf("faced %w\nIn case of questions regarding the validation, please refer to https://github.com/ximion/appstream", err)
		}
	}

	if len(runtimeFile) < 1 {
		runtimeDir := filepath.Clean(helpers.Here() + "/../share/AppImageKit/runtime/")
		if _, err := os.Stat(runtimeDir); os.IsNotExist(err) {
			runtimeDir = helpers.Here()
		}
		runtimeFile = runtimeDir + "/runtime-" + arch
		if !helpers.CheckIfFileExists(runtimeFile) {
			log.Println("Cannot find " + runtimeFile + ", exiting")
			log.Println("It should have been bundled, but you can get it from https://github.com/AppImage/AppImageKit/releases/continuous")
			// TODO: Download it from there?
			return fmt.Errorf("cannot find %s, it should have been bundled", runtimeFile)
		}
	} else if !helpers.CheckIfFileExists(runtimeFile) {
		return fmt.Errorf("cannot find %s, exiting", runtimeFile)
	}

	// Find out the size of the binary runtime
	fi, err := os.Stat(runtimeFile)
	if err != nil {
		return fmt.Errorf("runtime %w", err)
	}
	offset := fi.Size()

	// We supply our own fstime rather than letting mksquashfs determine it
	// so that we know its value for being able to publish it
	FSTime := time.Now()
	fstime := strconv.FormatInt(FSTime.Unix(), 10) // Seconds since epoch.  Default to current time

	// Turns out that using time.Now() is more precise than a Unix timestamp (seconds precision).
	// Hence, we convert back from the Unix timestamp to be consistent.
	if n, err := strconv.Atoi(fstime); err == nil {
		FSTime = time.Unix(int64(n), 0)
	} else {
		fmt.Println("Time conversion error:", fstime, "is not an integer.")
		FSTime = time.Unix(0, 0)
	}

	// Exit if we cannot set the permissions of the AppDir,
	// this is important e.g., for Firejail
	// https://github.com/AppImage/AppImageKit/issues/1032#issuecomment-596225173
	info, err := os.Stat(appdir) // TODO: Walk all directories instead of just looking at the AppDir itself
	if err != nil {
		return fmt.Errorf("cannot get the permissions of the AppDir %w", err)
	}
	m := info.Mode()
	if m&(1<<2) == 0 {
		// Other users don't have read permission, https://stackoverflow.com/a/45430141
		return fmt.Errorf("wrong permissions on AppDir, please set it to 0755 and try again")
	}

	// "mksquashfs", source, destination, "-offset", offset, "-comp", "zstd", "-root-owned", "-noappend", "-b", "1M"
	cmd := exec.Command("mksquashfs", appdir, target, "-offset", strconv.FormatInt(offset, 10), "-fstime", fstime, "-comp", squashfsCompressionType, "-root-owned", "-noappend", "-b", "1M")
	fmt.Println(cmd.String())
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("mksquashfs %w", err)
	}

	// Embed the binary runtime into the squashfs
	fmt.Println("Embedding ELF...")

	err = helpers.WriteFileIntoOtherFileAtOffset(runtimeFile, target, 0)
	if err != nil {
		return fmt.Errorf("embedding runtime %w", err)
	}

	fmt.Println("Marking the AppImage as executable...")
	_ = os.Chmod(target, 0755)

	// Get the filesize in bytes of the resulting AppImage
	fi, err = os.Stat(target) //TODO: start using variable fi
	if err != nil {
		return fmt.Errorf("could not get size of AppImage: %w", err)
	}

	// Construct update information
	// check if we have received the updateInformation param
	// in mkappimage, we have a -u --updateinformation flag which
	// allows to provide a string as update information
	// however, appimagetool calls this function with updateinformation=""
	// which will be overwritten in the following lines of code
	updateinformation := updateInformation

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
	ghToken, ghTokenFound := os.LookupEnv("GITHUB_TOKEN")
	if os.Getenv("TRAVIS_REPO_SLUG") != "" && generateUpdateInformation {
		fmt.Println("Running on Travis CI")
		if os.Getenv("TRAVIS_PULL_REQUEST") != "false" {
			fmt.Println("Will not calculate update information for GitHub because this is a pull request")
		} else if !ghTokenFound || ghToken == "" {
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

	// If we know this is a GitHub Actions workflow,
	// then fill in update information based on GITHUB_REPOSITORY
	//     https://docs.github.com/en/actions/configuring-and-managing-workflows/using-environment-variables#default-environment-variables
	//     GITHUB_REPOSITORY: The slug (in form: owner_name/repo_name) of the repository currently being built.
	//     GITHUB_REF: e.g., "refs/pull/421/merge", "refs/heads/master"
	if os.Getenv("GITHUB_REPOSITORY") != "" && generateUpdateInformation {
		fmt.Println("Running on GitHub Actions")
		if strings.Contains(os.Getenv("GITHUB_REF"), "/pull/") {
			fmt.Println("Will not calculate update information for GitHub because this is a pull request")
		} else {
			parts := strings.Split(os.Getenv("GITHUB_REPOSITORY"), "/")
			var channel string
			if os.Getenv("GITHUB_REF") != "" && os.Getenv("GITHUB_REF") != "refs/heads/master" {
				channel = "latest"
			} else {
				channel = "continuous"
			}
			updateinformation = "gh-releases-zsync|" + parts[0] + "|" + parts[1] + "|" + channel + "|" + nameWithUnderscores + "-" + "*-" + arch + ".AppImage.zsync"
			fmt.Println("Calculated updateinformation:", updateinformation)
		}
	}

	// declare an empty digest
	// we will replace this digest with a sha256 signature if the appimage
	// does not contain update information.
	// if it does contain update information, we should first try to sign it
	// with the PGP signature
	digest := ""

	if updateinformation != "" {
		err = helpers.ValidateUpdateInformation(updateinformation)
		if err != nil {
			return fmt.Errorf("VerifyUpdateInformation %w", err)
		}

		err = helpers.EmbedStringInSegment(target, ".upd_info", updateinformation)
		if err != nil {
			return fmt.Errorf("EmbedStringInSegment %w", err)
		}
	} else {
		// Embed the SHA256 digest only for appimages which are not having
		// update information.
		// Embed SHA256 digest into '.sha256_sig' section if it exists
		// This is not part of the AppImageSpec yet, but in the future we will want to put this into the AppImageSpec:
		// If an AppImage is not signed, it should have the SHA256 digest in the '.sha256_sig' section; this might
		// eventually remove the need for an extra '.digest_md5' section and hence simplify the format
		digest = helpers.CalculateSHA256Digest(target)
		err = helpers.EmbedStringInSegment(target, ".sha256_sig", digest)
		if err != nil {
			return fmt.Errorf("EmbedStringInSegment %w", err)
		}
	}

	// TODO: calculate and embed MD5 digest (in case we want to use it)
	// https://github.com/AppImage/AppImageKit/blob/801e789390d0e6848aef4a5802cd52da7f4abafb/src/appimagetool.c#L961

	/*
		TA sez:
		First, embed the update information
		Then comes the MD5 digest, don't ask me why
		then comes the signature
		and then the key
		So only signature and key must be zeroed for the signature checking
		Technically it may not make so much sense
	*/

	// The actual signing

	// Decrypt the private key which we need for signing
	if helpers.CheckIfFileExists(helpers.EncPrivkeyFileName) == true {
		_, ok := os.LookupEnv(helpers.EnvSuperSecret)
		if !ok {
			return fmt.Errorf("environment variable %s not present, cannot sign", helpers.EnvSuperSecret)
		}

		fmt.Println("Attempting to decrypt the private key...")
		// TODO: Replace with native Go code in ossl.go
		superSecret := os.Getenv(helpers.EnvSuperSecret)
		if superSecret == "" {
			return fmt.Errorf("could not get secure environment variable $%s, exiting", helpers.EnvSuperSecret)
		}
		// Note: 06065064:digital envelope routines:EVP_DecryptFinal_ex:bad decrypt:evp_enc.c:539
		// OpenSSL 1.1.0 changed from MD5 to SHA-256; they broke stuff (again). Adding '-md sha256' seems to solve it
		// TODO: Replace OpenSSL call with native Go code
		// https://stackoverflow.com/a/43847627
		cmd := "openssl aes-256-cbc -pass pass:" + superSecret + " -in " + helpers.EncPrivkeyFileName + " -out " + helpers.PrivkeyFileName + " -d -a -md sha256"
		err = helpers.RunCmdStringTransparently(cmd)
		if err != nil {
			fmt.Printf("Could not decrypt the private key using the password in $%s, exiting\n", helpers.EnvSuperSecret)
			return fmt.Errorf("could not decrypt the private key using the password in $%s", helpers.EnvSuperSecret)
		}
	}

	// Sign the AppImage
	if helpers.CheckIfFileExists(helpers.PrivkeyFileName) {
		fmt.Println("Attempting to sign the AppImage...")
		err = helpers.SignAppImage(target, digest)
		if err != nil {
			_ = os.Remove(helpers.PrivkeyFileName)
			return fmt.Errorf("SignAppImage %w", err)
		}
		_ = os.Remove(helpers.PrivkeyFileName)
	}

	// Embed public key into '.sig_key' section if it exists
	buf, err := os.ReadFile(gitRoot + "/" + helpers.PubkeyFileName)
	if err != nil {
		fmt.Println("Could not read "+gitRoot+"/"+helpers.PubkeyFileName+":", err)
	} else {
		err = helpers.EmbedStringInSegment(target, ".sig_key", string(buf))
		if err != nil {
			return fmt.Errorf("EmbedStringInSegment %w", err)
		}
	}

	// No updateinformation was provided nor calculated, so the following steps make no sense.
	// Hence, we print an information message and exit.
	if updateinformation == "" {
		fmt.Println("Almost a success")
		fmt.Println("")
		fmt.Println("The AppImage was created, but is lacking update information.")
		fmt.Println("Possibly it was built on a local developer machine.")
		fmt.Println("Such an AppImage is fine for local use but should not be distributed.")
		fmt.Println("Please build on one of the supported CI systems like Travis CI")
		fmt.Println("if you want your AppImage to be updatable\nand have update notifications published.")
		return nil
	}

	// If updateinformation was provided, then we also generate the zsync file (after having signed the AppImage)
	if updateinformation != "" {
		opts := zsync.Options{Url: filepath.Base(target)}
		zsync.ZsyncMake(target, opts)

		// Check if the zsync file is really there
		_, err = os.Stat(target + ".zsync")
		if err != nil {
			return fmt.Errorf("zsync file not generated %w", err)
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
	if err != nil {
		log.Println("Getting commit message from travis error:", err)
	} else {
		fmt.Println("Commit message for this commit:", body)
	}

	// If it's a TRAVIS CI, then upload the release assets and zsync file
	if os.Getenv("TRAVIS_REPO_SLUG") != "" {
		cmd := exec.Command("uploadtool", target, target+".zsync")
		fmt.Println(cmd.String())
		out, err := cmd.CombinedOutput()
		fmt.Printf("%s", string(out))
		if err != nil {
			return fmt.Errorf("uploadtool %w", err)
		}

		// If upload succeeded, publish MQTT message
		// TODO: Message AppImageHub instead, which in turn messages the clients

		helpers.PublishMQTTMessage(updateinformation, pl)
	}

	// everything went well.
	fmt.Println("Success")
	fmt.Println("")
	fmt.Println("Please consider submitting your AppImage to AppImageHub, the crowd-sourced")
	fmt.Println("central directory of available AppImages, by opening a pull request")
	fmt.Println("at https://github.com/AppImage/appimage.github.io")
	return nil
}
