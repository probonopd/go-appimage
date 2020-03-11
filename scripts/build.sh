#!/bin/bash

set -e
set -x

# Build and upload the contents of this repository.
#
# NOTE: Please contact the author before trying to convert this script
# into any other language. It is intentionally a bash script rather than
# a Makefile, CMake file etc.
#
# This script is tested only on Ubuntu and Travis CI; adding further
# complexity is not desired.
#
# Changes that reduce the number of LOC, TODO, and FIXME are welcome.

#############################################################
# Get Go and other dependencies
##############################################################

# Disregard any other Go environment that may be on the system (e.g., on Travis CI)
unset GOARCH GOBIN GOEXE GOHOSTARCH GOHOSTOS GOOS GORACE GOROOT GOTOOLDIR CC GOGCCFLAGS CGO_ENABLED
export GOPATH=$HOME/go

# Export version and build number
if [ ! -z "$TRAVIS_BUILD_NUMBER" ] ; then
  export COMMIT="${TRAVIS_BUILD_NUMBER}" # "${TRAVIS_JOB_WEB_URL} on $(date +'%Y-%m-%d_%T')"
  export VERSION=$TRAVIS_BUILD_NUMBER
else
  export COMMIT=$(date '+%Y-%m-%d_%T')
  export VERSION=$(date '+%Y-%m-%d_%T')
fi

# Get pinned version of Go directly from upstream
if [ "aarch64" == "$TRAVIS_ARCH" ] ; then export ARCH=arm64 ; fi
if [ "amd64" == "$TRAVIS_ARCH" ] ; then export ARCH=amd64 ; fi
wget -c -nv https://dl.google.com/go/go1.13.4.linux-$ARCH.tar.gz
sudo tar -C /usr/local -xzf go*.tar.gz
export PATH=/usr/local/go/bin:$PATH

# Get dependencies needed for CGo # FIXME: Get rid of the need for CGo and, in return, those dependencies
sudo apt-get -q update
if [ $(go env GOHOSTARCH) == "amd64" ] ; then sudo apt-get -y install gcc-multilib autoconf ; fi
if [ $(go env GOHOSTARCH) == "arm64" ] ; then sudo apt-get -y install gcc-arm-linux-gnueabi autoconf ; fi

##############################################################
# Build appimagetool
##############################################################

# 64-bit
go get -v github.com/probonopd/go-appimage/src/appimagetool
go build -trimpath -ldflags="-s -w -X main.commit=$COMMIT" github.com/probonopd/go-appimage/src/appimagetool
mv ./appimagetool appimagetool-$(go env GOHOSTARCH)

# 32-bit
if [ $(go env GOHOSTARCH) == "amd64" ] ; then 
  env CGO_ENABLED=1 GOOS=linux GOARCH=386 go build -trimpath -ldflags="-s -w -X main.commit=$COMMIT" github.com/probonopd/go-appimage/src/appimagetool
  mv ./appimagetool appimagetool-386
elif [ $(go env GOHOSTARCH) == "arm64" ] ; then 
  env CC=arm-linux-gnueabi-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=5 go build -trimpath -ldflags="-s -w -X main.commit=$COMMIT" github.com/probonopd/go-appimage/src/appimagetool
  mv ./appimagetool appimagetool-arm
fi

##############################################################
# Bild appimaged
##############################################################

# FIXME: Workaround for:
# go/src/github.com/go-language-server/uri/uri.go:15:2: cannot find package "golang.org/x/xerrors" in any of:
# 	/usr/local/go/src/golang.org/x/xerrors (from $GOROOT)
# 	/home/me/go/src/golang.org/x/xerrors (from $GOPATH)
go get -v golang.org/x/xerrors

