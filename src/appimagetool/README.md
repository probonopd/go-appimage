# appimagetool

This is an experimental implementation of the AppImage command line tool, `appimagetool`, in Go, mainly to see what is possible. It can also do, using the `deploy` verb, what [linuxdeployqt](https://github.com/probonopd/linuxdeployqt) does.

## Installation and usage

Assuming you are using a 64-bit Intel machine (amd64, also known as x86_64), you can use our pre-compiled binaries. To try it out:

```bash
wget -c https://github.com/$(wget -q https://github.com/probonopd/go-appimage/releases/expanded_assets/continuous -O - | grep "appimagetool-.*-x86_64.AppImage" | head -n 1 | cut -d '"' -f 2)
chmod +x appimagetool-*.AppImage
./appimagetool-*.AppImage -s deploy ./AppDir/usr/share/applications/*.desktop # Bundle EVERYTHING
# or 
./appimagetool-*.AppImage deploy ./AppDir/usr/share/applications/*.desktop # Bundle everything expect what comes with the base system
# and
VERSION=1.0 ./appimagetool-*.AppImage ./AppDir # turn AppDir into AppImage
```

<https://github.com/probonopd/go-appimage/releases/tag/continuous> has builds for 32-bit Intel, 32-bit ARM (e.g., Raspberry Pi), and 64-bit ARM.

### Recognized env vars

- `QTDIR`: root directory for the Qt installation to copy shared libraries from, e.g. `/usr/lib/qt6/`

## Update Information (for CI/CD)

When `appimagetool` runs on GitHub Actions, it automatically:

1. **Embeds [UpdateInformation](https://github.com/AppImage/AppImageSpec/blob/master/draft.md#update-information)** into the AppImage
2. **Generates a `.zsync` file** alongside the AppImage for efficient delta updates
3. **Uploads both files** to the GitHub Release

The UpdateInformation format is:
```
gh-releases-zsync|<owner>|<repo>|<release>|<AppName>-*-<arch>.AppImage.zsync
```

This is detected automatically using `GITHUB_REPOSITORY` and `GITHUB_REF` environment variables.

The release channel is determined as:
- `continuous` - for builds from the master branch
- `latest` - for tagged releases (non-continuous)

## Features

Implemented

* Creates AppImage
* If running on GitHub Actions, determines updateinformation, embeds updateinformation, signs, and writes zsync file
* Simplified signing
* Automatic upload to GitHub Releases
* Prepare self-contained AppDirs using the `deploy` verb
* Bundle GStreamer
* Bundle Qt
* Bundle Qml
* Obey excludelist (unless invoked in self-contained a.k.a. "bundle everything" mode)

Envisioned

* Bundle QtWebEngine (untested)
* Bundle Python
* GitLab support
* OBS support
* ...

## Building

If for whatever reason you would like to build from source:

```bash
scripts/build.sh appimagetool
```
