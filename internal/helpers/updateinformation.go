package helpers

import (
	"errors"
	"net/url"
	"strings"
)

// updateinformation started out as a string that tells AppImageUpdate where to grab updates from.
// Turns out that it can also be used to identify a set of AppImages that belong together
// among which it makes sense to compare version numbers. Because it identifies the author,
// "channel" (e.g., continuous,...)
// Hence we are using it as the main identifier for AppImages now, similar
// to how the Play Store uses strings like "com.spotify.music" to identify apps.

// For example, if the system wants to update an application,
// we search for the newest AppImage that has update information of the updater.
// This way, no matter how many versions of the updater are on the system,
// we are using the most recent one.
// This is kinda replicating Launch Services behavior using XDG standards.

// VerifyUpdateInformation verifies whether updateinformation is corerct.
// This is currently a stub. TODO: Implement more checks.
// Returns error.
// TODO: Eventually use this in AppImageHub, too

// Please note that pre-releases are not being considered when using "latest".
// You will have to explicitly provide the name of a release.
// When using e.g., uploadtool, the name of the release created will
// always be "continuous",
// hence, you can just specify that value instead of "latest".
type UpdateInformation struct {
	transportmechanism string
	fileurl            string
	username           string
	repository         string
	releasename        string // latest will automatically use the latest release as determined by the GitHub API
	filename           string // filename of the zsync file on GitHub, * is a wildcard
	packagename        string
}

// NewUpdateInformationFromString returns an UpdateInformation struct
// for the given updateinformation string, and err
// TODO: Use UpdateInformation structs throughout the codebase
func NewUpdateInformationFromString(updateinformation string) (UpdateInformation, error) {

	ui := UpdateInformation{}

	parts := strings.Split(updateinformation, "|")

	err := ValidateUpdateInformation(updateinformation)
	if err != nil {
		return ui, err
	}

	ui.transportmechanism = parts[0]
	if ui.transportmechanism == "zsync" {
		if len(parts) < 2 {
			return ui, errors.New("Too short")
		}
		ui.fileurl = parts[1]
	} else if ui.transportmechanism == "gh-releases-zsync" {
		if len(parts) < 5 {
			return ui, errors.New("Too short")
		}
		ui.username = parts[1]
		ui.repository = parts[2]
		ui.releasename = parts[3]
		ui.filename = parts[4]
	} else if ui.transportmechanism == "bintray-zsync" {
		if len(parts) < 5 {
			return ui, errors.New("Too short")
		}
		ui.username = parts[1]
		ui.repository = parts[2]
		ui.packagename = parts[3]
		ui.filename = parts[4] // a.k.a. "zsync path"
	} else {
		return ui, errors.New("This transport mechanism is not yet implemented")
	}
	return ui, nil
}

// ValidateUpdateInformation validates an updateinformation string,
// returns error.
// TODO: Build this into NewUpdateInformationFromString and get rid of it?
func ValidateUpdateInformation(updateinformation string) error {
	parts := strings.Split(updateinformation, "|")
	if len(parts) < 2 {
		return errors.New("Too short")
	}
	// Check for allowed transport mechanisms,
	// https://github.com/AppImage/AppImageSpec/blob/master/draft.md#update-information
	transportMechanisms := []string{"zsync", "bintray-zsync", "gh-releases-zsync"}
	detectedTm := ""
	for _, tm := range transportMechanisms {
		if parts[0] != tm {
			detectedTm = tm
		}
	}
	if detectedTm == "" {
		return errors.New("Invalid transport mechanism")
	}

	// Currently updateinformation needs to end in "zsync" for all transport mechanisms,
	// although this might change in the future
	// Note that it is allowable to have something like "some.zsync?foo=bar", which is why we parse it as an URL
	u, err := url.Parse(parts[len(parts)-1])
	if err != nil {
		return errors.New("Cannot parse URL")
	}
	if detectedTm == "zsync" && u.Scheme == "" { // FIXME: This apparently never triggers, why?
		return errors.New("Scheme is missing, zsync needs e.,g,. http:// or https://")
	}
	if strings.HasSuffix(u.Path, ".zsync") == false {
		return errors.New(updateinformation + " does not end in .zsync")
	}

	return nil
}

func getChangelogHeadlineForUpdateInformation(updateinformation string) string {
	return ""
}
