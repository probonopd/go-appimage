# Go AppImage [![Build Status](https://travis-ci.com/probonopd/appimage.svg?branch=master)](https://travis-ci.com/probonopd/appimage)

Purely experimental playgroud for a Go implementation of AppImage tools.

Nothing useful here to see at this time. You might be looking for https://github.com/AppImage/AppImageKit instead in case you are looking for current production code.

## Why Go?

I am playing around with Go because

* Go follows the "keep it simple" principle - in line with what I like
* Go compiles code to static binaries by default - no messing around with shared libraries that tend to break on some target systems (e.g., for converting SVG to PNG)
* Go does not need Makefiles, Autoconf, CMake, Meson - stuff that adds "meta work" which I don't like to spend my time on
* Go is designed with concurrency and networking in mind - stuff that will come in handy for building in p2p distribution and updating
* Go is something I want to learn - and one learns best using a concrete project

## Ideas

* Build in p2p distribution using IPFS (which is written in Go)? https://github.com/hsanjuan/ipfs-lite/
* Build in zsync (Go should be a good choice for it)? https://github.com/agriardyan/go-zsyncmake/ + TBD

## TODO

* Get rid of C code embedded in Go (e.g., for the calculation of the size of an ELF)

## Conventions

* https://github.com/golang-standards/project-layout/tree/master/pkg
