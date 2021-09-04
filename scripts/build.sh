#!/bin/bash

# set -e
# set -x

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

# First get the architecture we're currently on

# Make sure we have a good Go environment
if [[ $(go version) != "go version go1.17"* ]]; then
  ARCH=$(uname -m)
  if [ ARCH == "x86_64"]; then
    ARCH=amd64
  elif [[ ARCH == *"86" ]]; then
    ARCH=386
  elif [[ ARCH == "aarch64"* ]]; then
    ARCH=arm64
  elif [[ ARCH == "armv8"* ]]; then
    ARCH=arm64
  elif [[ ARCH == "arm"* ]]; then
    ARCH=arm
  else
    exit 1
  fi
  if [ -z $GOPATH ] ; then
    GOPATH=$PWD/gopath
  fi
  mkdir -p $GOPATH/src || true

  # Export version and build number
  if [ ! -z "$GITHUB_RUN_NUMBER" ] ; then
    export COMMIT="${GITHUB_RUN_NUMBER}" # "${TRAVIS_JOB_WEB_URL} on $(date +'%Y-%m-%d_%T')"
    export VERSION=$GITHUB_RUN_NUMBER
  else
    export COMMIT=$(date '+%Y-%m-%d_%H%M%S')
    export VERSION=$(date '+%Y-%m-%d_%H%M%S')
  fi
  wget -c -nv https://dl.google.com/go/go1.17.linux-$ARCH.tar.gz
  mkdir path || true
  tar -C $PWD/path -xzf go*.tar.gz
  PATH=$PWD/path/go/bin:$PATH
else
  ARCH=$(go env GOHOSTARCH)  
fi
##############################################################
# Build appimagetool, appimaged, and mkappimage
##############################################################

cd $GITHUB_WORKSPACE
mkdir build || true

# 64-bit
go build -o ./build -v -trimpath -ldflags="-s -w -X main.commit=$COMMIT" ./src/...
mv ./build/appimaged ./build/appimaged-$(go env GOHOSTARCH)
mv ./build/appimagetool ./build/appimagetool-$(go env GOHOSTARCH)
mv ./build/mkappimage ./build/mkappimage-$(go env GOHOSTARCH)

# 32-bit
if [ $(go env GOHOSTARCH) == "amd64" ] ; then 
  env GOOS=linux GOARCH=386 go build -o ./build -v -trimpath -ldflags="-s -w -X main.commit=$COMMIT" ./src/...
  mv ./build/appimaged ./build/appimaged-386
  mv ./build/appimagetool ./build/appimagetool-386
  mv ./build/mkappimage ./build/mkappimage-386
elif [ $(go env GOHOSTARCH) == "arm64" ] ; then
  env CC=arm-linux-gnueabi-gcc GOOS=linux GOARCH=arm GOARM=6 go build -o ./build -v -trimpath -ldflags="-s -w -X main.commit=$COMMIT" ./src/...
  mv ./build/appimaged ./build/appimaged-arm
  mv ./build/appimagetool ./build/appimagetool-arm
  mv ./build/mkappimage ./build/mkappimage-arm
fi


##############################################################
# Eat our own dogfood, use appimagetool to make 
# and upload AppImages
##############################################################

cd ./build

# For some weird reason, no one seems to agree on what architectures
# should be called... argh
if [ $ARCH == "arm64" ] ; then
  ARCH=aarch64
else
  ARCH=x86_64
fi

