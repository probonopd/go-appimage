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
  echo "  If not given, only the host architecture is built. NOTE: building for arm on x86 and vice versa is NOT supported"
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
    export GOARCH=arm64
    AIARCH=aarch64
  elif [ $ARCH == 386 ]; then
    export GOARCH=386
    AIARCH=i686
  elif [ $ARCH == arm ]; then
    export GOARCH=arm
    export GOARM=6
    AIARCH=armhf
  elif [ $ARCH == amd64 ]; then
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
  $BUILDDIR/$PROG-$ARCH --help
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
    if [ $ARCH != arm64 ]; then
      ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-$AIARCH )
    else
      ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-$AIARCH -O runtime-$ARCH)
    fi
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
  $BUILDDIR/appimagetool-$ARCH.AppDir/usr/bin/appimagetool $BUILDDIR/$PROG-$ARCH.AppDir
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

# Install dependencies if needed
if [ $GITHUB_ACTIONS ]; then
  sudo apt-get update
  sudo apt-get install --yes wget file gcc
fi

# Setup go1.17 if it's not installed
if [[ $(go version) != "go version go1.17"* ]]; then
  ARCH=$(uname -m)
  case $ARCH in
    x86_64)
      ARCH=amd64;;
    aarch64)
      ARCH=arm64;;
    armv8)
      ARCH=arm64;;
    *)
      echo "Building on an unsupported system architecture: $ARCH"
      exit 1;;
  esac
  mkdir -p $GOPATH/src || true
  wget -c -nv https://dl.google.com/go/go1.17.linux-$ARCH.tar.gz
  mkdir path || true
  tar -C $PWD/path -xzf go*.tar.gz
  PATH=$PWD/path/go/bin:$PATH
fi

if [ -z $BUILDTOOL ]; then
  BUILDTOOL=(appimaged appimagetool mkappimage)
fi

if [ -z $BUILDARCH ]; then
  BUILDARCH=$(go env GOHOSTARCH)
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