# appimaged

This is an experimental implementation of the optional AppImage daemon, `appimaged`, in Go, mainly to see what is possible.

## Installation and usage

Assuming you are using a 64-bit Intel machine (arm64, also known as x86_64), you can use our pre-compiled binaries. To try it out:

```
# Remove pre-existing similar tools
systemctl --user stop appimaged.service || true
sudo apt-get -y remove appimagelauncher || true

# Clear caches
rm "$HOME"/.thumbnails/normal/*
rm "$HOME"/.thumbnails/large/*
rm "$HOME"/.local/share/applications/appimage*

# Launch
./appimaged-*.AppImage
```

https://github.com/probonopd/appimage/releases/tag/continuous has builds for 32-bit Intel, 32-bit ARM (e.g., Raspberry Pi), and 64-bit ARM.

## Features

Implemented

* Registers type-1 and type-2 AppImages
* Detects mounted and unmounted partitions by watching DBus
* Significantly lower CPU and memory usage than other implementations
* Error notifications in case applications cannot be launched for whatever reason
* If Firejail is on the $PATH, various options for running applications sandboxed via the context menu
* If AppImageUpdate is on the $PATH, updating applications via the context menu
* Opening the containing folder via the context menu
* Extracting AppImages via the context menu
* Announces itself on the local network using Zeroconf (more to come)

Envisioned

* Seamless P2P distribution using IPFS
* Real-time notification based on PubSub when updates are available
* ...

## Building

If for whatever reason you would like to build from source:

```
sudo apt-get -y install gcc 
if [ -z $GOPATH ] ; then export GOPATH=$HOME/go ; fi
go get github.com/probonopd/appimage/src/appimaged 
go build -trimpath -ldflags="-s -w" github.com/probonopd/appimage/src/appimaged
```
