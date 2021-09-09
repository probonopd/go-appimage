#!/bin/bash

set -e
set -x

# Now accepting arguments!
# If you want a non-native architecture, specify those first, then which
# tools you want to build (leave empty for all).
# Architectures are using GOARCH values, specifically
# amd64, arm64, arm, and 386
#
# ex: ./build.sh amd64 arm appimagetool
#
# TODO: make this better with an arch paramater: ./build -a amd64,arm appimagetool

# Build and upload the contents of this repository.
#
# NOTE: Please contact the author before trying to convert this script
# into any other language. It is intentionally a bash script rather than
# a Makefile, CMake file etc.
#
# Made primarily to work on Ubuntu 18.04 with Github Actions. If building
# locally, it might work but you have to solve your own problems.

help_message() {
  echo "Usage: build.sh -a [arch] -o [output directory] -dc [programs to build]"
  echo ""
  echo "If you don't specify which programs to build, all programs will be built. Supports building appimaged, appimagetool, and mkappimage."
  echo "If appimagetool is omitted, it is still built to package the other programs."
  echo ""
  echo "-a"
  echo "  Optional comma seperated list of architectures to build for (as defined by GOARCH)."
  echo "  If not given, all architectures are built. Supports amd64, arm64, 386, and arm"
  echo "  ex: build.sh -a amd64,386"
  echo ""
  echo "-o"
  echo "  Specify a build/output directory. By default creates a folder named build in the project files"
  echo ""
  echo "-dc"
  echo "  Don't clean-up build files."
  echo ""
  echo "-h"
  echo "  Prints this message"
  exit 0
}

# Sets the necessary environment variables for the given Architecture
set_arch_env () {
  local ARCH=$1
  if [ $ARCH == arm64 ]; then
    export GOPATH=$HOME/go-arm64
    export GOARCH=arm64
    AIARCH=aarch64
  elif [ $ARCH == 386 ]; then
    export GOPATH=$HOME/go-386
    export GOARCH=386
    AIARCH=i686
  elif [ $ARCH == arm ]; then
    export GOPATH=$HOME/go-arm
    export GOARCH=arm
    export GOARM=6
    AIARCH=armhf
  elif [ $ARCH == amd64 ]; then
    export GOPATH=$HOME/go
    export GOARCH=amd64
    AIARCH=x86_64
  else
    echo "Invalid architecture: $ARCH"
    exit 1
  fi
}

# Build the given program at the given architecture. Used via build $arch $program
build () {
  local ARCH=$1
  local PROG=$2
  CLEANUP+=($BUILDDIR/$PROG-$ARCH.AppDir)
  CLEANUP+=($BUILDDIR/$PROG-$ARCH)
  set_arch_env $ARCH
  go build -o $BUILDDIR -v -trimpath -ldflags="-s -w -X main.commit=$COMMIT" $PROJECT/src/$PROG
  mv $BUILDDIR/$PROG $BUILDDIR/$PROG-$ARCH
  # common appimage steps
  rm -rf $BUILDDIR/$PROG-$ARCH.AppDir || true
  mkdir -p $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin
  cp $BUILDDIR/$PROG-$ARCH $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/$PROG
  ( cd $BUILDDIR/$PROG-$ARCH.AppDir/ ; ln -s usr/bin/$PROG AppRun)
  cp $PROJECT/data/appimage.png $BUILDDIR/$PROG-$ARCH.AppDir/
  if [ $PROG == appimaged ]; then
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar-$AIARCH -O bsdtar )
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs-$AIARCH -O unsquashfs )
    cat > $BUILDDIR/$PROG-$ARCH.AppDir/appimaged.desktop <<\EOF
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
  elif [ $PROG == appimagetool ]; then
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/desktop-file-validate-$AIARCH -O desktop-file-validate )
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/mksquashfs-$AIARCH -O mksquashfs )
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/patchelf-$AIARCH -O patchelf )
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-$AIARCH )
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/uploadtool/raw/master/upload.sh -O uploadtool )
    cat > $BUILDDIR/$PROG-$ARCH.AppDir/appimagetool.desktop <<\EOF
