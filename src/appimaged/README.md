# appimaged

This is an experimental implementation of the optional AppImage daemon, `appimaged`, in Go, mainly to see what is possible.

To try it out:

```
# Remove pre-existing similar tools
systemctl --user stop appimaged || true
systemctl --user stop appimagelauncherd || true
sudo apt-get -yremove appimagelauncher || true

# Clear caches
rm "$HOME"/.thumbnails/normal/*
rm "$HOME"/.thumbnails/large/*
rm "$HOME"/.local/share/applications/appimage*

# Get everything (as long as this is not packaged as an AppImage yet)
wget -c https://github.com/probonopd/static-tools/releases/download/continuous/unsquashfs
wget -c https://github.com/probonopd/appimage/releases/download/continuous/appimaged-x86_64
chmod +x appimaged-x86_64 unsquashfs

# Launch
./appimaged-x86_64
```