# 64-bit
go get -v github.com/probonopd/go-appimage/src/appimaged || true # FIXME: Why does this comand return a non-0 exit status?
( cd $GOPATH/src/github.com/srwiley/oksvg ; git checkout gradfix ) # FIXME: This is probably not the way to do it
( cd $GOPATH/src/github.com/go-language-server/uri ; git checkout a822a9b ) # FIXME: Workaround for: appimage.go:23:2: code in directory /home/me/go/src/github.com/go-language-server/uri expects import "go.lsp.dev/uri"
go build -trimpath -ldflags="-s -w -X main.commit=$COMMIT" github.com/probonopd/go-appimage/src/appimaged
mv ./appimaged appimaged-$(go env GOHOSTARCH)

# 23-bit
if [ $(go env GOHOSTARCH) == "amd64" ] ; then 
  env CGO_ENABLED=1 GOOS=linux GOARCH=386 go build -trimpath -ldflags="-s -w -X main.commit=$COMMIT" github.com/probonopd/go-appimage/src/appimaged
  mv ./appimaged appimaged-386
elif [ $(go env GOHOSTARCH) == "arm64" ] ; then
  env CC=arm-linux-gnueabi-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=5 go build -trimpath -ldflags="-s -w -X main.commit=$COMMIT" github.com/probonopd/go-appimage/src/appimaged
  mv ./appimaged appimaged-arm
fi

##############################################################
# Eat our own dogfood, use appimagetool to make 
# and upload AppImages
# TODO: Do this for ARM as well
##############################################################

unset ARCH # It contains "amd64" which we cannot use since we need "x86_64"

if [ $(go env GOHOSTARCH) != "amd64" ] ; then
  exit 0
fi

# Make appimagetool AppImage
rm -rf appimagetool.AppDir || true
mkdir -p appimagetool.AppDir/usr/bin
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/desktop-file-validate )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/mksquashfs )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-x86_64 && ln -s runtime-x86_64 runtime-amd64 )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/uploadtool/raw/master/upload.sh -O uploadtool )
wget https://github.com/NixOS/patchelf/archive/0.9.tar.gz # 0.10 cripples files, hence use 0.9
tar xf 0.9.tar.gz 
cd patchelf-*/
./bootstrap.sh
./configure --prefix=/usr
make -j$(nproc) LDFLAGS=-static
strip src/patchelf
cd -
mkdir -p appimagetool.AppDir/usr/bin
cp patchelf-*/src/patchelf appimagetool.AppDir/usr/bin/
chmod +x appimagetool.AppDir/usr/bin/*
cp appimagetool-amd64 appimagetool.AppDir/usr/bin/appimagetool
( cd appimagetool.AppDir/ ; ln -s usr/bin/appimagetool AppRun)
cp $GOPATH/src/github.com/probonopd/go-appimage/data/appimage.png appimagetool.AppDir/
cat > appimagetool.AppDir/appimagetool.desktop <<\EOF
[Desktop Entry]
Type=Application
Name=appimagetool
Exec=appimagetool
Comment=Tool to generate AppImages from AppDirs
Icon=appimage
Categories=Development;
Terminal=true
EOF
PATH=./appimagetool.AppDir/usr/bin/:$PATH appimagetool ./appimagetool.AppDir

# Make appimaged AppImage
rm -rf appimaged.AppDir || true
mkdir -p appimaged.AppDir/usr/bin
( cd appimaged.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar )
( cd appimaged.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs )
chmod +x appimaged.AppDir/usr/bin/*
cp appimaged-amd64 appimaged.AppDir/usr/bin/appimaged
( cd appimaged.AppDir/ ; ln -s usr/bin/appimaged AppRun)
cp $GOPATH/src/github.com/probonopd/go-appimage/data/appimage.png appimaged.AppDir/
cat > appimaged.AppDir/appimaged.desktop <<\EOF
[Desktop Entry]
Type=Application
Name=appimaged
Exec=appimaged
Comment=Optional daemon that integrates AppImages into the system
Icon=appimage
Categories=Utility;
Terminal=true
NoDisplay=true
EOF
./appimagetool-*-x86_64.AppImage ./appimaged.AppDir
