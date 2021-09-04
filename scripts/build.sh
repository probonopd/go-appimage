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
unset GOARCH GOBIN GOEXE GOHOSTARCH GOHOSTOS GOOS GORACE GOROOT GOTOOLDIR CC GOGCCFLAGS CGO_ENABLED GO111MODULE
if [ -z $GOPATH ] ; then
  GOPATH=$PWD/gopath
fi
mkdir -p $GOPATH/src || true

# Export version and build number
if [ ! -z "$TRAVIS_BUILD_NUMBER" ] ; then
  COMMIT="${TRAVIS_BUILD_NUMBER}" # "${TRAVIS_JOB_WEB_URL} on $(date +'%Y-%m-%d_%T')"
  VERSION=$TRAVIS_BUILD_NUMBER
else
  COMMIT=$(date '+%Y-%m-%d_%H%M%S')
  VERSION=$(date '+%Y-%m-%d_%H%M%S')
fi

# Get pinned version of Go directly from upstream
if [ "aarch64" == "$TRAVIS_ARCH" ] ; then ARCH=arm64 ; fi
if [ "amd64" == "$TRAVIS_ARCH" ] ; then ARCH=amd64 ; fi
wget -c -nv https://dl.google.com/go/go1.17.linux-$ARCH.tar.gz
mkdir path || true
tar -C $PWD/path -xzf go*.tar.gz
PATH=$PWD/path/go/bin:$PATH

##############################################################
# Build appimagetool, appimaged, and mkappimage
##############################################################

cd $TRAVIS_BUILD_DIR
go get -d -v ./...
# Download it to the normal location for later, but it'll probably fail, so we allow it
# TODO: Fix it so we don't need this step

# 64-bit
go build -o $GOPATH/src -v -trimpath -ldflags="-s -w -X main.commit=$COMMIT" ./src/...
mv $GOPATH/src/appimaged $GOPATH/src/appimaged-$(go env GOHOSTARCH)
mv $GOPATH/src/appimagetool $GOPATH/src/appimagetool-$(go env GOHOSTARCH)
mv $GOPATH/src/mkappimage $GOPATH/src/mkappimage-$(go env GOHOSTARCH)

# 32-bit
if [ $(go env GOHOSTARCH) == "amd64" ] ; then 
  env GOOS=linux GOARCH=386 go build -o $GOPATH/src -v -trimpath -ldflags="-s -w -X main.commit=$COMMIT" ./src/...
  mv $GOPATH/src/appimaged $GOPATH/src/appimaged-386
  mv $GOPATH/src/appimagetool $GOPATH/src/appimagetool-386
  mv $GOPATH/src/mkappimage $GOPATH/src/mkappimage-386
elif [ $(go env GOHOSTARCH) == "arm64" ] ; then
  env CC=arm-linux-gnueabi-gcc GOOS=linux GOARCH=arm GOARM=6 go build -o $GOPATH/src -v -trimpath -ldflags="-s -w -X main.commit=$COMMIT" ./src/...
  mv $GOPATH/src/appimaged $GOPATH/src/appimaged-arm
  mv $GOPATH/src/appimagetool $GOPATH/src/appimagetool-arm
  mv $GOPATH/src/mkappimage $GOPATH/src/mkappimage-arm
fi


##############################################################
# Eat our own dogfood, use appimagetool to make 
# and upload AppImages
##############################################################

cd $GOPATH/src

unset ARCH # It contains "amd64" which we cannot use since we need "x86_64"

# For some weird reason, no one seems to agree on what architectures
# should be called... argh
if [ "$TRAVIS_ARCH" == "aarch64" ] ; then
  ARCHITECTURE=aarch64
else
  ARCHITECTURE=x86_64
fi

