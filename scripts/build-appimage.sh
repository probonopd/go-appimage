#!/usr/bin/env bash
set -eux

mv "./$BUILD_APP" ./$BUILD_APP-$(go env GOHOSTARCH)

# export the ARCHITECTURE
export ARCHITECTURE=$BUILD_ARCH
if [[ "$BUILD_ARCH" == "386" ]]; then
    export ARCHITECTURE="i686"
fi
if [[ "$BUILD_ARCH" == "amd64" ]]; then
    export ARCHITECTURE="x86_64"
fi
if [[ "$BUILD_ARCH" == "arm64" ]]; then
    export ARCHITECTURE="aarch64"
fi
if [[ "$BUILD_ARCH" == "arm" ]]; then
    export ARCHITECTURE="armhf"
fi

export QEMU_USER_STATIC=""
if [[ "$BUILD_ARCH" == "arm64" ]]; then
    export LD_LIBRARY_PATH="/lib/$ARCHITECTURE-linux-gnu"
    export QEMU_USER_STATIC="qemu-aarch64-static -L /usr/$ARCHITECTURE-linux-gnu/ "
fi
if [[ "$BUILD_ARCH" == "arm" ]]; then
    sudo ln -s /lib/arm-linux-gnueabihf/ld-linux.so.3 /lib/ld-linux.so.3
    export LD_LIBRARY_PATH="/lib/arm-linux-gnueabihf"
    export QEMU_LD_PREFIX=/usr/arm-linux-gnueabihf/
    export QEMU_USER_STATIC="qemu-arm-static -L /usr/arm-linux-gnueabihf/ "
fi


mkdir -p "$BUILD_APP.AppDir/usr/bin"

if [[ "$BUILD_APP" != "appimaged" ]]; then
    ( cd "$BUILD_APP.AppDir/usr/bin/" ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/desktop-file-validate-$ARCHITECTURE -O desktop-file-validate )
    ( cd "$BUILD_APP.AppDir/usr/bin/" ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/mksquashfs-$ARCHITECTURE -O mksquashfs )
    ( cd "$BUILD_APP.AppDir/usr/bin/" ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/patchelf-$ARCHITECTURE -O patchelf )
    ( cd "$BUILD_APP.AppDir/usr/bin/" ; wget -c https://github.com/AppImage/AppImageKit/releases/download/continuous/runtime-$ARCHITECTURE )
    ( cd "$BUILD_APP.AppDir/usr/bin/" ; wget -c https://github.com/probonopd/uploadtool/raw/master/upload.sh -O uploadtool )
fi
if [[ "$BUILD_APP" != "appimagetool" ]]; then
    ( cd "$BUILD_APP.AppDir/usr/bin/" ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/bsdtar-$ARCHITECTURE -O bsdtar )
    ( cd "$BUILD_APP.AppDir/usr/bin/" ; wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs-$ARCHITECTURE -O unsquashfs )
fi
chmod +x $BUILD_APP.AppDir/usr/bin/*
cp "$BUILD_APP-$(go env GOHOSTARCH)" "$BUILD_APP.AppDir/usr/bin/$BUILD_APP"
( cd $BUILD_APP.AppDir/ ; ln -s usr/bin/$BUILD_APP AppRun)
cp data/appimage.png $BUILD_APP.AppDir/
cat > $BUILD_APP.AppDir/$BUILD_APP.desktop <<\EOF
[Desktop Entry]
Type=Application
Name=REPLACE_ME_APPNAME
Exec=REPLACE_ME_APPNAME
Comment=REPLACE_ME_APPNAME - tool to generate AppImages from AppDirs
Icon=appimage
Categories=Development;
Terminal=true
EOF
sed -i "s,REPLACE_ME_APPNAME,$BUILD_APP,g" $BUILD_APP.AppDir/$BUILD_APP.desktop

if [[ "$BUILD_APP" == "appimagetool" ]]; then
    ln -s $BUILD_APP.AppDir/usr/bin/* .
    PATH="$BUILD_APP.AppDir/usr/bin/:$PATH" $QEMU_USER_STATIC ./appimagetool-* ./$BUILD_APP.AppDir || true  # FIXME: remove this true
else
    # use our own dog food :)
    chmod +x ./appimagetool-*-deploy*.AppImage/*.AppImage
    $QEMU_USER_STATIC ./appimagetool-*.AppImage/*.AppImage ./$BUILD_APP.AppDir || true
fi
rm -rf ./appimagetool-*-deploy*.AppImage
mkdir dist
mv *.AppImage dist
