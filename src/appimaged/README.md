# appimaged

This is an experimental implementation of the optional AppImage daemon, `appimaged`, in Go, mainly to see what is possible.

## Installation and usage

Assuming you are using a 64-bit Intel machine (arm64, also known as x86_64), you can use our pre-compiled binaries. To try it out, boot a Ubuntu, Debian, Fedora, openSUSE, elementary OS, KDE neon,... Live ISO and run:

```
# Remove pre-existing similar tools
systemctl --user stop appimaged.service || true
sudo apt-get -y remove appimagelauncher || true

# Clear cache
rm "$HOME"/.local/share/applications/appimage*

# Optionally, install Firejail (if you want sandboxing functionality)

# Download
wget -c https://github.com/$(wget -q https://github.com/probonopd/go-appimage/releases -O - | grep "appimaged-.*-x86_64.AppImage" | head -n 1 | cut -d '"' -f 2) -P ~/Applications/
chmod +x ~/Applications/appimaged-*.AppImage

# Launch
~/Applications/appimaged-*.AppImage
```

## Uninstallation
(Should remove local copy of appimaged*.AppImage or just stop and remove the service?)

```
systemctl --user stop appimaged.service || true
sudo rm /etc/systemd/user/appimaged.service
rm ~/.local/share/applications/appimagekit*.desktop
rm ~/Applications/appimaged-646-x86_64.AppImage
```

## Notes
Do not remove installed "~/Applications/appimaged*.AppImage". The service is running from this location (unless you want to do the Uninstallation process)

The extension of AppImage files MUST be case-sensitive to be recognized by appimaged service.

Folders being watched for AppImage files:
* /usr/local/bin
* /opt
* ~/Applications
* ~/.local/bin
* /tmp/.mount_appimahvMxtu/usr/bin
* /bin
* /sbin
* /usr/bin
* /usr/sbin
* /usr/local/bin
* /usr/local/sbin
* /usr/games
* /usr/local/games
* /snap/bin

https://github.com/probonopd/go-appimage/releases/tag/continuous has builds for 32-bit Intel, 32-bit ARM (e.g., Raspberry Pi), and 64-bit ARM.

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

```
sudo apt-get -y install gcc 
if [ -z $GOPATH ] ; then export GOPATH=$HOME/go ; fi
go get github.com/probonopd/go-appimage/src/appimaged 
go build -trimpath -ldflags="-s -w" github.com/probonopd/go-appimage/src/appimaged
```
