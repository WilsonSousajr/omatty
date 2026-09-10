package main

import "runtime/debug"

// version is the release this binary was built from, written by the linker:
//
//	go build -ldflags "-X main.version=v0.1.0" ./cmd/omatty
//
// A package-level var rather than a const because -X can only write to a var;
// nothing assigns to it at runtime, so the "no package-level mutable state"
// rule in AGENTS.md is not what this is. Empty under a plain `go build`, and
// versionLine turns that into "dev" (#134).
var version string

// versionLine renders what `omatty --version` prints. It takes the version
// rather than reading the package var so the empty-linker case is testable
// without a build flag.
//
//	versionLine("v0.1.0")  // "omatty v0.1.0"
//	versionLine("")        // "omatty dev (built from source)"
func versionLine(v string) string {
	if v == "" {
		return "omatty dev (built from source)"
	}
	return "omatty " + v
}

// isVersionFlag reports whether arg asks for the version. Three spellings
// answer: "version" matches omatty's other subcommands, and the two dashed
// forms are what a stranger tries first. "-v" is deliberately not one of
// them - it reads as "verbose" at least as often as "version", and omatty
// has no flag parser to disambiguate it in (#134).
func isVersionFlag(arg string) bool {
	return arg == "version" || arg == "--version" || arg == "-version"
}

// resolveVersion picks the version to report. The linker's -X value wins; a
// binary from `go install github.com/WilsonSousajr/omatty/cmd/omatty@v0.1.0`
// carries no -X but does carry the module version, and that is the install
// line the README gives, so falling back to it is what stops every ordinary
// install reporting "dev" (#134).
//
// What the fallback actually yields is worth knowing, because it is not one
// value: at a clean tagged commit the go tool stamps the tag, on a dirty tree
// a pseudo-version ending "+dirty" that names the commit - more useful in a
// bug report than "dev", not less - and with -buildvcs=false or from a source
// tarball, nothing. "(devel)" is the fourth: a non-version some toolchains
// report, treated here the same as none at all.
//
// Pure, taking both strings, so every branch is testable without a build.
func resolveVersion(linked, module string) string {
	if linked != "" {
		return linked
	}
	if module == "(devel)" {
		return ""
	}
	return module
}

// moduleVersion is the version the go tool stamped into this binary, or ""
// when the build carries no module information at all (`go test` does not).
func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Version
}
