package helpers

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/google/go-github/github" // with go modules disabled
)

func GetCommitMessageForLatestCommit(ui UpdateInformation) (string, error) {

	if ui.transportmechanism == "gh-releases-zsync" {

		client := github.NewClient(nil)
		var TRAVIS_COMMIT string
		release, _, err := client.Repositories.GetReleaseByTag(context.Background(), ui.username, ui.repository, ui.releasename)
		if err == nil {
			// log.Println("github", release.GetHTMLURL())
			// log.Println("github", release.GetBody())
			// log.Println("github", release.GetAssetsURL())
			// log.Println("github", release.GetTagName())         // E.g., "continuous"
			// log.Println("github", release.GetTargetCommitish()) // E.g., "a4039871c082489b4ac5c3b0ab98d3617c408e53"
			TRAVIS_COMMIT = release.GetTargetCommitish()
		} else {
			return "", err
		}

		commit, _, err := client.Git.GetCommit(context.Background(), ui.username, ui.repository, TRAVIS_COMMIT)
		if err == nil {
			return commit.GetMessage(), err
		} else {
			return "", err
		}

	} else {

		return "", errors.New("Not yet implemented for this transport mechanism")
	}
}

// Uses the TRAVIS_COMMIT environment variable
func GetCommitMessageForThisCommitOnTravis() (string, error) {

	client := github.NewClient(nil)

	TRAVIS_COMMIT := os.Getenv("TRAVIS_COMMIT")
	if TRAVIS_COMMIT == "" {
		return "", errors.New("TRAVIS_COMMIT environment variable missing. Not running on Travis CI?")
	}

	repo_slug := os.Getenv("TRAVIS_REPO_SLUG")
	if repo_slug == "" {
		return "", errors.New("TRAVIS_REPO_SLUG environment variable missing. Not running on Travis CI?")
	}

	parts := strings.Split(repo_slug, "/")
	if len(parts) < 2 {
		return "", errors.New("Cannot split repo_slug")
	}

	commit, _, err := client.Git.GetCommit(context.Background(), parts[0], parts[1], TRAVIS_COMMIT)
	if err == nil {
		return commit.GetMessage(), err
	} else {
		return "", err
	}

}
