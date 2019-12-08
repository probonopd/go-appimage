# appimagetool

This is an experimental implementation of the AppImage command line tool, `appimagetool`, in Go, mainly to see what is possible.

## Installation and usage

Assuming you are using a 64-bit Intel machine (arm64, also known as x86_64), you can use our pre-compiled binaries. To try it out:

```
wget -c https://github.com/$(wget -q https://github.com/probonopd/go-appimage/releases -O - | grep "appimagetool-.*-x86_64.AppImage" | head -n 1 | cut -d '"' -f 2)
chmod +x appimagetool-*.AppImage
VERSION=1.0 ./appimagetool-*.AppImage ./Some.AppDir
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

Envisioned

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
