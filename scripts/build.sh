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
  echo "  If not given, only the host architecture is built."
  echo "  ex: build.sh -a amd64,386"
  echo ""
  echo "-o"
  echo "  Specify a build/output directory. By default creates a folder named build in the project files"
  echo ""
  echo "-dc"
  echo "  Don't clean-up build files."
  echo ""
  echo "-pc"
  echo "  Pre-Clean the build directory before building"
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
  set_arch_env $1
  case $1 in
    amd64) local ARCH=x86_64;;
    386) local ARCH=i686;;
    arm64) local ARCH=aarch64;;
    arm) local ARCH=armhf;;
  esac
  local PROG=$2
  CLEANUP+=($BUILDDIR/$PROG-$ARCH.AppDir)
  # go clean
  echo ARCH: $ARCH
  echo GOARCH: $GOARCH
  echo GOHOSTARCH: $GOHOSTARCH
  echo BUILDARCH: $BUILDARCH
  echo GOGCCFLAGS: $GOGCCFLAGS
  CGO_LDFLAGS="-no-pie" CC=/usr/local/musl/bin/musl-gcc go build -o $BUILDDIR -v -trimpath -ldflags="-linkmode external -extldflags \"-static\" -s -w -X main.commit=$COMMIT" $PROJECT/src/$PROG
  # common appimage steps
  rm -rf $BUILDDIR/$PROG-$ARCH.AppDir || true
  mkdir -p $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin
  mv $BUILDDIR/$PROG $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/$PROG
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
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/runtime-fuse2-aarch64 -O runtime-aarch64 )
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/runtime-fuse2-armhf -O runtime-armhf )
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/runtime-fuse2-i686 -O runtime-i686 )
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/runtime-fuse2-x86_64 -O runtime-x86_64 )     
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
    ( cd $BUILDDIR/$PROG-$ARCH.AppDir/usr/bin/ ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/runtime-fuse2-$AIARCH -O runtime-$AIARCH )
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
  $BUILDDIR/appimagetool-x86_64.AppDir/usr/bin/appimagetool $BUILDDIR/$PROG-$ARCH.AppDir
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
    -pc)
      PRECLEAN=true;;
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
  export VERSION=$((GITHUB_RUN_NUMBER+646))
else
  export COMMIT=$(date '+%Y-%m-%d_%H%M%S')
  export VERSION=$(date '+%Y-%m-%d_%H%M%S')
fi

# Install dependencies if needed
if [ $GITHUB_ACTIONS ]; then
  sudo apt-get update
  sudo apt-get install --yes wget file
fi

# Allow to statically link Go programs, even with cgo, using musl libc, like this:
# CC=/usr/local/musl/bin/musl-gcc go build --ldflags '-linkmode external -extldflags "-static"' hello.go
# https://honnef.co/posts/2015/06/statically_compiled_go_programs__always__even_with_cgo__using_musl/
if [ ! -e "/usr/local/musl/bin/musl-gcc" ]; then
  wget -c -q http://www.musl-libc.org/releases/musl-1.1.10.tar.gz
  tar -xvf musl-*.tar.gz
  cd musl-*/
  ./configure --enable-gcc-wrapper
  make -j$(nproc)
  sudo make install
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
if [ ! -z $PRECLEAN ]; then
  rm -rf $BUILDDIR || true
fi
mkdir -p $BUILDDIR || true
cd $BUILDDIR

# We always want the amd64 appimagetool built first so that other AppImages can be built.
# If this isn't wanted, we clean it up afterwards
build amd64 appimagetool

for arch in ${BUILDARCH[@]}; do
  for tool in ${BUILDTOOL[@]}; do
    if [ $arch == amd64 ] && [ $tool == appimagetool ]; then
      BUILDINGAPPIMAGETOOL=true
    else
      build $arch $tool
    fi
  done
done

if [ -z $BUILDINGAPPIMAGETOOL ]; then
  CLEANUP+=($BUILDDIR/appimagetool-$VERSION-x86_64.AppImage)
fi

if [ -z $DONTCLEAN]; then
  for file in ${CLEANUP[@]}; do
    echo $file
    rm -rf $file || true
  done
fi
