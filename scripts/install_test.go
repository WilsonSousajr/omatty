// These tests guard scripts/install.sh (#517), the one line a stranger
// without Homebrew or Go runs to try omatty. Every run is hermetic: a fake
// release tree is served through file://, `uname` is a shim that names the
// platform, and a fake `brew` records what it was asked to do. ci.yml runs
// the same script against the real latest release on both runners.
package scripts_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// platforms maps `uname -s`/`uname -m` output to the GoReleaser build it
// must pick: the four archives .goreleaser.yaml publishes.
var platforms = []struct{ os, arch, target string }{
	{"Darwin", "arm64", "darwin_arm64"},
	{"Darwin", "x86_64", "darwin_amd64"},
	{"Linux", "aarch64", "linux_arm64"},
	{"Linux", "x86_64", "linux_amd64"},
}

// installRun is one hermetic run of install.sh.
type installRun struct {
	t        *testing.T
	root     string // the temporary directory everything lives under
	releases string // the fake releases tree, served as file://
	shims    string // first on PATH: uname, and brew when a test adds it
	dir      string // where omatty lands: the default, $HOME/.local/bin
	env      []string
}

// newInstallRun builds a releases tree holding v9.9.8 and v9.9.9 (latest)
// for every platform, and a uname shim reporting Linux x86_64.
func newInstallRun(t *testing.T) *installRun {
	t.Helper()
	root := t.TempDir()
	r := &installRun{
		t:        t,
		root:     root,
		releases: filepath.Join(root, "releases"),
		shims:    filepath.Join(root, "shims"),
		dir:      filepath.Join(root, "home", ".local", "bin"),
	}
	r.publish("download/v9.9.8", "9.9.8")
	r.publish("latest/download", "9.9.9")
	r.writeShim("uname", `case $1 in -s) echo "${FAKE_OS:-Linux}" ;; -m) echo "${FAKE_ARCH:-x86_64}" ;; esac`)
	return r
}

// publish writes one release's archives and checksums.txt under rel.
func (r *installRun) publish(rel, version string) {
	r.t.Helper()
	dir := filepath.Join(r.releases, rel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		r.t.Fatal(err)
	}
	var sums strings.Builder
	for _, p := range platforms {
		name := fmt.Sprintf("omatty_%s_%s.tar.gz", version, p.target)
		archive := fakeArchive(r.t, "#!/bin/sh\necho \"omatty v"+version+" "+p.target+"\"\n")
		if err := os.WriteFile(filepath.Join(dir, name), archive, 0o644); err != nil {
			r.t.Fatal(err)
		}
		fmt.Fprintf(&sums, "%x  %s\n", sha256.Sum256(archive), name)
	}
	r.writeChecksums(rel, sums.String())
}

func (r *installRun) writeChecksums(rel, sums string) {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.releases, rel, "checksums.txt"), []byte(sums), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *installRun) readChecksums(rel string) string {
	r.t.Helper()
	b, err := os.ReadFile(filepath.Join(r.releases, rel, "checksums.txt"))
	if err != nil {
		r.t.Fatal(err)
	}
	return string(b)
}

