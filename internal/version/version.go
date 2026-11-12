// Package version holds the build's version, set at link time from the VERSION
// file:
//
//	go build -ldflags "-X github.com/beacon-engine/pulse/internal/version.Version=0.1.0"
//
// Release images carry the plain version (0.1.0); other master builds carry
// <version>-dev+<commit>. Local builds without the flag report "dev".
package version

var Version = "dev"
