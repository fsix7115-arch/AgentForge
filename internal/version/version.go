// Package version carries build identity.
//
// It is a separate package so that every other package can import it without
// creating a cycle with cmd/agentforge, and so `go build -ldflags` has exactly
// one obvious place to write to.
package version

// These are overridden at link time:
//
//	go build -ldflags "-X .../internal/version.Version=1.2.3"
var (
	Version   = "0.1.0-dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// String renders the full build identity for `agentforge version`.
func String() string {
	return Version + " (" + Commit + ", built " + BuildDate + ")"
}
