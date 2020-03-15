# opensuse Tumbleweed Live ISO

## Testing `appimaged` (including autostart using systemd)

1. Write the openSUSE Tumbleweed Live ISO to USB stick using e.g., the balenaEtcher ApppImage
2. Boot into the openSUSE Tumbleweed Live ISO
3. Download the appimaged AppImage from [here](../../releases/tag/continuous)
4. Move it to `~/Applications` and make it executable
5. Double-click to run
6. Download some AppImage and see it being integrated into the menu automatically, and icons being displayed correctly
7. Reboot the system. Note that everything still works because the openSUSE Tumbleweed Live ISO is using persistency, and because appimaged has set itself as a service that gets automatically started
