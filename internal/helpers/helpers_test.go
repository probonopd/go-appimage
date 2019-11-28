package helpers_test

import (
	"testing"

	"github.com/probonopd/go-appimage/internal/helpers"
)

var err error

func TestValidateUpdateInformation(t *testing.T) {

	// Ensure that bad updateinformation throws an error
	bads := []string{
		"foo",
		"https://foo.bar",
		"hhttps://foo.bar",
		"zsync|foo",
		"zsync|https://foo.bar",
		"zsync|hhttps://foo.bar",
		"gh-releases-zsync|user|project|latest|App*-x86_64.AppImage",         // Need zsync, not AppImage
		"gh-releases-zsync|user|project|latest|App*-x86_64.AppImage?foo=bar", // Need zsync, not AppImage
	}

	for _, bad := range bads {
		err = helpers.ValidateUpdateInformation(bad)
		if err == nil {
			t.Errorf("Despite corrupt updateinformation it was deemed correct: " + bad)
		}
	}

	// Ensure that good updateinformation does not thros an error

	goods := []string{
		"gh-releases-zsync|user|project|latest|App*-x86_64.AppImage.zsync",
		"gh-releases-zsync|user|project|latest|App*-x86_64.AppImage.zsync?foo=bar", // This is allowed and needed
	}

	for _, good := range goods {
		err = helpers.ValidateUpdateInformation(good)
		if err != nil {
			t.Errorf("Despite correct updateinformation it was deemed corrupt: " + good)
		}
	}

}