[Desktop Entry]
Type=Application
Name=appimagetool
Exec=appimagetool
Comment=Tool to generate AppImages from AppDirs
Icon=appimage
Categories=Development;
Terminal=true
EOF
  elif [ $PROG == mkappimage ]; then
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/desktop-file-validate-$AIARCH -O desktop-file-validate )
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/mksquashfs-$AIARCH -O mksquashfs )
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/patchelf-$AIARCH -O patchelf )
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-$AIARCH )
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/uploadtool/raw/master/upload.sh -O uploadtool )
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar-$AIARCH -O bsdtar )
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs-$AIARCH -O unsquashfs )
    cat > $BUILDDIR/$PROG-$ARCH.AppDir/mkappimage.desktop <<\EOF
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
  fi
  chmod +x $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/*
  # if [ $ARCH == arm64 ]; then
  #   qemu-aarch64 $BUILDDIR/appimagetool-$ARCH.AppDir/usr/bin/appimagetool $BUILDDIR/$PROG-$ARCH.AppDir
  # elif [ $ARCH == arm ]; then
  #   qemu-arm $BUILDDIR/appimagetool-$ARCH.AppDir/usr/bin/appimagetool $BUILDDIR/$PROG-$ARCH.AppDir
  # else
  $BUILDDIR/appimagetool-$ARCH.AppDir/usr/bin/appimagetool $BUILDDIR/$PROG-$ARCH.AppDir
  # fi
}

#############################################################
# Parse arguments
#############################################################

while [ $# -gt 0 ]; do
  case $1 in
    -a)
      BUILDARCH=(${2//,/ })
      shift;;
    -o)
      BUILDDIR=$2
      shift;;
    -dc)
      DONTCLEAN=true;;
    -h)
      help_message;;
    help)
      help_message;;
    appimaged)
      BUILDTOOL+=($1);;
    appimagetool)
      BUILDTOOL+=($1);;
    mkappimage)
      BUILDTOOL+=($1);;
    *)
      echo "Invalid parameter $1"
      exit 1;;
  esac
  shift
done
if [ -z $BUILDTOOL ]; then
  BUILDTOOL=(appimaged appimagetool mkappimage)
fi
if [ -z $BUILDARCH ]; then
  BUILDARCH=(amd64 386 arm64 arm)
fi

# Might need to add these at some point. We'll try without first though..

# 32-bit
# if [ $(go env GOHOSTARCH) == "amd64" ] ; then 
#   USEARCH=386
#   sudo dpkg --add-architecture i386
#   sudo apt-get update
#   sudo apt-get install libc6:i386 zlib1g:i386 libfuse2:i386
# elif [ $(go env GOHOSTARCH) == "arm64" ] ; then
#   USEARCH=arm
#   sudo dpkg --add-architecture armhf
#   sudo apt-get update
#   sudo apt-get install libc6:armhf zlib1g:armhf zlib1g-dev:armhf libfuse2:armhf libc6-armel:armhf
# fi

# Check for necessary qemu versions
# for arch in ${BUILDARCH[@]}; do
#   if [ $arch == arm64 ]; then
#     if [[ $(whereis qemu-aarch64) != "qemu-aarch64: /usr/bin/qemu-aarch64" ]]; then
#       if [ $GITHUB_ACTIONS ]; then
#         sudo apt update
#         sudo apt install qemu-user
#       else
#         echo "qemu-aarch64 is missing. This is need to to build for arm64."
#         exit 1
#       fi
#     fi
#   fi
#   if [ $arch == arm ]; then
#     if [[ $(whereis qemu-arm) != "qemu-arm: /usr/bin/qemu-arm" ]]; then
#       if [ $GITHUB_ACTIONS ]; then
#         sudo apt update
#         sudo apt install qemu-user
#       else
#         echo "qemu-arm is missing. This is need to to build for arm."
#         exit 1
#       fi
#     fi
#   fi
# done

#############################################################
# Setup environment
#############################################################

# Export version and build number
if [ ! -z "$GITHUB_RUN_NUMBER" ] ; then
  export COMMIT="${GITHUB_RUN_NUMBER}"
  export VERSION=$GITHUB_RUN_NUMBER
else
  export COMMIT=$(date '+%Y-%m-%d_%H%M%S')
  export VERSION=$(date '+%Y-%m-%d_%H%M%S')
fi

# Setup go1.17 if it's not installed
if [[ $(go version) != "go version go1.17"* ]]; then
  mkdir -p $GOPATH/src || true
  wget -c -nv https://dl.google.com/go/go1.17.linux-arm64.tar.gz
  mkdir path || true
  tar -C $PWD/path -xzf go*.tar.gz
  PATH=$PWD/path/go/bin:$PATH
fi

# Get the directories ready
if [ $GITHUB_ACTIONS ]; then
  PROJECT=$GITHUB_WORKSPACE
else
  PROJECT=$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )/..
fi
if [ -z $BUILDDIR ]; then
  BUILDDIR=$PROJECT/build
fi
mkdir -p $BUILDDIR || true
cd $BUILDDIR

BUILDINGAPPIMAGETOOL=false

for tool in ${BUILDTOOL[@]}; do
  if [ $tool == appimagetool ]; then
    BUILDINGAPPIMAGETOOL=true
    break
  fi
done

for arch in ${BUILDARCH[@]}; do
  # We need to make sure appimagetool is available. If we don't want it built, we mark it for deletion.
  build $arch appimagetool

  if [ ! $BUILDINGAPPIMAGETOOL ]; then
    CLEANUP+=($BUILDDIR/appimagetool*.AppImage)
  fi
  for tool in ${BUILDTOOL[@]}; do
    if [ tool != appimagetool ]; then
      build $arch $tool
    fi
  done
done

if [ ! $DONTCLEAN ]; then
  for file in ${CLEANUP[@]}; do
    echo $file
    rm -rf $file || true
  done
fi