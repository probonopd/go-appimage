# mkappimage

This is a low-level tool that converts an AppDir into an AppImage. This is provided mainly for other higher-level tools that need this functionality. It alone will not produce usable AppImages. __If you would like to make AppImages, you should NOT be using this tool__ unless you know exactly what you are doing and need a low-level tool.

## Installation and usage

Assuming you are using a 64-bit Intel machine (amd64, also known as x86_64), you can use our pre-compiled binaries. To try it out:

```
wget -c https://github.com/$(wget -q https://github.com/probonopd/go-appimage/releases/expanded_assets/continuous -O - | grep "mkappimage-.*-x86_64.AppImage" | head -n 1 | cut -d '"' -f 2)
chmod +x mkappimage-*.AppImage
VERSION=1.0 ./mkappimage-*.AppImage ./Some.AppDir # turn AppDir into AppImage
```

https://github.com/probonopd/go-appimage/releases/tag/continuous has builds for 32-bit Intel, 32-bit ARM (e.g., Raspberry Pi), and 64-bit ARM.


## Building

If for whatever reason you would like to build from source:

```
sudo apt-get -y install gcc 
if [ -z $GOPATH ] ; then export GOPATH=$HOME/go ; fi
go get github.com/probonopd/go-appimage/src/mkappimage 
go build -trimpath -ldflags="-s -w" github.com/probonopd/go-appimage/src/mkappimage
```