# Make appimagetool AppImage
rm -rf appimagetool.AppDir || true
mkdir -p appimagetool.AppDir/usr/bin
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/desktop-file-validate-$ARCHITECTURE -O desktop-file-validate )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/mksquashfs-$ARCHITECTURE -O mksquashfs )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/patchelf-$ARCHITECTURE -O patchelf )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-$ARCHITECTURE )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/uploadtool/raw/master/upload.sh -O uploadtool )
chmod +x appimagetool.AppDir/usr/bin/*
cp appimagetool-$(go env GOHOSTARCH) appimagetool.AppDir/usr/bin/appimagetool
( cd appimagetool.AppDir/ ; ln -s usr/bin/appimagetool AppRun)
cp $TRAVIS_BUILD_DIR/data/appimage.png appimagetool.AppDir/
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
( cd appimaged.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar-$ARCHITECTURE -O bsdtar )
( cd appimaged.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs-$ARCHITECTURE -O unsquashfs )
chmod +x appimaged.AppDir/usr/bin/*
cp appimaged-$(go env GOHOSTARCH) appimaged.AppDir/usr/bin/appimaged
( cd appimaged.AppDir/ ; ln -s usr/bin/appimaged AppRun)
cp $TRAVIS_BUILD_DIR/data/appimage.png appimaged.AppDir/
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
./appimagetool-*-$ARCHITECTURE.AppImage ./appimaged.AppDir

# Make mkappimage AppImage
rm -rf mkappimage.AppDir
mkdir -p mkappimage.AppDir/usr/bin
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/desktop-file-validate-$ARCHITECTURE -O desktop-file-validate )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/mksquashfs-$ARCHITECTURE -O mksquashfs )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/patchelf-$ARCHITECTURE -O patchelf )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-$ARCHITECTURE )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/uploadtool/raw/master/upload.sh -O uploadtool )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar-$ARCHITECTURE -O bsdtar )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs-$ARCHITECTURE -O unsquashfs )
chmod +x mkappimage.AppDir/usr/bin/*
cp mkappimage-$(go env GOHOSTARCH) mkappimage.AppDir/usr/bin/mkappimage
( cd mkappimage.AppDir/ ; ln -s usr/bin/mkappimage AppRun)
cp $GOPATH/src/github.com/probonopd/go-appimage/data/appimage.png mkappimage.AppDir/
cat > mkappimage.AppDir/mkappimage.desktop <<\EOF
[Desktop Entry]
Type=Application
Name=mkappimage
Exec=mkappimage
Comment=Core AppImage creation tool
Icon=appimage
Categories=Utility;
Terminal=true
NoDisplay=true
EOF
./appimagetool-*-$ARCHITECTURE.AppImage ./mkappimage.AppDir


### 32-bit

# For some weird reason, no one seems to agree on what architectures
# should be called... argh
if [ "$TRAVIS_ARCH" == "aarch64" ] ; then
  export ARCHITECTURE=armhf
else
  export ARCHITECTURE=i686
fi

######################## FIXME: instaed of repeating all of what follows, turn it into a fuction that gets called

# Make appimagetool AppImage
rm -rf appimagetool.AppDir || true
mkdir -p appimagetool.AppDir/usr/bin
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/desktop-file-validate-$ARCHITECTURE -O desktop-file-validate )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/mksquashfs-$ARCHITECTURE -O mksquashfs )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/patchelf-$ARCHITECTURE -O patchelf )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-$ARCHITECTURE )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/uploadtool/raw/master/upload.sh -O uploadtool )
chmod +x appimagetool.AppDir/usr/bin/*

# 32-bit
if [ $(go env GOHOSTARCH) == "amd64" ] ; then 
  USEARCH=386
  sudo dpkg --add-architecture i386
  sudo apt-get update
  sudo apt-get install libc6:i386 zlib1g:i386 libfuse2:i386
elif [ $(go env GOHOSTARCH) == "arm64" ] ; then
  USEARCH=arm
  sudo dpkg --add-architecture armhf
  sudo apt-get update
  sudo apt-get install libc6:armhf zlib1g:armhf zlib1g-dev:armhf libfuse2:armhf libc6-armel:armhf
fi

cp appimagetool-$USEARCH appimagetool.AppDir/usr/bin/appimagetool
( cd appimagetool.AppDir/ ; ln -s usr/bin/appimagetool AppRun)
cp $TRAVIS_BUILD_DIR/data/appimage.png appimagetool.AppDir/
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
( cd appimaged.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar-$ARCHITECTURE -O bsdtar )
( cd appimaged.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs-$ARCHITECTURE -O unsquashfs )
chmod +x appimaged.AppDir/usr/bin/*
cp appimaged-$USEARCH appimaged.AppDir/usr/bin/appimaged
( cd appimaged.AppDir/ ; ln -s usr/bin/appimaged AppRun)
cp $TRAVIS_BUILD_DIR/data/appimage.png appimaged.AppDir/
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
./appimagetool-*-$ARCHITECTURE.AppImage ./appimaged.AppDir

# Make mkappimage AppImage
rm -rf mkappimage.AppDir || true
mkdir -p mkappimage.AppDir/usr/bin
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/desktop-file-validate-$ARCHITECTURE -O desktop-file-validate )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/mksquashfs-$ARCHITECTURE -O mksquashfs )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/patchelf-$ARCHITECTURE -O patchelf )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-$ARCHITECTURE )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/uploadtool/raw/master/upload.sh -O uploadtool )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar-$ARCHITECTURE -O bsdtar )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs-$ARCHITECTURE -O unsquashfs )
chmod +x mkappimage.AppDir/usr/bin/*
cp mkappimage-$USEARCH mkappimage.AppDir/usr/bin/mkappimage
( cd mkappimage.AppDir/ ; ln -s usr/bin/mkappimage AppRun)
cp $GOPATH/src/github.com/probonopd/go-appimage/data/appimage.png mkappimage.AppDir/
cat > mkappimage.AppDir/mkappimage.desktop <<\EOF
[Desktop Entry]
Type=Application
Name=mkappimage
Exec=mkappimage
Comment=Core AppImage creation tool
Icon=appimage
Categories=Utility;
Terminal=true
NoDisplay=true
EOF
./appimagetool-*-$ARCHITECTURE.AppImage ./mkappimage.AppDir
