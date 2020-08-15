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
unset GOARCH GOBIN GOEXE GOHOSTARCH GOHOSTOS GOOS GORACE GOROOT GOPATH GOTOOLDIR CC GOGCCFLAGS CGO_ENABLED GO111MODULE
export GOPATH=/tmp/go
mkdir -p $GOPATH/src

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

cd $GOPATH/src
go get -d -v github.com/probonopd/go-appimage/...

# 64-bit
go build -v -trimpath -ldflags="-s -w -X main.commit=$COMMIT" github.com/probonopd/go-appimage/src/appimagetool
mv ./appimagetool appimagetool-$(go env GOHOSTARCH)

# 32-bit
if [ $(go env GOHOSTARCH) == "amd64" ] ; then 
  env CGO_ENABLED=1 GOOS=linux GOARCH=386 go build -v -trimpath -ldflags="-s -w -X main.commit=$COMMIT" github.com/probonopd/go-appimage/src/appimagetool
  mv ./appimagetool appimagetool-386
elif [ $(go env GOHOSTARCH) == "arm64" ] ; then 
  env CC=arm-linux-gnueabi-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=5 go build -v -trimpath -ldflags="-s -w -X main.commit=$COMMIT" github.com/probonopd/go-appimage/src/appimagetool
  mv ./appimagetool appimagetool-arm
fi

##############################################################
# Bild appimaged
##############################################################

# 64-bit
go build -v -trimpath -ldflags="-s -w -X main.commit=$COMMIT" github.com/probonopd/go-appimage/src/appimaged
mv ./appimaged appimaged-$(go env GOHOSTARCH)

# 23-bit
if [ $(go env GOHOSTARCH) == "amd64" ] ; then 
  env CGO_ENABLED=1 GOOS=linux GOARCH=386 go build -v -trimpath -ldflags="-s -w -X main.commit=$COMMIT" github.com/probonopd/go-appimage/src/appimaged
  mv ./appimaged appimaged-386
elif [ $(go env GOHOSTARCH) == "arm64" ] ; then
  env CC=arm-linux-gnueabi-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm GOARM=5 go build -v -trimpath -ldflags="-s -w -X main.commit=$COMMIT" github.com/probonopd/go-appimage/src/appimaged
  mv ./appimaged appimaged-arm
fi

##############################################################
# Eat our own dogfood, use appimagetool to make 
# and upload AppImages
##############################################################

unset ARCH # It contains "amd64" which we cannot use since we need "x86_64"

# For some weird reason, no one seems to agree on what architectures
# should be called... argh
if [ "$TRAVIS_ARCH" == "aarch64" ] ; then
  export ARCHITECTURE=aarch64
else
  export ARCHITECTURE=x86_64
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
( cd appimaged.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar-$ARCHITECTURE -O bsdtar )
( cd appimaged.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs-$ARCHITECTURE -O unsquashfs )
chmod +x appimaged.AppDir/usr/bin/*
cp appimaged-$(go env GOHOSTARCH) appimaged.AppDir/usr/bin/appimaged
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
./appimagetool-*-$ARCHITECTURE.AppImage ./appimaged.AppDir


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

# 23-bit
if [ $(go env GOHOSTARCH) == "amd64" ] ; then 
  USEARCH=386
elif [ $(go env GOHOSTARCH) == "arm64" ] ; then
  USEARCH=arm
fi

cp appimagetool-$USEARCH appimagetool.AppDir/usr/bin/appimagetool
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
( cd appimaged.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar-$ARCHITECTURE -O bsdtar )
( cd appimaged.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs-$ARCHITECTURE -O unsquashfs )
chmod +x appimaged.AppDir/usr/bin/*
cp appimaged-$USEARCH appimaged.AppDir/usr/bin/appimaged
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
./appimagetool-*-$ARCHITECTURE.AppImage ./appimaged.AppDir
