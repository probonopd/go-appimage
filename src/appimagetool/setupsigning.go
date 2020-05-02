// Encrypting and uploading a private key
// for signing AppImages with Travis CI
// without needing the travis command line tool
// Based on https://gist.github.com/kzap/5819745 and https://docs.travis-ci.com/user/encrypting-files/
//
// TODO: Use https://github.com/src-d/go-git or https://github.com/google/go-github to
// * Get info about the repo this is supposed to run in
// * Commit the encrypted file
// * even git push?

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"

	"strings"
	"time"

	"github.com/probonopd/go-appimage/internal/helpers"
	"github.com/shuheiktgw/go-travis"
	"gopkg.in/src-d/go-git.v4"
)

var overwritePtr = flag.Bool("o", false, "Overwrite pre-existing files")

func setupSigning() {

	// Parse command line arguments
	flag.Parse()

	// Check if we are on a clean git repository. Exit as fast as possible if we are not.
	var gitRepo *git.Repository

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("os.Getwd:", err)
		os.Exit(1)
	}

	gitRepo, err = git.PlainOpenWithOptions(cwd, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		fmt.Println("git:", err)
	}

	if gitRepo == nil {
		fmt.Println("Could not open git repository at", cwd+". \nPlease execute this command from within a clean git repository.")
		os.Exit(1)
	}

	gitWorktree, _ := gitRepo.Worktree()
	s, _ := gitWorktree.Status()
	clean := s.IsClean()

	if clean == false {
		fmt.Println("Repository is not clean. Please commit or stash any changes first.")
		os.Exit(1)

	}

	// Check for needed files on $PATH
	tools := []string{"sh", "git", "openssl"}
	err = helpers.CheckForNeededTools(tools)
	if err != nil {
		os.Exit(1)
	}

	// Exit if the repo already contains the files we are about to add
	exitIfFileExists(helpers.PubkeyFileName, "Public key")
	exitIfFileExists(helpers.PrivkeyFileName, "Private key")
	exitIfFileExists(helpers.EncPrivkeyFileName, "Encrypted private key")

	// Get repo_slug.
	gitRemote, err := gitRepo.Remote("origin")
	if err != nil {
		fmt.Println("Could not get git remote")
		os.Exit(1)
	}
	components := strings.Split(gitRemote.Config().URLs[0], "/")
	repoSlug := components[len(components)-2] + "/" + components[len(components)-1]
	// fmt.Println(repo_slug)

	// Ask user for GitHub token
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("A GitHub token is needed. Get it from")
	fmt.Println("https://github.com/settings/tokens")
	fmt.Println("The GitHub token is used to store the decryption password")
	fmt.Println("for the private signing key as private variable on Travis CI")
	fmt.Print("Enter GitHub token: ")
	text, _ := reader.ReadString('\n')
	token := strings.TrimSpace(text)
	// fmt.Println(token)

	fmt.Println("Repositories can be on travis.com or on travis.org")
	fmt.Println("Is your repository on travis.com? Answer 'no' if org (yes/no)")
	var client *travis.Client
	var travisSettingsURL string
	if AskForConfirmation() == true {
		fmt.Println("Assuming your repository is on travis.com")
		client = travis.NewClient(travis.ApiComUrl, token)
		travisSettingsURL = "https://travis-ci.com/" + repoSlug + "/settings"
	} else {
		fmt.Println("Assuming your repository is on travis.org")
		client = travis.NewClient(travis.ApiOrgUrl, token)
		travisSettingsURL = "https://travis-ci.org/" + repoSlug + "/settings"
	}

	// Read existing environment variables on Travis CI
	esList, resp, err := client.EnvVars.ListByRepoSlug(context.Background(), repoSlug)
	if err != nil {
		fmt.Println("client.EnvVars.ListByRepoSlug:", err)
		os.Exit(1)
	}
	if resp.StatusCode < 200 || 300 <= resp.StatusCode {
		fmt.Printf("Could not read existing environment variables on Travis CI for repo %s: invalid http status: %s", repoSlug, resp.Status)
	}

	var existingVars []string
	for _, e := range esList {
		// fmt.Println("* Name:", *e.Name, "Public:", *e.Public, "Id:", *e.Id)
		existingVars = append(existingVars, *e.Name)
	}

	if Contains(existingVars, helpers.EnvSuperSecret) == true {
		fmt.Println("Environment variable", helpers.EnvSuperSecret, "already exists on Travis CI")
		fmt.Println("You can check it on", travisSettingsURL)
		fmt.Println("It looks like this repository is already set up for signing. Exiting")
		os.Exit(1)
	}

	// GPG
	// Unattended GPG key generation
	// We could have used something like
	// co = exec.Command("ssh-keygen", "-f", "id_rsa", "-t", "rsa", "-f", "./privkey", "-P", "")
	// ssh-keygen -f id_rsa -t rsa -b 4096 -f ./privkey -P ""
	// But it gets messy real quick (see that empty argument?), hence we use native Go instead.

	fmt.Println("Generating key pair...")
	helpers.CreateAndValidateKeyPair()

	// Check if we succeeded until here
	if _, err := os.Stat(helpers.PrivkeyFileName); err != nil {
		fmt.Println("Could not create private key, exiting")
		os.Exit(1)
	}

	// Check if password/secret already exists, delete it if -o was specified, exit otherwise
	if _, err := os.Stat("secret"); err == nil {
		if *overwritePtr == false {
			fmt.Println("Secret already exists, exiting")
			os.Exit(1)
		} else {
			err := os.Remove("secret")
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}

	// Generate the password which we will store as a
	// private environment variable on Travis CI
	// and will use to encrypt the private key
	superSecretPassword := generatePassword()
	f, err := os.Create("secret")
	if err != nil {
		fmt.Println("Could not open file for writing secret, exiting")
		os.Exit(1)
	}
	defer f.Close()
	_, err = f.WriteString(superSecretPassword)
	if err != nil {
		fmt.Println("Could not write secret, exiting")
		os.Exit(1)
	}

	// Encrypt the private key using the password
	// TODO: Replace with native Go code in ossl.go
	cmd := "openssl aes-256-cbc -pass file:./secret -in ./" + helpers.PrivkeyFileName + " -out ./" + helpers.EncPrivkeyFileName + " -a"
	err = helpers.RunCmdStringTransparently(cmd)
	if err != nil {
		fmt.Println("Could not encrypt the private key using the password, exiting")
		os.Exit(1)
	}

	// Check if we succeeded until here
	if _, err := os.Stat(helpers.EncPrivkeyFileName); err != nil {
		fmt.Println("Could not encrypt private key, exiting")
		os.Exit(1)
	}

	// Delete unneeded public key
	/*
		err = os.Remove("privkey.pub")
		if err != nil {
			fmt.Println("Could not delete privkey.pub, exiting")
			os.Exit(1)
		}
	*/

	// Delete unencrypted private key
	os.Remove("secret") // Seemingly this gives an error even if it deleted the file...

	// Delete unencrypted private key
	os.Remove(helpers.PrivkeyFileName) // Seemingly this gives an error even if it deleted the file...

	SetTravisEnv(client, repoSlug, existingVars, "GITHUB_TOKEN", token, travisSettingsURL)
	SetTravisEnv(client, repoSlug, existingVars, helpers.EnvSuperSecret, superSecretPassword, travisSettingsURL)
	// SetTravisEnv(client, repo_slug, existingVars, "FOO", "BAR")

	_, err = gitWorktree.Add(helpers.EncPrivkeyFileName)
	if err != nil {
		fmt.Println("Could not add encrypted private key to git repository")
		os.Exit(1)
	}

	_, err = gitWorktree.Add(helpers.PubkeyFileName)
	if err != nil {
		fmt.Println("Could not add public key to git repository")
		os.Exit(1)
	}

	// TODO: Can we automate this?
	fmt.Println("")
	fmt.Println("Your super secret password is:")
	fmt.Println(superSecretPassword)
	fmt.Println("Store it in a safe location, along with your encrypted private key in " + helpers.EncPrivkeyFileName + ".")
	fmt.Println("")
	fmt.Println("You can decrypt it with:")
	fmt.Println("openssl aes-256-cbc -pass \"pass:" + superSecretPassword + "\" -in " + helpers.EncPrivkeyFileName + " -out " + helpers.PrivkeyFileName + " -d -a")
	fmt.Println("")
	/*	Actually let appimagetool do this... no need for this in .travis.yml
		fmt.Println("Please add to your .travis.yml file:")
		fmt.Println("----------------------------------------------------------------------------------------------")
		fmt.Println("before_install:")
		fmt.Println("  - openssl aes-256-cbc -pass \"pass:$" + helpers.EnvSuperSecret + "\" -in " + helpers.EncPrivkeyFileName + " -out " + helpers.PrivkeyFileName + " -d -a")
		fmt.Println("----------------------------------------------------------------------------------------------")
	*/
	fmt.Println("Then, run 'git commit' and 'git push'")
}

