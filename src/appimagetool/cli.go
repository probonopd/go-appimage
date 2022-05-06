package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/probonopd/go-appimage/internal/helpers"
	"github.com/urfave/cli/v2"
)

// array of string, Sections contains
// * update information
// * sha256 signature of the appimage
// * signature key
// * MD5 digest
var Sections = []string{".upd_info", ".sha256_sig", ".sig_key", ".digest_md5"}

// bootstrapAppImageDeploy wrapper function to deploy an AppImage
// from Desktop file
// 		Args: c: cli.Context
func bootstrapAppImageDeploy(c *cli.Context) error {
	// make sure the user provided one and one only desktop
	if c.NArg() != 1 {
		log.Println("Please supply the path to a desktop file in an FHS-like AppDir")
		log.Println("a FHS-like structure, e.g.:")
		log.Println(os.Args[0], "appdir/usr/share/applications/myapp.desktop")
		log.Fatal("Terminated.")
	}
	options = DeployOptions{
		standalone:     c.Bool("standalone"),
		libAppRunHooks: c.Bool("libapprun_hooks"),
	}
	AppDirDeploy(c.Args().Get(0))
	return nil
}

// bootstrapValidateAppImage wrapper function to validate a AppImage
// 		Args: c: cli.Context
func bootstrapValidateAppImage(c *cli.Context) error {

	// make sure that we received only 1 file path
	if c.NArg() != 1 {
		log.Fatal("Please specify the file path to an AppImage to validate")
	}

	// get the first argument, which is file path to the AppImage
	filePathToValidate := c.Args().Get(0)

	// does the file exist? if not early-exit
	if !helpers.CheckIfFileExists(filePathToValidate) {
		log.Fatal("The specified file could not be found")
	}

	// Calculate the SHA256 signature
	d := helpers.CalculateSHA256Digest(filePathToValidate)
	log.Println("Calculated sha256 digest:", d)
	ent, err := helpers.CheckSignature(filePathToValidate)

	if err != nil {
		// we encountered an error :(
		log.Fatal("Could not validate the signature of", filePathToValidate)
	}

	log.Println(filePathToValidate, "has a valid signature")

	// TODO: Do something useful with this information
	log.Println("Identities:", ent.Identities)
	log.Println("KeyIdShortString:", ent.PrimaryKey.KeyIdShortString())
	log.Println("CreationTime:", ent.PrimaryKey.CreationTime)
	log.Println("KeyId:", ent.PrimaryKey.KeyId)
	log.Println("Fingerprint:", ent.PrimaryKey.Fingerprint)

	// happily ever after! no errors occured
	return nil
}

// bootstrapSetupSigning wrapper function to setup signing in
// the current Git repository
// 		Args: c: cli.Context
func bootstrapSetupSigning(c *cli.Context) error {
	return setupSigning(c.Bool("overwrite"))
}

