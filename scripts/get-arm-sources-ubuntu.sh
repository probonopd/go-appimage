#!/usr/bin/env bash
# we are going to do something weird
# lets replace azure's sources.list
set -eux
echo "Regenerating sources for $APT_GET_ARCH"
echo "deb [arch=amd64,i386] http://us.archive.ubuntu.com/ubuntu/ bionic main restricted universe multiverse" | sudo tee /etc/apt/sources.list
echo "deb [arch=amd64,i386] http://us.archive.ubuntu.com/ubuntu/ bionic-updates main restricted universe multiverse" | sudo tee -a /etc/apt/sources.list
echo "deb [arch=amd64,i386] http://us.archive.ubuntu.com/ubuntu/ bionic-backports main restricted universe multiverse" | sudo tee -a /etc/apt/sources.list
echo "deb [arch=amd64,i386] http://security.ubuntu.com/ubuntu bionic-security main restricted universe multiverse" | sudo tee -a /etc/apt/sources.list

echo "deb [arch=arm64,armhf,ppc64el,s390x] http://ports.ubuntu.com/ubuntu-ports/ bionic main restricted universe multiverse" | sudo tee -a /etc/apt/sources.list
echo "deb [arch=arm64,armhf,ppc64el,s390x] http://ports.ubuntu.com/ubuntu-ports/ bionic-updates main restricted universe multiverse" | sudo tee -a /etc/apt/sources.list
echo "deb [arch=arm64,armhf,ppc64el,s390x] http://ports.ubuntu.com/ubuntu-ports/ bionic-backports main restricted universe multiverse" | sudo tee -a /etc/apt/sources.list
echo "deb [arch=arm64,armhf,ppc64el,s390x] http://ports.ubuntu.com/ubuntu-ports/ bionic-security main restricted universe multiverse" | sudo tee -a /etc/apt/sources.list
sudo dpkg --add-architecture "$APT_GET_ARCH"
sudo apt-get update
sudo apt-get install -qq -y linux-libc-dev:$APT_GET_ARCH
sudo apt-get install -qq -y libc6:$APT_GET_ARCH zlib1g:$APT_GET_ARCH zlib1g-dev:$APT_GET_ARCH libfuse2:$APT_GET_ARCH 

if [[ "$APT_GET_ARCH" == "arm" ]]; then
    sudo apt-get install -qq -y libc6-armel:$APT_GET_ARCH
fi
sudo apt-get -y -qq install qemu-arm-static
sudo apt-get -qq -y install gcc-arm-linux-gnueabi gcc-aarch64-linux-gnu autoconf
