package main

import (
	"github.com/probonopd/go-appimage/internal/helpers"
	"github.com/urfave/cli/v2"
	"log"
	"os"
	"path/filepath"
	"time"
)


// listFilesInAppImage lists the files in the AppImage, similar to
// the ls command in UNIX systems
func listFilesInAppImage(path string) {
	appimage := NewAppImage(path)
	err := appimage.ShowContents(false)
	if err != nil {
		log.Fatal("Failed to list files: error code:", err)
	}

}

// listLongFilesInAppImage lists all the files in the appimage
// similar to `ls -al`. It shows the file permissions also
func listLongFilesInAppImage(path string) {
	appimage := NewAppImage(path)
	err := appimage.ShowContents(true)
	if err != nil {
		log.Fatal("Failed to list files: error code:", err)
	}

}


// bootstrapMkAppImage is a function which converts cli.Context to
// string based arguments, checks if all the files
// provided as arguments exists. If yes add the current path to PATH,
// check if all the necessary dependencies exist,
// finally check if the provided argument, AppDir is a directly.
// Call GenerateAppImage with the converted arguments
// 		Args: c: cli.Context
func bootstrapMkAppImage(c *cli.Context) error {

	// check if the number of arguments are stictly 1, if not
	// return
	if c.NArg() < 1 {
		log.Fatal("Please specify the path to the AppDir/AppImage which you would like to aid.")
	}
	fileToAppDir := c.Args().Get(0)

	// does the file exist? if not early-exit
	if ! helpers.CheckIfFileOrFolderExists(fileToAppDir) {
		log.Fatal("The specified directory does not exist")
	}

	// Add the location of the executable to the $PATH
	helpers.AddHereToPath()

	// Check whether we have a sufficient version of mksquashfs for -offset
	if helpers.CheckIfSquashfsVersionSufficient("mksquashfs") == false {
		os.Exit(1)
	}

	// Check if is directory, then assume we want to convert an AppDir into an AppImage
	fileToAppDir, _ = filepath.EvalSymlinks(fileToAppDir)
	osStatInfo, osStatErr := os.Stat(fileToAppDir)

	// we had some errors while stat'ing the file
	// exit early
	if osStatErr != nil {
		log.Fatal("Failed to process the supplied directory / AppImage. Is it a valid directory / AppImage?")
	}


	if osStatInfo.IsDir() {
		// check if the file provided is an AppDir Directory

		// Check for needed files on $PATH
		// curl is needed by uploadtool; TODO: Replace uploadtool with native Go code
		// "sh", "strings", "grep" are needed by appdirtool to parse qt_prfxpath; TODO: Replace with native Go code
		tools := []string{"file", "mksquashfs", "desktop-file-validate", "uploadtool", "patchelf", "desktop-file-validate", "patchelf"} // "sh", "strings", "grep" no longer needed?; "curl" is needed for uploading only, "glib-compile-schemas" is needed in some cases only
		helpers.CheckIfAllToolsArePresent(tools)

		// check if we need to guess the update information
		shouldGuessUpdateInformation := false
		if c.Bool("guess") {
			shouldGuessUpdateInformation = true
		}

		// is manual compressor provided? if yes use that, else default
		compressionType := "gzip"
		if c.String("comp") != "" {
			compressionType = c.String("comp")
		}

		// should we validate the appstream files?
		shouldValidateAppstream := true
		if c.Bool("no-appstream") {
			shouldValidateAppstream = false
		}

		// get the update information from the --updateinformation flag on CLI
		// by default, we assume its empty string
		receivedUpdateInformation := ""
		if c.String("updateinformation") != "" {
			// NOTE: if you provide both --updateinformation and --guess parameter together,
			// then the guess has precedence over the provided updateinformation
			receivedUpdateInformation = c.String("updateinformation")
		}

		// now generate the appimage
		GenerateAppImage(fileToAppDir, shouldGuessUpdateInformation, compressionType, shouldValidateAppstream, receivedUpdateInformation)


	} else {
		if c.Bool("list") || c.Bool("listlong") {
			// check if the file provided as argument is an AppImage
			// Check for needed files on $PATH
			tools := []string{"unsquashfs", "bsdtar", "file", "mksquashfs", "desktop-file-validate", "uploadtool", "patchelf", "desktop-file-validate", "patchelf"} // "sh", "
			// curl is needed by uploadtool; TODO: Replace uploadtool with native Go code
			// "sh", "strings", "grep" are needed by appdirtool to parse qt_prfxpath; TODO: Replace with native Go code
			helpers.CheckIfAllToolsArePresent(tools)
			if c.Bool("list") {
				listFilesInAppImage(fileToAppDir)
			} else {
				listLongFilesInAppImage(fileToAppDir)
			}
		} else {
			log.Fatal("The supplied argument is a file and is not known in " +
				"combination with the provided arguments.")
		}

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
		Name:                   "mkappimage",
		Authors: 				[]*cli.Author{{Name: "AppImage Project"}},
		Version:                version,
		Usage:            		"Core tool to convert AppDir to AppImage (experimental)",
		EnableBashCompletion:   false,
		HideHelp:               false,
		HideVersion:            false,
		Compiled:               time.Time{},
		Copyright:              "MIT License",
		Action: 				bootstrapMkAppImage,

	}

	// define flags, such as --libapprun_hooks, --standalone here ...
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name: "libapprun_hooks",
			Usage: "Use libapprun_hooks",
		},
		&cli.BoolFlag{
			Name: "overwrite",
			Aliases: []string{"o"},
			Usage: "Overwrite existing files",
		},
		&cli.BoolFlag{
			Name: "guess",
			Aliases: []string{"g"},
			Usage: "Guess update information based on GitHub, Travis CI or GitLab environment variables",
		},
		&cli.BoolFlag{
			Name: "standalone",
			Aliases: []string{"s"},
			Usage: "Make standalone self-contained bundle",
		},
		&cli.BoolFlag{
			Name: "no-appstream",
			Aliases: []string{"n"},
			Usage: "Do not check AppStream metadata",
		},
		&cli.StringFlag{
			Name: "comp",
			Usage: "Squashfs compression",
		},
		&cli.StringFlag{
			Name: "updateinformation",
			Aliases: []string{"u", "updateinfo"},
			Usage: "Embed update information STRING; if zsyncmake is installed, generate zsync file",
		},
		&cli.BoolFlag{
			Name: "list",
			Aliases: []string{"l"},
			Usage: "List files in SOURCE AppImage",
		},
		&cli.BoolFlag{
			Name: "listlong",
			Aliases: []string{"ll"},
			Usage: "List files in SOURCE AppImage (similar to ls -al)",
		},
		&cli.BoolFlag{
			Name: "verbose",
			Usage: "Produce verbose output",
		},
	}

	errRuntime := app.Run(os.Args)
	if errRuntime != nil {
		log.Fatal(errRuntime)
	}

}
