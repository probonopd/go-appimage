// TODO: Use https://github.com/src-d/go-git or https://github.com/google/go-github to
// * Get changelog history and publish it on PubSub

package main

// ============================
// CONSTANTS
// ============================

// https://blog.kowalczyk.info/article/vEja/embedding-build-number-in-go-executable.html
// The build script needs to set, e.g.,
// go build -ldflags "-X main.commit=$TRAVIS_BUILD_NUMBER"
var commit string

// path to libc
var LibcDir = "libc"
