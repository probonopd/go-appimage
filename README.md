# Go AppImage ![](https://github.com/probonopd/go-appimage/actions/workflows/build.yaml/badge.svg)

An implementation of [AppImage](https://appimage.org) tools written in Go by the inventor of the AppImage format.

Documentation:

* [`appimagetool`](https://github.com/probonopd/go-appimage/blob/master/src/appimagetool/README.md), a tool to deploy dependencies into AppDirs (including things like Qt, Gtk, GStreamer,...) and to convert AppDirs into AppImages
* [`appimaged`](https://github.com/probonopd/go-appimage/blob/master/src/appimaged/README.md), an optional daemon that integrates AppImages into the system, shows their icons, and makes them executable

Download them from https://github.com/probonopd/go-appimage/releases/tag/continuous.

## Why Go?

* Go follows the "keep it simple" principle - in line with what I like
* Go compiles code to static binaries by default - no messing around with shared libraries that tend to break on some target systems (e.g., for converting SVG to PNG), no need to build in Docker containers with ancient systems for compatibility
* Go does not need Makefiles, Autoconf, CMake, Meson - stuff that adds "meta work" which I don't like to spend my time on
* Go is designed with concurrency and networking in mind - stuff that will come in handy for building in p2p distribution and updating
* Go is something I want to learn - and one learns best using a concrete project

## TODO

* Get rid of C code embedded in Go
* Get rid of external tools from https://github.com/probonopd/static-tools and replace with native Go

## Conventions

* https://github.com/golang-standards/project-layout/tree/master/pkg

## Stargazers over time
[![Stargazers over time](https://starchart.cc/probonopd/go-appimage.svg?variant=adaptive)](https://starchart.cc/probonopd/go-appimage)