// bootstrapAppImageSections is a function which converts cli.Context to
// string based arguments. Wrapper function to show the sections of the AppImage
// 		Args: c: cli.Context
func bootstrapAppImageSections(c *cli.Context) error {
	// check if the number of arguments are stictly 1, if not
	// return
	if c.NArg() != 1 {
		log.Fatal("Please specify the file path to an AppImage to validate")

	}
	fileToAppImage := c.Args().Get(0)

	// does the file exist? if not early-exit
	if !helpers.CheckIfFileExists(fileToAppImage) {
		log.Fatal("The specified file could not be found")
	}

	fmt.Println("")
	for _, section := range Sections {
		offset, length, err := helpers.GetSectionOffsetAndLength(fileToAppImage, section)
		if err != nil {
			log.Println("Error getting ELF section", section, err)
		} else {
			uidata, err := helpers.GetSectionData(fileToAppImage, section)
			fmt.Println("")
			if err != nil {
				_, _ = os.Stderr.WriteString("Could not find  ELF section " + section + ", exiting\n")
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
	return nil
}

// bootstrapAppImageBuild is a function which converts cli.Context to
// string based arguments, checks if all the files
// provided as arguments exists. If yes add the current path to PATH,
// check if all the necessary dependencies exist,
// finally check if the provided argument, AppDir is a directly.
// Call GenerateAppImage with the converted arguments
// 		Args: c: cli.Context
func bootstrapAppImageBuild(c *cli.Context) error {

	// check if the number of arguments are stictly 1, if not
	// return
	if c.NArg() != 1 {
		log.Fatal("Please specify the path to the AppDir which you would like to aid.")

	}
	fileToAppDir := c.Args().Get(0)

	// does the file exist? if not early-exit
	if !helpers.CheckIfFileOrFolderExists(fileToAppDir) {
		log.Fatal("The specified directory does not exist")
	}

	// Add the location of the executable to the $PATH
	helpers.AddHereToPath()

	// Check for needed files on $PATH
	tools := []string{"file", "mksquashfs", "desktop-file-validate", "uploadtool", "patchelf", "desktop-file-validate", "patchelf"} // "sh", "strings", "grep" no longer needed?; "curl" is needed for uploading only, "glib-compile-schemas" is needed in some cases only
	// curl is needed by uploadtool; TODO: Replace uploadtool with native Go code
	// "sh", "strings", "grep" are needed by appdirtool to parse qt_prfxpath; TODO: Replace with native Go code
	helpers.CheckIfAllToolsArePresent(tools)

	// Check whether we have a sufficient version of mksquashfs for -offset
	if helpers.CheckIfSquashfsVersionSufficient("mksquashfs") == false {
		os.Exit(1)
	}

	// Check if is directory, then assume we want to convert an AppDir into an AppImage
	fileToAppDir, _ = filepath.EvalSymlinks(fileToAppDir)
	if info, err := os.Stat(fileToAppDir); err == nil && info.IsDir() {
		// Generate the AppImage
		// for optimum performance, the following default parameters are passed
		// fileToAppDir: 				fileToAppDir
		// generateUpdateInformation: 	true (always guess based on environment variables)
		// squashfsCompressionType: 	zstd
		// checkAppStreamMetadata: 		true (always verify the appstream metadata if files exists
		// 								using appstreamcli
		// updateinformation: 			"" 	(empty string, we want to guess the update information from
		//								scratch, and if we fail to guess it, then no update metadata for
		// 								the appimage)
		GenerateAppImage(
			fileToAppDir, "",
			true,
			"",
			"zstd",
			true,
			"",
			"appimagetool",
		)
	} else {
		// TODO: If it is a file, then check if it is an AppImage and if yes, extract it
		log.Fatal("Supplied argument is not a directory \n" +
			"To extract an AppImage, run it with --appimage-extract \n")

	}
	return nil
}

// main Command Line Entrypoint. Defines the command line structure
// and assign each subcommand and option to the appropriate function
// which should be triggered when the subcommand is used
func main() {

	var version string

	// Derive the commit message from -X main.commit=$YOUR_VALUE_HERE
	// if the build does not have the commit variable set externally,
	// fall back to unsupported custom build
	if commit != "" {
		version = commit
	} else {
		version = "unsupported custom build"
	}

	// let the user know that we are running within a docker container
	checkRunningWithinDocker()

	// build the Command Line interface
	// https://github.com/urfave/cli/blob/master/docs/v2/manual.md

	// basic information
	app := &cli.App{
		Name:                 "appimagetool",
		Authors:              []*cli.Author{{Name: "Simon Peter"}},
		Version:              version,
		Usage:                "An automatic tool to create AppImages",
		EnableBashCompletion: false,
		HideHelp:             false,
		HideVersion:          false,
		Compiled:             time.Time{},
		Copyright:            "MIT License",
		Action:               bootstrapAppImageBuild,
	}

	// define subcommands, like 'deploy', 'validate', ...
	app.Commands = []*cli.Command{
		{
			Name:   "deploy",
			Usage:  "Turns PREFIX directory into AppDir by deploying dependencies and AppRun file",
			Action: bootstrapAppImageDeploy,
		},
		{
			Name:   "validate",
			Usage:  "Calculate the sha256 digest and check whether the signature is valid",
			Action: bootstrapValidateAppImage,
		},
		{
			Name:   "setupsigning",
			Usage:  "Prepare a git repository that is used with Travis CI for signing AppImages",
			Action: bootstrapSetupSigning,
		},
		{
			Name:   "sections",
			Usage:  "",
			Action: bootstrapAppImageSections,
		},
	}

	// define flags, such as --libapprun_hooks, --standalone here ...
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "libapprun_hooks",
			Aliases: []string{"l"},
			Usage:   "Use libapprun_hooks",
		},
		&cli.BoolFlag{
			Name:    "overwrite",
			Aliases: []string{"o"},
			Usage:   "Overwrite existing files",
		},
		&cli.BoolFlag{
			Name:    "standalone",
			Aliases: []string{"s"},
			Usage:   "Make standalone self-contained bundle",
		},
	}

	// TODO: move travis based Sections to travis.go in future
	if os.Getenv("TRAVIS_TEST_RESULT") == "1" {
		log.Fatal("$TRAVIS_TEST_RESULT is 1, exiting...")
	}

	errRuntime := app.Run(os.Args)
	if errRuntime != nil {
		log.Fatal(errRuntime)
	}

}
