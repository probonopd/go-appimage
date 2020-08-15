// TODO: Use https://github.com/src-d/go-git or https://github.com/google/go-github to
// * Get changelog history and publish it on PubSub

package main

import (
	// "crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agriardyan/go-zsyncmake/zsync"
	"gopkg.in/ini.v1"

	"github.com/probonopd/go-appimage/internal/helpers"
)

// https://blog.kowalczyk.info/article/vEja/embedding-build-number-in-go-executable.html
// The build script needs to set, e.g.,
// go build -ldflags "-X main.commit=$TRAVIS_BUILD_NUMBER"
var commit string

var standalonePtr = flag.Bool("s", false, "Make standalone (self-contained) bundle")

func main() {

	var version string
	if commit != "" {
		version = commit
	} else {
		version = "unsupported custom build"
	}

	// Detect if we are running inside Docker; https://github.com/AppImage/AppImageKit/issues/912
	// If the file /.dockerenv exists, and/or if /proc/1/cgroup begins with /lxc/ or /docker/
	res, err := ioutil.ReadFile("/proc/1/cgroup")
	if err != nil {
		helpers.PrintError("ReadFile /proc/1/cgroup", err)
		os.Exit(1)
	}
	if strings.HasPrefix(string(res), "/lxc") || strings.HasPrefix(string(res), "/docker") || helpers.Exists("/.dockerenv") == true {
		log.Println("Running inside Docker. Please make sure that the environment variables from Travis CI")
		log.Println("available inside Docker if you are running on Travis CI.")
		log.Println("This can be achieved by using something along the lines of 'docker run --env-file <(env)'.")
		log.Println("Please see https://github.com/docker/cli/issues/2210.")
	}

	if os.Getenv("TRAVIS_TEST_RESULT") == "1" {
		log.Println("$TRAVIS_TEST_RESULT is 1, exiting...")
		os.Exit(1)
	}

	sections := []string{".upd_info", ".sha256_sig", ".sig_key", ".digest_md5"}

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, filepath.Base(os.Args[0])+" "+version+"\n")
		fmt.Fprintf(os.Stderr, "\n")

		fmt.Fprintf(os.Stderr, "Tool to convert an AppDir into an AppImage.\n")
		fmt.Fprintf(os.Stderr, "If it is running on Travis CI, it also uploads the AppImage\nto GitHub Releases, creates update and publishes the information needed\nfor updating the AppImage.\n")
		fmt.Fprintf(os.Stderr, "\n")

		fmt.Fprintf(os.Stderr, "Usage:\n")

		fmt.Fprintf(os.Stderr, filepath.Base(os.Args[0])+" deploy <path to PREFIX directory>\n")
		fmt.Fprintf(os.Stderr, "\tTurns PREFIX directory into AppDir by deploying dependencies and AppRun file\n")

		fmt.Fprintf(os.Stderr, filepath.Base(os.Args[0])+" <path to AppDir>\n")
		fmt.Fprintf(os.Stderr, "\tConvert the supplied AppDir into an AppImage and \n\t(if running on Travis CI) sign, upload, and publish it\n")

		fmt.Fprintf(os.Stderr, filepath.Base(os.Args[0])+" validate <path to AppImage>\n")
		fmt.Fprintf(os.Stderr, "\tCalculate the sha256 digest and check whether the signature is valid\n")

		fmt.Fprintf(os.Stderr, filepath.Base(os.Args[0])+" setupsigning\n")
		fmt.Fprintf(os.Stderr, "\tPrepare a git repository that is used with Travis CI \n\tfor signing AppImages\n")

		fmt.Fprintf(os.Stderr, filepath.Base(os.Args[0])+" sections <path to AppImage>\n")
		fmt.Fprintf(os.Stderr, "\tPrint the AppImage specific ELF sections (for debugging), namely\n\t")
		for _, section := range sections {
			fmt.Print(section, " ")
		}
		fmt.Fprintf(os.Stderr, "\n")

		flag.PrintDefaults()
	}

	flag.Parse()

	// Always show version
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, filepath.Base(os.Args[0])+" "+version+"\n")
	fmt.Fprintf(os.Stderr, "\n")

	// Add the location of the executable to the $PATH
	helpers.AddHereToPath()

	// Check for needed files on $PATH
	tools := []string{"file", "mksquashfs", "desktop-file-validate", "uploadtool", "patchelf", "desktop-file-validate", "patchelf"} // "sh", "strings", "grep" no longer needed?; "curl" is needed for uploading only, "glib-compile-schemas" is needed in some cases only
	// curl is needed by uploadtool; TODO: Replace uploadtool with native Go code
	// "sh", "strings", "grep" are needed by appdirtool to parse qt_prfxpath; TODO: Replace with native Go code
	for _, t := range tools {
		_, err := exec.LookPath(t)
		if err != nil {
			log.Println("Required helper tool", t, "missing")
			os.Exit(1)
		}
	}

	// Check whether we have a sufficient version of mksquashfs for -offset
	if helpers.CheckIfSquashfsVersionSufficient("mksquashfs") == false {
		os.Exit(1)
	}

	if len(flag.Args()) > 0 {
		switch flag.Args()[0] {
		case "deploy":
			if len(flag.Args()) < 2 {
				log.Println("Please supply the path to a desktop file in an FHS-like AppDir")
				log.Println("a FHS-like structure, e.g.:")
				log.Println(os.Args[0], "appdir/usr/share/applications/myapp.desktop")
				os.Exit(1)
			}
			AppDirDeploy(flag.Args()[1])
			os.Exit(0)
		case "validate":
			if len(flag.Args()) > 1 {
				if helpers.CheckIfFileExists(os.Args[2]) {
					d := helpers.CalculateSHA256Digest(os.Args[2])
					log.Println("Calculated sha256 digest:", d)
					ent, err := helpers.CheckSignature(os.Args[2])
					if err == nil {
						log.Println(flag.Args()[1], "has a valid signature")
						// TODO: Do something useful with this information
						log.Println("Identities:", ent.Identities)
						log.Println("KeyIdShortString:", ent.PrimaryKey.KeyIdShortString())
						log.Println("CreationTime:", ent.PrimaryKey.CreationTime)
						log.Println("KeyId:", ent.PrimaryKey.KeyId)
						log.Println("Fingerprint:", ent.PrimaryKey.Fingerprint)
					} else {
						log.Println("Could not validate signature of", os.Args[2]+":", err)
						os.Exit(1)
					}
				} else {
					log.Println(flag.Args()[1], "does not exist")
					os.Exit(1)
				}
			} else {
				fmt.Println("Please specify an AppImage to validate")
				os.Exit(1)
			}
			os.Exit(0)
		case "setupsigning":
			setupSigning()
			os.Exit(0)
		case "sections":
			if len(flag.Args()) > 1 {
				if helpers.CheckIfFileExists(flag.Args()[1]) {

					fmt.Println("")
					for _, section := range sections {
						offset, length, err := helpers.GetSectionOffsetAndLength(flag.Args()[1], section)
						if err != nil {
							log.Println("Error getting ELF section", section, err)
						} else {
							uidata, err := helpers.GetSectionData(flag.Args()[1], section)
							fmt.Println("")
							if err != nil {
								os.Stderr.WriteString("Could not find  ELF section " + section + ", exiting\n")
								log.Println("Error getting ELF section", section, err)
							} else {
								log.Println("ELF section", section, "offset", offset, "length", length)
								fmt.Println("")
								fmt.Println(uidata)
								fmt.Println("")
								fmt.Println("Which is as a string:")
								fmt.Println("")
								fmt.Println(string(uidata))
								fmt.Println("")
								fmt.Println("===========================================================")
								fmt.Println("")
							}
						}
					}
				} else {
					log.Println(flag.Args()[1], "does not exist")
					os.Exit(1)
				}
			} else {
				log.Println("Please specify an AppImage to print the sections")
				os.Exit(1)
			}
			os.Exit(0)

		}
	}

	// Check if first argument is present, exit otherwise
	if len(flag.Args()) < 1 {
		os.Stderr.WriteString("Please specify an AppDir to be converted to an AppImage \n")
		os.Exit(1)
	}

	// Check if is directory, then assume we want to convert an AppDir into an AppImage
	firstArg, _ := filepath.EvalSymlinks(flag.Args()[0])
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

	var version string
	version = os.Getenv("VERSION")
	travisBuildNumber := os.Getenv("TRAVIS_BUILD_NUMBER")
	// On Travis use $TRAVIS_BUILD_NUMBER
	if version == "" && travisBuildNumber != "" {
		log.Println("NOTE: Using", travisBuildNumber, "from $TRAVIS_BUILD_NUMBER as the version")
		log.Println("      Please set the $VERSION environment variable if this is not intended")
		version = travisBuildNumber
	}

	gitRoot := ""
	gitRepo, err := helpers.GetGitRepository()
	if err != nil {
		log.Println("Apparently not in a git repository")
	} else {
		gitWt, err := gitRepo.Worktree()
		if err == nil {
			gitRoot = gitWt.Filesystem.Root()
			log.Println("git root:", gitRoot)
			if version == "" {
				gitHead, _ := gitRepo.Head()
				version = gitHead.Hash().String()[:7] // This equals 'git rev-parse --short HEAD'
				if err != nil {
					os.Stderr.WriteString("Could not determine version automatically, please supply the application version as $VERSION " + filepath.Base(os.Args[0]) + " ... \n")
					os.Exit(1)
				} else {
					log.Println("NOTE: Using", version, "from 'git rev-parse --short HEAD' as the version")
					log.Println("      Please set the $VERSION environment variable if this is not intended")
				}
			}
		} else {
			fmt.Println("Could not get root of git repository")
		}
	}

	// If no version found, exit
	if version == "" {
		os.Stderr.WriteString("Version not found, aborting. Set it with VERSION=... " + os.Args[0] + "\n")
		os.Exit(1)
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

	err = helpers.CheckDesktopFile(desktopfile)
	if err != nil {
		helpers.PrintError("CheckDesktopFile", err)
		os.Exit(1)
	}

	// Read "Name=" key and convert spaces into underscores
	d, err := ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, // Do not cripple lines hat contain ";"
		desktopfile)
	helpers.PrintError("ini.load", err)
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
					helpers.PrintError("Determine architecture", err)
				} else if info.IsDir() == false && strings.Contains(info.Name(), ".so.") {
					arch, err := helpers.GetElfArchitecture(path)
					helpers.PrintError("Determine architecture", err)
					if helpers.SliceContains(archs, arch) == false {
						log.Println("Architecture of", info.Name()+":", arch)
					}
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
	d, err = ini.LoadSources(ini.LoadOptions{IgnoreInlineComment: true}, // Do not cripple lines hat contain ";"
		desktopfile)
	ini.PrettyFormat = false
	helpers.PrintError("ini.load", err)
	d.Section("Desktop Entry").Key("X-AppImage-Version").SetValue(version)
	err = d.SaveTo(desktopfile)
	helpers.PrintError("Save desktop file", err)

	// Construct target AppImage filename
	target := nameWithUnderscores + "-" + version + "-" + arch + ".AppImage"
	log.Println("Target AppImage filename:", target)

	var iconfile string

	// Check if we find a png matching the Icon= key in the top-level directory of the AppDir
	// or at usr/share/icons/hicolor/256x256/apps/ in the AppDir
	// We insist on a png because otherwise we need to costly convert it to png at integration time
	// since thumbails need to be in png format
	if helpers.CheckIfFileExists(appdir+"/"+iconname+".png") == true {
		iconfile = appdir + "/" + iconname + ".png"
	} else if helpers.CheckIfFileExists(appdir + "/usr/share/icons/hicolor/256x256/apps/" + iconname + ".png") {
		// Search in usr/share/icons/hicolor/256x256 and copy from there
		input, err := ioutil.ReadFile(appdir + "/usr/share/icons/hicolor/256x256/apps/" + iconname + ".png")
		if err != nil {
			log.Println(err)
			return
		}
		iconfile = appdir + "/usr/share/icons/hicolor/256x256/apps/" + iconname + ".png"
	} else {
		os.Stderr.WriteString("Could not find icon file at " + appdir + "/" + iconname + ".png" + "\n")
		os.Stderr.WriteString("nor at " + appdir + "/usr/share/icons/hicolor/256x256/apps/" + iconname + ".png" + ", exiting\n")
		os.Exit(1)
	}
	log.Println("Icon file:", iconfile)

	log.Println("TODO: Check validity and size of png")

	// Deleting pre-existing .DirIcon
	if helpers.CheckIfFileExists(appdir+"/.DirIcon") == true {
		log.Println("Deleting pre-existing .DirIcon")
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
	appstreamfile := appdir + "/usr/share/metainfo/" + strings.Replace(filepath.Base(desktopfile), ".desktop", ".appdata.xml", -1)
	if helpers.CheckIfFileExists(appstreamfile) == false {
		log.Println("WARNING: AppStream upstream metadata is missing, please consider creating it in")
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
		log.Println("Cannot find " + runtimefilepath + ", exiting")
		log.Println("It should have been bundled, but you can get it from https://github.com/AppImage/AppImageKit/releases/continuous")
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

	// Exit if we cannot set the permissions of the AppDir,
	// this is important e.g., for Firejail
	// https://github.com/AppImage/AppImageKit/issues/1032#issuecomment-596225173
	info, err := os.Stat(appdir) // TODO: Walk all directories instead of just looking at the AppDir itself
	m := info.Mode()
	if m&(1<<2) == 0 {
		// Other users don't have read permission, https://stackoverflow.com/a/45430141
		log.Println("Wrong permissions on AppDir, please set it to 0755 and try again")
		os.Exit(1)
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
	ghToken, ghTokenFound := os.LookupEnv("GITHUB_TOKEN")
	if os.Getenv("TRAVIS_REPO_SLUG") != "" {
		fmt.Println("Running on Travis CI")
		if os.Getenv("TRAVIS_PULL_REQUEST") != "false" {
			fmt.Println("Will not calculate update information for GitHub because this is a pull request")
		} else if ghTokenFound == false || ghToken == "" {
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

		helpers.EmbedStringInSegment(target, ".upd_info", updateinformation)
		if err != nil {
			helpers.PrintError("EmbedStringInSegment", err)
			os.Exit(1)
		}
	}

	// Embed SHA256 digest into '.sha256_sig' section if it exists
	// This is not part of the AppImageSpec yet, but in the future we will want to put this into the AppImageSpec:
	// If an AppImage is not signed, it should have the SHA256 digest in the '.sha256_sig' section; this might
	// eventually remove the need for an extra '.digest_md5' section and hence simplify the format
	digest := helpers.CalculateSHA256Digest(target)
	err = helpers.EmbedStringInSegment(target, ".sha256_sig", digest)
	if err != nil {
		helpers.PrintError("EmbedStringInSegment", err)
		os.Exit(1)
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
		if ok != true {
			fmt.Println("Environment variable", helpers.EnvSuperSecret, "not present, cannot sign")
			os.Exit(1)
		}

		fmt.Println("Attempting to decrypt the private key...")
		// TODO: Replace with native Go code in ossl.go
		superSecret := os.Getenv(helpers.EnvSuperSecret)
		if superSecret == "" {
			fmt.Println("Could not get secure environment variable $" + helpers.EnvSuperSecret + ", exiting")
			os.Exit(1)
		}
		// Note: 06065064:digital envelope routines:EVP_DecryptFinal_ex:bad decrypt:evp_enc.c:539
		// OpenSSL 1.1.0 changed from MD5 to SHA-256; they broke stuff (again). Adding '-md sha256' seems to solve it
		// TODO: Replace OpenSSL call with native Go code
		// https://stackoverflow.com/a/43847627
		cmd := "openssl aes-256-cbc -pass pass:" + superSecret + " -in " + helpers.EncPrivkeyFileName + " -out " + helpers.PrivkeyFileName + " -d -a -md sha256"
		err = helpers.RunCmdStringTransparently(cmd)
		if err != nil {
			fmt.Println("Could not decrypt the private key using the password in $" + helpers.EnvSuperSecret + ", exiting")
			os.Exit(1)
		}
	}

	if helpers.CheckIfFileExists(helpers.PrivkeyFileName) == true {
		fmt.Println("Attempting to sign the AppImage...")
		err = helpers.SignAppImage(target, digest)
		if err != nil {
			helpers.PrintError("SignAppImage", err)
			os.Remove(helpers.PrivkeyFileName)
			os.Exit(1)
		}
		os.Remove(helpers.PrivkeyFileName)
	}

	// Embed public key into '.sig_key' section if it exists
	buf, err := ioutil.ReadFile(gitRoot + "/" + helpers.PubkeyFileName)
	if err != nil {
		fmt.Println("Could not read "+gitRoot+"/"+helpers.PubkeyFileName+":", err)
	} else {
		err = helpers.EmbedStringInSegment(target, ".sig_key", string(buf))
		if err != nil {
			helpers.PrintError("EmbedStringInSegment", err)
			os.Exit(1)
		}
	}

	if updateinformation == "" {
		// No updateinformation was provided nor calculated, so the following steps make no sense.
		// Hence we print an information message and exit.
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