// SetTravisEnv sets a private variable on Travis CI
func SetTravisEnv(client *travis.Client, repoSlug string, existingVars []string, name string, value string, travisSettingsURL string) {
	body := travis.EnvVarBody{Name: name, Value: value, Public: false}
	if Contains(existingVars, name) == false {
		fmt.Println("Set environment variable", name, "on Travis CI...")
		_, resp, err := client.EnvVars.CreateByRepoSlug(context.Background(), repoSlug, &body)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(resp.Status)
		}
	} else {
		fmt.Println("Environment variable", name, "already exists on Travis CI, not changing it")
	}
	fmt.Println("You can edit it on", travisSettingsURL)
}

// Contains answers whether a []string contains a string
func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// exitIfFileExists checks if file already exists, deletes it if -o was specified, exit otherwise
func exitIfFileExists(file string, description string) {
	if _, err := os.Stat(file); err == nil {
		if *overwritePtr == false {
			fmt.Println(description, "'"+file+"'", "already exists, exiting")
			os.Exit(1)
		} else {
			err := os.Remove(file)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}
}

//////////////////////////////// start AskForConfirmation

// AskForConfirmation uses Scanln to parse user input. A user must type in "yes" or "no" and
// then press enter. It has fuzzy matching, so "y", "Y", "yes", "YES", and "Yes" all count as
// confirmations. If the input is not recognized, it will ask again. The function does not return
// until it gets a valid response from the user. Typically, you should use fmt to print out a question
// before calling askForConfirmation. E.g. fmt.Println("WARNING: Are you sure? (yes/no)")
// From https://gist.github.com/albrow/5882501#file-confirm-go-L9
func AskForConfirmation() bool {
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		fmt.Println(err)
	}
	okayResponses := []string{"y", "Y", "yes", "Yes", "YES"}
	nokayResponses := []string{"n", "N", "no", "No", "NO"}
	if containsString(okayResponses, response) {
		return true
	} else if containsString(nokayResponses, response) {
		return false
	} else {
		fmt.Println("Please type yes or no and then press enter:")
		return AskForConfirmation()
	}
}

// posString returns the first index of element in slice.
// If slice does not contain element, returns -1.
func posString(slice []string, element string) int {
	for index, elem := range slice {
		if elem == element {
			return index
		}
	}
	return -1
}

// containsString returns true iff slice contains element that ends with the given string
func containsString(slice []string, element string) bool {

	for _, item := range slice {
		if strings.HasSuffix(item, element) == true {
			return true
		}
	}

	return false
}

//////////////////////////////// end AskForConfirmation

// generatePassword generates a random password consisting
// consisting of letters, numbers, and selected special characters
// https://stackoverflow.com/a/22892986
func generatePassword() string {
	rand.Seed(time.Now().UnixNano())
	var letters = []rune("1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, 24)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
