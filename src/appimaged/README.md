# appimaged

This is an experimental implementation of the optional AppImage daemon, `appimaged`, in Go.

## Initial setup

Assuming you are using a 64-bit Intel machine (amd64, also known as x86_64), you can use our pre-compiled binaries. To try it out, boot a Ubuntu, Debian, Fedora, openSUSE, elementary OS, KDE neon,... Live ISO and run:

```bash
# Remove pre-existing conflicting tools (if any)
systemctl --user stop appimaged.service || true
sudo apt-get -y purge appimagelauncher || true
[ -f ~/.config/systemd/user/default.target.wants/appimagelauncherd.service ] && rm ~/.config/systemd/user/default.target.wants/appimagelauncherd.service

# Clear cache
rm "$HOME"/.local/share/applications/appimage*

# Optionally, install Firejail (if you want sandboxing functionality)

# Download
mkdir -p ~/Applications
wget -c https://github.com/$(wget -q https://github.com/probonopd/go-appimage/releases/expanded_assets/continuous -O - | grep "appimaged-.*-x86_64.AppImage" | head -n 1 | cut -d '"' -f 2) -P ~/Applications/
chmod +x ~/Applications/appimaged-*.AppImage

# Launch
~/Applications/appimaged-*.AppImage
```

## Removal

```bash
systemctl --user disable --now appimaged.service || true
rm ~/.config/systemd/user/appimaged.service
rm ~/.local/share/applications/appimagekit*.desktop
rm ~/Applications/appimaged-*-x86_64.AppImage
```

## Notes

Do not remove "~/Applications/appimaged*.AppImage". The service is running from this location (unless you want to do the uninstallation process)

The extension of AppImage files MUST be case-sensitive to be recognized by appimaged service.

Folders being watched for AppImage files:

* /usr/local/bin
* /opt
* ~/Applications
* ~/.local/bin
* ~/Downloads
* $PATH, which frequently includes /bin, /sbin, /usr/bin, /usr/sbin, /usr/local/bin, /usr/local/sbin, and other locations

<https://github.com/probonopd/go-appimage/releases/tag/continuous> has builds for 32-bit Intel, 32-bit ARM (e.g., Raspberry Pi), and 64-bit ARM.

## Features

Implemented

* Registers type-1 and type-2 AppImages
* Detects mounted and unmounted partitions by watching DBus
* Significantly lower CPU and memory usage than other implementations
* Error notifications in case applications cannot be launched for whatever reason
* If Firejail is on the $PATH, various options for running applications sandboxed via the context menu
* Updating applications via the context menu
* Opening the containing folder via the context menu
* Extracting AppImages via the context menu
* Announces itself on the local network using Zeroconf (more to come)
* Real-time notification based on PubSub when updates are available, as soon as they are uploaded
* Quality checking of AppImages and notifications in case of errors (can be extended)
* Launch Services like functionality, e.g., being able to launch the newest version of an AppImage that we know of

Envisioned

* Seamless P2P distribution using IPFS?
* Decentralized PubSub using IPFS?
* Web-of-trust (based on a Social Graph)?
* Blockchain?
* ...

## Building

If for whatever reason you would like to build from source:

```bash
scripts/build.sh appimaged
```
