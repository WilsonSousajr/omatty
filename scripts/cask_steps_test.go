// These tests guard scripts/cask-steps.sh and the pipeline around it (#369).
// GoReleaser v2.18.2 renders the cask's install hook as a raw `postflight`
// block, which Homebrew deprecates, and every `brew install` printed a warning
// asking the installer to report a bug in our tap. So GoReleaser renders the
// cask without the hook and does not upload it; cask-steps.sh writes the hook
// as `postflight_steps`, and release.yml publishes the result.
package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// caskSteps copies cask into a temporary file, runs cask-steps.sh on it, and
// returns the file's content afterwards and the script's combined output.
func caskSteps(t *testing.T, cask string) (string, string, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "omatty.rb")
	if err := os.WriteFile(path, []byte(cask), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(filepath.Join(repoRoot(t), "scripts", "cask-steps.sh"), path)
	out, err := cmd.CombinedOutput()
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	return string(after), string(out), err
}

// renderedCask is the cask as GoReleaser renders it once the hook is gone.
func renderedCask(t *testing.T) string {
	t.Helper()
	return caskFixture(t, "omatty-cask.rb")
}

// caskFixture reads a cask from scripts/testdata. omatty-cask-v0.8.1.rb is
// the cask v0.8.1 published, raw postflight block and all.
func caskFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The hook still has to run: the binary is unsigned, and Gatekeeper refuses a
// quarantined copy. It goes after the binary stanza, where Homebrew's stanza
// order puts postflight_steps. `{{staged_path}}` is spelled as a token because
// a step's args are plain strings, never resolved against the staged
// directory, and must_succeed: false keeps system_command's old tolerance.
func TestCaskSteps_writesTheHookAsPostflightSteps_issue369(t *testing.T) {
	after, out, err := caskSteps(t, renderedCask(t))
	if err != nil {
		t.Fatalf("cask-steps.sh: %v\n%s", err, out)
	}
	want := `  postflight_steps do
    on_macos do
      run "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "{{staged_path}}/omatty"], must_succeed: false
    end
  end
`
	at := strings.Index(after, want)
	if at < 0 {
		t.Fatalf("the cask lacks the postflight_steps stanza:\n%s", after)
	}
	if binary := strings.Index(after, `binary "omatty"`); binary < 0 || binary > at {
		t.Errorf("postflight_steps is not after the binary stanza:\n%s", after)
	}
	if zap := strings.Index(after, "# No zap stanza required"); zap < at {
		t.Errorf("postflight_steps is not before the closing stanzas:\n%s", after)
	}
	if strings.Contains(after, "postflight do") {
		t.Errorf("the cask still has a deprecated postflight block:\n%s", after)
	}
}

// If GoReleaser renders the raw block again, somebody put the hook back into
// .goreleaser.yaml; publishing both would install the warning again.
func TestCaskSteps_refusesADeprecatedPostflightBlock_issue369(t *testing.T) {
	cask := caskFixture(t, "omatty-cask-v0.8.1.rb")
	after, out, err := caskSteps(t, cask)
	if err == nil {
		t.Errorf("cask-steps.sh accepted a cask with a deprecated postflight block:\n%s", after)
	}
	if after != cask {
		t.Errorf("cask-steps.sh changed a cask it refused:\n%s", after)
	}
	if !strings.Contains(out, "postflight") {
		t.Errorf("the refusal does not name the postflight block: %q", out)
	}
}

// Once GoReleaser can emit the steps itself (goreleaser#6873), this script is
// the thing to delete; running it on such a cask would write the hook twice.
func TestCaskSteps_refusesACaskThatAlreadyHasTheSteps_issue369(t *testing.T) {
	once, out, err := caskSteps(t, renderedCask(t))
	if err != nil {
		t.Fatalf("cask-steps.sh: %v\n%s", err, out)
	}
	twice, _, err := caskSteps(t, once)
	if err == nil {
		t.Errorf("cask-steps.sh ran twice on the same cask:\n%s", twice)
	}
	if twice != once {
		t.Errorf("cask-steps.sh changed a cask it refused:\n%s", twice)
	}
}

// Without a binary stanza there is nowhere legal to put the hook.
func TestCaskSteps_refusesACaskWithNoBinaryStanza_issue369(t *testing.T) {
	cask := strings.Replace(renderedCask(t), "  binary \"omatty\"\n", "", 1)
	after, _, err := caskSteps(t, cask)
	if err == nil {
		t.Errorf("cask-steps.sh accepted a cask with no binary stanza:\n%s", after)
	}
}

// GoReleaser must still render the cask - `repository` is what makes it do
// that - but never upload it, and never carry the hook itself.
func TestGoReleaser_RendersTheCaskButNeverUploadsIt_issue369(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// Only configuration lines count: the comment explaining the hook's
	// absence has to name it.
	var config []string
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "#") {
			config = append(config, line)
		}
	}
	cfg := strings.Join(config, "\n")
	casks := cfg[strings.Index(cfg, "\nhomebrew_casks:"):]
	for _, want := range []string{"skip_upload: true", "repository:", "name: homebrew-tap"} {
		if !strings.Contains(casks, want) {
			t.Errorf(".goreleaser.yaml's homebrew_casks lacks %q", want)
		}
	}
	for _, unwanted := range []string{"hooks:", "system_command", "postflight"} {
		if strings.Contains(casks, unwanted) {
			t.Errorf(".goreleaser.yaml's homebrew_casks still has %q, which renders the deprecated block", unwanted)
		}
	}
}

// A tag publishes the patched cask, and only after GoReleaser has rendered it.
func TestCI_ReleasePublishesThePatchedCask_issue369(t *testing.T) {
	release := releaseWorkflow(t)
	steps := []string{
		"goreleaser/goreleaser-action",
		"./scripts/cask-steps.sh dist/homebrew/Casks/omatty.rb",
		"repository: WilsonSousajr/homebrew-tap",
		"git push",
	}
	last := -1
	for _, step := range steps {
		at := strings.Index(release, step)
		if at < 0 {
			t.Fatalf("release.yml lacks %q", step)
		}
		if at < last {
			t.Errorf("release.yml runs %q out of order; want %v", step, steps)
		}
		last = at
	}
	if !strings.Contains(release, "secrets.HOMEBREW_TAP_TOKEN") {
		t.Error("release.yml no longer passes HOMEBREW_TAP_TOKEN, so the tap cannot be written")
	}
}

// Every pull request patches the cask its snapshot rendered and lints it, and
// installs a patched cask for real on macOS with deprecations made fatal - so
// a cask that would warn again fails a pull request rather than a stranger's
// install.
func TestCI_ChecksThePatchedCaskOnEveryPullRequest_issue369(t *testing.T) {
	ci := ciWorkflow(t)
	snapshot := strings.Index(ci, "--snapshot")
	patch := strings.Index(ci, "./scripts/cask-steps.sh dist/homebrew/Casks/omatty.rb")
	if snapshot < 0 || patch < snapshot {
		t.Errorf("ci.yml does not patch the cask after the snapshot renders it (snapshot %d, patch %d)", snapshot, patch)
	}
	for _, want := range []string{"brew style", "HOMEBREW_DEVELOPER: 1", "brew install --cask", "macos-latest"} {
		if !strings.Contains(ci, want) {
			t.Errorf("ci.yml lacks %q", want)
		}
	}
}
