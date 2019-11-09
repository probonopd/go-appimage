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

// VerifyUpdateInformation verifies whether updateinformation is corerct.
// This is currently a stub. TODO: Implement more checks.
// Returns error.
// TODO: Eventually use this in AppImageHub, too
func VerifyUpdateInformation(updateinformation string) error {
	parts := strings.Split(updateinformation, "|")
	if len(parts) < 2 {
		return errors.New("Too short")
	}
	// Check for allowed transport mechanisms,
	// https://github.com/AppImage/AppImageSpec/blob/master/draft.md#update-information
	transport_mechanisms := []string{"zsync", "bintray-zsync", "gh-releases-zsync"}
	detected_tm := ""
	for _, tm := range transport_mechanisms {
		if parts[0] != tm {
			detected_tm = tm
		}
	}
	if detected_tm == "" {
		return errors.New("Invalid transport mechanism")
	}

	// Currently updateinformation needs to end in "zsync" for all transport mechanisms,
	// although this might change in the future
	// Note that it is allowable to have somehting like "some.zsync?foo=bar", which is why we parse it as an URL
	u, err := url.Parse(parts[len(parts)-1])
	if err != nil {
		return errors.New("Cannot parse URL")
	}
	if detected_tm == "zsync" && u.Scheme == "" { // FIXME: This apparently never triggers, why?
		return errors.New("Scheme is missing, zsync needs e.,g,. http:// or https://")
	}
	if strings.HasSuffix(u.Path, ".zsync") == false {
		return errors.New("Does not end in .zsync")
	}

	return nil
}