// fakeArchive is a GoReleaser-shaped tar.gz: the binary at the root, beside
// the files .goreleaser.yaml adds.
func fakeArchive(t *testing.T, binary string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range []struct {
		name, body string
		mode       int64
	}{{"LICENSE", "MIT\n", 0o644}, {"omatty", binary, 0o755}} {
		hdr := &tar.Header{Name: f.name, Mode: f.mode, Size: int64(len(f.body)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(f.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// writeShim puts an executable sh script called name first on PATH.
func (r *installRun) writeShim(name, body string) {
	r.t.Helper()
	if err := os.MkdirAll(r.shims, 0o755); err != nil {
		r.t.Fatal(err)
	}
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(r.shims, name), []byte(script), 0o755); err != nil {
		r.t.Fatal(err)
	}
}

// fakeBrew installs a brew shim that logs every call. installed says
// whether `brew list --cask omatty` finds omatty; an install or upgrade puts
// an omatty on PATH, as the real cask would.
func (r *installRun) fakeBrew(installed bool) string {
	r.t.Helper()
	log := filepath.Join(r.root, "brew.log")
	list := "exit 1"
	if installed {
		list = "exit 0"
	}
	r.writeShim("brew", fmt.Sprintf(`echo "$*" >> %q
case $1 in
list) %s ;;
install|upgrade) printf '#!/bin/sh\necho "omatty v9.9.9 brew"\n' > %q; chmod +x %q ;;
esac`, log, list, filepath.Join(r.shims, "omatty"), filepath.Join(r.shims, "omatty")))
	return log
}

// run executes install.sh the way the one-liner does, `sh` reading the
// script, with a PATH of the shims and the system's own tools only.
func (r *installRun) run(extra ...string) (string, error) {
	r.t.Helper()
	cmd := exec.Command("sh", filepath.Join(repoRoot(r.t), "scripts", "install.sh"))
	cmd.Env = append([]string{
		"HOME=" + filepath.Join(r.root, "home"),
		"PATH=" + r.shims + ":/usr/bin:/bin",
		"OMATTY_RELEASE_URL=file://" + r.releases,
	}, append(r.env, extra...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (r *installRun) installed() string {
	r.t.Helper()
	out, err := exec.Command(filepath.Join(r.dir, "omatty")).Output()
	if err != nil {
		r.t.Fatalf("the installed omatty does not run: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func (r *installRun) assertNothingInstalled() {
	r.t.Helper()
	entries, _ := os.ReadDir(r.dir)
	for _, e := range entries {
		r.t.Errorf("found %s in the install directory; want nothing installed", e.Name())
	}
}

// The latest release's archive for this platform, verified, installed, and
// run, so the user sees the build they got.
func TestInstall_installsTheLatestReleaseAndRunsIt_issue517(t *testing.T) {
	r := newInstallRun(t)
	out, err := r.run()
	if err != nil {
		t.Fatalf("install.sh: %v\n%s", err, out)
	}
	if got := r.installed(); got != "omatty v9.9.9 linux_amd64" {
		t.Errorf("installed %q; want the latest linux_amd64 build", got)
	}
	if !strings.Contains(out, "omatty v9.9.9 linux_amd64") {
		t.Errorf("install.sh did not finish by running `omatty --version`:\n%s", out)
	}
}

// Each of the four builds GoReleaser publishes is what its platform gets.
func TestInstall_picksTheBuildForEachPlatform_issue517(t *testing.T) {
	for _, p := range platforms {
		t.Run(p.target, func(t *testing.T) {
			r := newInstallRun(t)
			out, err := r.run("FAKE_OS="+p.os, "FAKE_ARCH="+p.arch)
			if err != nil {
				t.Fatalf("install.sh on %s %s: %v\n%s", p.os, p.arch, err, out)
			}
			if got, want := r.installed(), "omatty v9.9.9 "+p.target; got != want {
				t.Errorf("%s %s installed %q; want %q", p.os, p.arch, got, want)
			}
		})
	}
}

// A platform with no build stops before downloading anything, and says what
// works instead.
func TestInstall_aPlatformWithNoBuildPointsAtGoInstall_issue517(t *testing.T) {
	for _, p := range []struct{ os, arch string }{{"FreeBSD", "x86_64"}, {"Linux", "armv7l"}} {
		r := newInstallRun(t)
		out, err := r.run("FAKE_OS="+p.os, "FAKE_ARCH="+p.arch)
		if err == nil {
			t.Errorf("install.sh succeeded on %s %s:\n%s", p.os, p.arch, out)
		}
		if !strings.Contains(out, "go install github.com/WilsonSousajr/omatty/cmd/omatty@latest") {
			t.Errorf("the refusal on %s %s does not point at go install:\n%s", p.os, p.arch, out)
		}
		r.assertNothingInstalled()
	}
}

// The negative control #517 requires: an archive that does not match its
// checksum is never installed, and the message says why.
func TestInstall_aTamperedChecksumInstallsNothing_issue517(t *testing.T) {
	r := newInstallRun(t)
	sums := r.readChecksums("latest/download")
	r.writeChecksums("latest/download", strings.Repeat("0", 64)+sums[64:])
	out, err := r.run("FAKE_OS=Darwin", "FAKE_ARCH=arm64")
	if err == nil {
		t.Errorf("install.sh installed an archive whose checksum does not match:\n%s", out)
	}
	if !strings.Contains(out, "checksum") {
		t.Errorf("the refusal does not mention the checksum:\n%s", out)
	}
	r.assertNothingInstalled()
}

// checksums.txt names the archive to fetch, so a name that is not an omatty
// archive for this platform - a path out of the release, say - is refused.
func TestInstall_refusesAnArchiveNameThatIsNotARelease_issue517(t *testing.T) {
	r := newInstallRun(t)
	r.writeChecksums("latest/download", strings.Repeat("a", 64)+"  ../../omatty_9.9.9_linux_amd64.tar.gz\n")
	out, err := r.run()
	if err == nil {
		t.Errorf("install.sh accepted an archive name outside the release:\n%s", out)
	}
	r.assertNothingInstalled()
}

// OMATTY_VERSION installs that release rather than the latest; the leading
// v is optional, as a user is as likely to type 9.9.8 as v9.9.8.
func TestInstall_aPinnedVersionInstallsThatRelease_issue517(t *testing.T) {
	for _, pin := range []string{"v9.9.8", "9.9.8"} {
		r := newInstallRun(t)
		out, err := r.run("OMATTY_VERSION=" + pin)
		if err != nil {
			t.Fatalf("install.sh with OMATTY_VERSION=%s: %v\n%s", pin, err, out)
		}
		if got := r.installed(); got != "omatty v9.9.8 linux_amd64" {
			t.Errorf("OMATTY_VERSION=%s installed %q; want v9.9.8", pin, got)
		}
	}
}

// Re-running upgrades in place and leaves nothing behind.
func TestInstall_rerunningUpgradesInPlace_issue517(t *testing.T) {
	r := newInstallRun(t)
	if out, err := r.run("OMATTY_VERSION=v9.9.8"); err != nil {
		t.Fatalf("first install: %v\n%s", err, out)
	}
	if out, err := r.run(); err != nil {
		t.Fatalf("second install: %v\n%s", err, out)
	}
	if got := r.installed(); got != "omatty v9.9.9 linux_amd64" {
		t.Errorf("after re-running, omatty is %q; want the latest", got)
	}
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("the install directory holds %d entries after an upgrade; want only omatty", len(entries))
	}
}

// OMATTY_INSTALL_DIR puts omatty somewhere other than ~/.local/bin.
func TestInstall_installsWhereTheInstallDirSays_issue517(t *testing.T) {
	r := newInstallRun(t)
	r.dir = filepath.Join(r.root, "elsewhere")
	if out, err := r.run("OMATTY_INSTALL_DIR=" + r.dir); err != nil {
		t.Fatalf("install.sh: %v\n%s", err, out)
	}
	r.installed()
	if _, err := os.Stat(filepath.Join(r.root, "home", ".local", "bin", "omatty")); err == nil {
		t.Error("install.sh also installed into ~/.local/bin")
	}
}

// A directory that is not on PATH gets the line to add, not silence.
func TestInstall_saysWhenTheDirectoryIsNotOnPath_issue517(t *testing.T) {
	r := newInstallRun(t)
	out, err := r.run()
	if err != nil {
		t.Fatalf("install.sh: %v\n%s", err, out)
	}
	if !strings.Contains(out, "not on your PATH") || !strings.Contains(out, r.dir) {
		t.Errorf("install.sh did not say %s is missing from PATH:\n%s", r.dir, out)
	}
}

// git and claude are required and missing here, so each is named; dtach is
// optional and gets the line to install it, never an install.
func TestInstall_namesMissingPrerequisites_issue517(t *testing.T) {
	r := newInstallRun(t)
	r.writeShim("git", "exit 0")
	out, err := r.run()
	if err != nil {
		t.Fatalf("install.sh: %v\n%s", err, out)
	}
	if strings.Contains(out, "git is not on your PATH") {
		t.Errorf("install.sh reported git missing although it is on PATH:\n%s", out)
	}
	for _, want := range []string{"Claude Code (claude) is not on your PATH", "brew install dtach", "apt install dtach"} {
		if !strings.Contains(out, want) {
			t.Errorf("install.sh output lacks %q:\n%s", want, out)
		}
	}
}

// With Homebrew present, the tap is the one place omatty is installed and
// upgraded, so the script hands off to it and fetches nothing itself.
func TestInstall_handsOffToHomebrew_issue517(t *testing.T) {
	r := newInstallRun(t)
	log := r.fakeBrew(false)
	out, err := r.run()
	if err != nil {
		t.Fatalf("install.sh: %v\n%s", err, out)
	}
	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), "install WilsonSousajr/tap/omatty") {
		t.Errorf("brew was not asked to install from the tap; it was asked:\n%s", calls)
	}
	if !strings.Contains(out, "omatty v9.9.9 brew") {
		t.Errorf("install.sh did not run the omatty brew installed:\n%s", out)
	}
	r.assertNothingInstalled()
}

// Run again with Homebrew, it upgrades rather than reinstalling.
func TestInstall_upgradesThroughHomebrewWhenInstalled_issue517(t *testing.T) {
	r := newInstallRun(t)
	log := r.fakeBrew(true)
	if out, err := r.run(); err != nil {
		t.Fatalf("install.sh: %v\n%s", err, out)
	}
	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), "upgrade --cask WilsonSousajr/tap/omatty") {
		t.Errorf("brew was not asked to upgrade; it was asked:\n%s", calls)
	}
	if strings.Contains(string(calls), "install ") {
		t.Errorf("brew was asked to install an omatty it already has:\n%s", calls)
	}
}

// OMATTY_NO_BREW forces the archive even with brew present - which is how
// CI exercises that path on macOS - and so does a pinned version, which
// Homebrew cannot install.
func TestInstall_theArchiveDespiteHomebrew_issue517(t *testing.T) {
	for _, env := range []string{"OMATTY_NO_BREW=1", "OMATTY_VERSION=v9.9.8"} {
		r := newInstallRun(t)
		log := r.fakeBrew(false)
		out, err := r.run(env)
		if err != nil {
			t.Fatalf("install.sh with %s: %v\n%s", env, err, out)
		}
		if calls, _ := os.ReadFile(log); len(calls) > 0 {
			t.Errorf("with %s, brew was still asked:\n%s", env, calls)
		}
		r.installed()
	}
}

// The script is piped into sh, so a download cut short runs whatever
// arrived. Every prefix of it must install nothing: the body is one function,
// called on the last line.
func TestInstall_aTruncatedScriptInstallsNothing_issue517(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(string(b), "\n")
	r := newInstallRun(t)
	for n := 1; n < len(lines)-1; n++ {
		prefix := filepath.Join(r.root, "prefix.sh")
		if err := os.WriteFile(prefix, []byte(strings.Join(lines[:n], "")), 0o644); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("sh", prefix)
		cmd.Env = []string{"HOME=" + filepath.Join(r.root, "home"), "PATH=" + r.shims + ":/usr/bin:/bin",
			"OMATTY_RELEASE_URL=file://" + r.releases}
		_ = cmd.Run()
		if _, err := os.Stat(r.dir); err == nil {
			t.Fatalf("the first %d lines of install.sh created %s", n, r.dir)
		}
	}
}

// shellcheck is part of the gate (#517), pinned like every other tool the
// gate installs, and AGENTS.md's gate list says so.
func TestCI_ShellchecksTheScripts_issue517(t *testing.T) {
	ci := ciWorkflow(t)
	pin := regexp.MustCompile(`SHELLCHECK_VERSION:\s*"?v[0-9]+\.[0-9]+\.[0-9]+"?`)
	if !pin.MatchString(ci) {
		t.Error("ci.yml does not pin SHELLCHECK_VERSION to an exact vX.Y.Z")
	}
	if !strings.Contains(ci, "shellcheck scripts/*.sh") {
		t.Error("ci.yml does not run shellcheck over scripts/*.sh")
	}
	agents, err := os.ReadFile(filepath.Join(repoRoot(t), "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agents), "shellcheck scripts/*.sh") {
		t.Error("AGENTS.md's gate list does not include shellcheck")
	}
}

// The hermetic tests above never touch a real release; ci.yml runs the
// script against the latest one on both runners, with the tampered-checksum
// negative control, and on macOS through the Homebrew hand-off too.
func TestCI_RunsTheInstallScriptAgainstARealRelease_issue517(t *testing.T) {
	ci := ciWorkflow(t)
	job := strings.Index(ci, "\n  install:")
	if job < 0 {
		t.Fatal("ci.yml has no install job")
	}
	body := ci[job:]
	if next := regexp.MustCompile(`\n  [a-z-]+:\n`).FindStringIndex(body[1:]); next != nil {
		body = body[:next[0]+1]
	}
	for _, want := range []string{"ubuntu-latest", "macos-latest", "scripts/install.sh", "OMATTY_NO_BREW: 1", "OMATTY_RELEASE_URL", "checksum", "brew list --cask omatty"} {
		if !strings.Contains(body, want) {
			t.Errorf("ci.yml's install job lacks %q", want)
		}
	}
}