# Make appimagetool AppImage
rm -rf appimagetool.AppDir || true
mkdir -p appimagetool.AppDir/usr/bin
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/desktop-file-validate-$ARCH -O desktop-file-validate )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/mksquashfs-$ARCH -O mksquashfs )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/patchelf-$ARCH -O patchelf )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-$ARCH )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/uploadtool/raw/master/upload.sh -O uploadtool )
chmod +x appimagetool.AppDir/usr/bin/*
cp appimagetool-$(go env GOHOSTARCH) appimagetool.AppDir/usr/bin/appimagetool
( cd appimagetool.AppDir/ ; ln -s usr/bin/appimagetool AppRun)
cp ../data/appimage.png appimagetool.AppDir/
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
( cd appimaged.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar-$ARCH -O bsdtar )
( cd appimaged.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs-$ARCH -O unsquashfs )
chmod +x appimaged.AppDir/usr/bin/*
cp appimaged-$(go env GOHOSTARCH) appimaged.AppDir/usr/bin/appimaged
( cd appimaged.AppDir/ ; ln -s usr/bin/appimaged AppRun)
cp ../data/appimage.png appimaged.AppDir/
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
./appimagetool-*-$ARCH.AppImage ./appimaged.AppDir

# Make mkappimage AppImage
rm -rf mkappimage.AppDir
mkdir -p mkappimage.AppDir/usr/bin
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/desktop-file-validate-$ARCH -O desktop-file-validate )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/mksquashfs-$ARCH -O mksquashfs )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/patchelf-$ARCH -O patchelf )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-$ARCH )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/uploadtool/raw/master/upload.sh -O uploadtool )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar-$ARCH -O bsdtar )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs-$ARCH -O unsquashfs )
chmod +x mkappimage.AppDir/usr/bin/*
cp mkappimage-$(go env GOHOSTARCH) mkappimage.AppDir/usr/bin/mkappimage
( cd mkappimage.AppDir/ ; ln -s usr/bin/mkappimage AppRun)
cp ../data/appimage.png mkappimage.AppDir/
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
./appimagetool-*-$ARCH.AppImage ./mkappimage.AppDir


### 32-bit

# For some weird reason, no one seems to agree on what architectures
# should be called... argh
if [ $ARCH == "aarch64" ] ; then
  export ARCH=armhf
else
  export ARCH=i686
fi

######################## FIXME: instaed of repeating all of what follows, turn it into a fuction that gets called

# Make appimagetool AppImage
rm -rf appimagetool.AppDir || true
mkdir -p appimagetool.AppDir/usr/bin
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/desktop-file-validate-$ARCH -O desktop-file-validate )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/mksquashfs-$ARCH -O mksquashfs )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/patchelf-$ARCH -O patchelf )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-$ARCH )
( cd appimagetool.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/uploadtool/raw/master/upload.sh -O uploadtool )
chmod +x appimagetool.AppDir/usr/bin/*

# 32-bit
if [ $(go env GOHOSTARCH) == "amd64" ] ; then 
  USEARCH=386
  # This allow for this script to be ran on something other then Debian systems when testing locally.
  if [ $GITHUB_ACTIONS == true ]; then
    sudo dpkg --add-architecture i386
    sudo apt-get update
    sudo apt-get install libc6:i386 zlib1g:i386 libfuse2:i386
  fi
elif [ $(go env GOHOSTARCH) == "arm64" ] ; then
  USEARCH=arm
  if [ $GITHUB_ACTIONS == true ]; then
    sudo dpkg --add-architecture armhf
    sudo apt-get update
    sudo apt-get install libc6:armhf zlib1g:armhf zlib1g-dev:armhf libfuse2:armhf libc6-armel:armhf
  fi
fi

cp appimagetool-$USEARCH appimagetool.AppDir/usr/bin/appimagetool
( cd appimagetool.AppDir/ ; ln -s usr/bin/appimagetool AppRun)
cp ../data/appimage.png appimagetool.AppDir/
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
( cd appimaged.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar-$ARCH -O bsdtar )
( cd appimaged.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs-$ARCH -O unsquashfs )
chmod +x appimaged.AppDir/usr/bin/*
cp appimaged-$USEARCH appimaged.AppDir/usr/bin/appimaged
( cd appimaged.AppDir/ ; ln -s usr/bin/appimaged AppRun)
cp ../data/appimage.png appimaged.AppDir/
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
./appimagetool-*-$ARCH.AppImage ./appimaged.AppDir

# Make mkappimage AppImage
rm -rf mkappimage.AppDir || true
mkdir -p mkappimage.AppDir/usr/bin
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/desktop-file-validate-$ARCH -O desktop-file-validate )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/mksquashfs-$ARCH -O mksquashfs )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/patchelf-$ARCH -O patchelf )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-$ARCH )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/uploadtool/raw/master/upload.sh -O uploadtool )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar-$ARCH -O bsdtar )
( cd mkappimage.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs-$ARCH -O unsquashfs )
chmod +x mkappimage.AppDir/usr/bin/*
cp mkappimage-$USEARCH mkappimage.AppDir/usr/bin/mkappimage
( cd mkappimage.AppDir/ ; ln -s usr/bin/mkappimage AppRun)
cp ../data/appimage.png mkappimage.AppDir/
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
./appimagetool-*-$ARCH.AppImage ./mkappimage.AppDir
