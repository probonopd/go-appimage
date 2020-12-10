# GoAppImage

AppImage manipulation from Go.

Currently tries to read the squashfs using pure go (using [this library](https://github.com/CalebQ42/squashfs)). I that doesn't work, falls back to calling `unsquashfs`.