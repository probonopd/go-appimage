# appimaged

This is an experimental implementation of the optional AppImage daemon, `appimaged`, in Go, mainly to see what is possible.

## Installation and usage

Assuming you are using a 64-bit Intel machine (arm64, also known as x86_64), you can use our pre-compiled binaries:

To try it out:

```
# Remove pre-existing similar tools
systemctl --user stop appimaged.service || true
systemctl --user stop appimagelauncherd.service || true
systemctl --user stop appimagelauncherfs.service || true
sudo apt-get -y remove appimagelauncher || true

# Clear caches
rm "$HOME"/.thumbnails/normal/*
rm "$HOME"/.thumbnails/large/*
rm "$HOME"/.local/share/applications/appimage*

# Get external tools. Eventually we want to replace those with native Go implementations
wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs
wget -c https://github.com/probonopd/appimage/releases/download/continuous/appimaged-amd64
chmod +x appimaged-* unsquashfs bsdtar

# Launch
./appimaged-*
```

https://github.com/probonopd/appimage/releases/tag/continuous also has builds for 32-bit Intel, 32-bit ARM (e.g., Raspberry Pi), and 64-bit ARM.

## Building

If for whatever reason you would like to build from source:

```
sudo apt-get -y install gcc 
go get -v github.com/probonopd/appimage/src/appimaged || true
if [ -z $GOPATH ] ; then export GOPATH=$HOME/go ; fi
rm -rf $GOPATH/src/github.com/purpleidea/mgmt/vendor/gopkg.in/fsnotify.v1/
go get github.com/probonopd/appimage/src/appimaged 
go build -trimpath -ldflags="-s -w" github.com/probonopd/appimage/src/appimaged
```
