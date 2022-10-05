# appimagetool

This is an experimental implementation of the AppImage command line tool, `appimagetool`, in Go, mainly to see what is possible. It can also do, using the `deploy` verb, what [linuxdeployqt](https://github.com/probonopd/linuxdeployqt) does.

## Installation and usage

Assuming you are using a 64-bit Intel machine (amd64, also known as x86_64), you can use our pre-compiled binaries. To try it out:

```
wget $(curl https://api.github.com/repos/probonopd/go-appimage/releases | jq -r '.[] | select(.tag_name == "continuous") | .assets[] | select((.name | endswith("x86_64.AppImage")) and (.name | contains("appimagetool"))) | .browser_download_url') -O appimagetool
chmod +x appimagetool-*.AppImage
./appimagetool-*.AppImage -s deploy appdir/usr/share/applications/*.desktop # Bundle EVERYTHING
# or 
./appimagetool-*.AppImage deploy appdir/usr/share/applications/*.desktop # Bundle everything expect what comes with the base system
# and
VERSION=1.0 ./appimagetool-*.AppImage ./Some.AppDir # turn AppDir into AppImage
```

https://github.com/probonopd/go-appimage/releases/tag/continuous has builds for 32-bit Intel, 32-bit ARM (e.g., Raspberry Pi), and 64-bit ARM.

## Features

Implemented

* Creates AppImage
* If running on GitHub, determines updateinformation, embeds updateinformation, signs, and writes zsync file
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

```
sudo apt-get -y install gcc 
if [ -z $GOPATH ] ; then export GOPATH=$HOME/go ; fi
go get github.com/probonopd/go-appimage/src/appimagetool 
go build -trimpath -ldflags="-s -w" github.com/probonopd/go-appimage/src/appimagetool
```
