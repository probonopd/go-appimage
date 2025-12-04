// Self-update functionality for appimaged
// Implements automatic update checking and downloading from GitHub releases

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/go-github/github"
	"github.com/probonopd/go-appimage/internal/helpers"
)

// selfUpdate performs a self-update of the appimaged AppImage
// It downloads the latest version from GitHub and replaces the current executable
func selfUpdate() error {
	if thisai == nil {
		return fmt.Errorf("cannot determine current AppImage path")
	}

	// Get the update information from the current AppImage
	ui := thisai.updateinformation
	if ui == "" {
		return fmt.Errorf("no update information embedded in this AppImage")
	}

	// Parse the update information
	// Format: gh-releases-zsync|user|repo|release|filename.zsync
	parts := strings.Split(ui, "|")
	if len(parts) < 5 || parts[0] != "gh-releases-zsync" {
		return fmt.Errorf("unsupported update information format: %s", ui)
	}

	owner := parts[1]
	repo := parts[2]
	releaseName := parts[3]

	log.Printf("Checking for updates from %s/%s (release: %s)", owner, repo, releaseName)

	// Get the appropriate architecture suffix
	arch := getArchSuffix()
	if arch == "" {
		return fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	// Find the download URL for the new AppImage
	downloadURL, err := getLatestAppImageURL(owner, repo, releaseName, arch)
	if err != nil {
		return fmt.Errorf("failed to get download URL: %v", err)
	}

	log.Printf("Downloading update from: %s", downloadURL)

	// Download to a temporary file
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "appimaged-update.AppImage")

	err = downloadFile(tmpFile, downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %v", err)
	}

	// Make the downloaded file executable
	err = os.Chmod(tmpFile, 0755)
	if err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to make update executable: %v", err)
	}

	// Get the current AppImage path
	currentPath := thisai.Path

	// Create a backup of the current AppImage
	backupPath := currentPath + ".bak"
	err = os.Rename(currentPath, backupPath)
	if err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to create backup: %v", err)
	}

	// Move the new AppImage to the current location
	err = os.Rename(tmpFile, currentPath)
	if err != nil {
		// Restore backup on failure
		os.Rename(backupPath, currentPath)
		return fmt.Errorf("failed to install update: %v", err)
	}

	// Remove the backup
	os.Remove(backupPath)

	log.Println("Self-update completed successfully!")
	sendDesktopNotification("Update complete", "appimaged has been updated.\nPlease restart the daemon.", 10000)

	return nil
}

// getArchSuffix returns the architecture suffix used in AppImage filenames
func getArchSuffix() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "386":
		return "i686"
	case "arm64":
		return "aarch64"
	case "arm":
		return "armhf"
	default:
		return ""
	}
}

// getLatestAppImageURL queries GitHub API to find the download URL for the latest AppImage
func getLatestAppImageURL(owner, repo, releaseName, arch string) (string, error) {
	client := github.NewClient(nil)

	var release *github.RepositoryRelease
	var err error

	if releaseName == "latest" {
		release, _, err = client.Repositories.GetLatestRelease(context.Background(), owner, repo)
	} else {
		release, _, err = client.Repositories.GetReleaseByTag(context.Background(), owner, repo, releaseName)
	}

	if err != nil {
		return "", fmt.Errorf("failed to get release: %v", err)
	}

	// Look for the appimaged AppImage for the right architecture
	expectedSuffix := "-" + arch + ".AppImage"
	for _, asset := range release.Assets {
		name := asset.GetName()
		if strings.HasPrefix(name, "appimaged-") && strings.HasSuffix(name, expectedSuffix) && !strings.HasSuffix(name, ".zsync") {
			return asset.GetBrowserDownloadURL(), nil
		}
	}

	return "", fmt.Errorf("no matching AppImage found for architecture %s", arch)
}

// downloadFile downloads a file from the given URL to the local path
func downloadFile(filepath string, url string) error {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// restartDaemon restarts the appimaged daemon
// This is called after a successful self-update
func restartDaemon() error {
	currentPath := thisai.Path

	// Start the new version
	cmd := exec.Command(currentPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start new daemon: %v", err)
	}

	log.Println("New daemon started, exiting current process...")

	// Exit the current process
	os.Exit(0)
	return nil
}

// performSelfUpdateIfAvailable checks if an update is available and performs it if autoupdate is enabled
func performSelfUpdateIfAvailable() {
	if !*autoupdatePtr {
		log.Println("Self-update available but autoupdate is disabled. Run with --autoupdate to enable.")
		return
	}

	log.Println("Performing self-update...")
	err := selfUpdate()
	if err != nil {
		helpers.PrintError("selfUpdate", err)
		sendDesktopNotification("Update failed", fmt.Sprintf("Failed to update appimaged:\n%v", err), 10000)
	}
}
