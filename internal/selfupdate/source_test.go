package selfupdate

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"1.2.3-next", "1.2.3"},
		{"v1.2.3-dirty", "1.2.3"},
		{"v1.2.3+meta", "1.2.3"},
		{"dev", ""},
		{"", ""},
		{"  v1.0.0  ", "1.0.0"},
	}
	for _, c := range cases {
		if got := NormalizeVersion(c.in); got != c.want {
			t.Errorf("NormalizeVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.4.2", "1.5.0", -1},
		{"1.5.0", "1.5.0", 0},
		{"", "1.0.0", -1},
		{"1.0.0", "", 1},
		{"", "", 0},
		{"2.0.0", "1.9.9", 1},
		{"0.1.2", "0.1.10", -1}, // numeric, not lexical
		{"1.0", "1.0.0", 0},
		{"1", "1.0.1", -1},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestArchiveName(t *testing.T) {
	cases := []struct {
		version, goos, goarch, want string
	}{
		{"v1.5.0", "darwin", "arm64", "agiler_1.5.0_darwin_arm64.tar.gz"},
		{"v1.5.0", "linux", "amd64", "agiler_1.5.0_linux_x86_64.tar.gz"},
		{"0.1.2", "darwin", "amd64", "agiler_0.1.2_darwin_x86_64.tar.gz"},
		{"v2.10.0", "linux", "arm64", "agiler_2.10.0_linux_arm64.tar.gz"},
	}
	for _, c := range cases {
		if got := ArchiveName(c.version, c.goos, c.goarch); got != c.want {
			t.Errorf("ArchiveName(%q,%q,%q) = %q, want %q", c.version, c.goos, c.goarch, got, c.want)
		}
	}
}

func TestChecksumLookup(t *testing.T) {
	sums := []byte(strings.TrimSpace(`
b16ff7cd25433b12b4e4af0351df482f80713b8789759bc2aafb2be0729ed685  agiler_0.1.0_darwin_arm64.tar.gz
4f75521a57342baf5d4dac454e458c1dc99d17dda4c9641ce3e188f91bef1756  agiler_0.1.0_darwin_x86_64.tar.gz
52a139fcbcb22be32cac462d9523ba853f2fb4ccffa49563bccd19442cec0cb7  agiler_0.1.0_linux_arm64.tar.gz
`))

	got, err := ChecksumLookup(sums, "agiler_0.1.0_linux_arm64.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "52a139fcbcb22be32cac462d9523ba853f2fb4ccffa49563bccd19442cec0cb7" {
		t.Errorf("wrong checksum: %q", got)
	}

	if _, err := ChecksumLookup(sums, "agiler_0.1.0_linux_x86_64.tar.gz"); err == nil {
		t.Error("expected error for missing entry, got nil")
	}
}

func TestDetectInstallSource(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GOBIN", "")
	t.Setenv("GOPATH", "")

	goInstalled := filepath.Join(home, "go", "bin", "agiler")

	cases := []struct {
		name    string
		exe     string
		version string
		want    InstallSource
	}{
		{"brew macos", "/opt/homebrew/bin/agiler", "v0.1.2", SourceHomebrew},
		{"brew caskroom", "/opt/homebrew/Caskroom/agiler/0.1.2/agiler", "v0.1.2", SourceHomebrew},
		{"brew cellar", "/opt/homebrew/Cellar/agiler/0.1.0/bin/agiler", "v0.1.2", SourceHomebrew},
		{"brew linux", "/home/linuxbrew/.linuxbrew/bin/agiler", "v0.1.2", SourceHomebrew},
		{"go install home", goInstalled, "v0.1.2", SourceGoInstall},
		{"self-managed", filepath.Join(home, ".local", "bin", "agiler"), "v0.1.2", SourceSelfManaged},
		{"self-managed system", "/usr/local/bin/agiler", "v0.1.2", SourceSelfManaged},
		{"dev build", filepath.Join(home, ".local", "bin", "agiler"), "dev", SourceDev},
		{"dev build anywhere", "/opt/homebrew/bin/agiler", "dev", SourceDev},
	}
	for _, c := range cases {
		got, _ := DetectInstallSource(c.exe, c.version)
		if got != c.want {
			t.Errorf("%s: DetectInstallSource(%q,%q) = %d, want %d", c.name, c.exe, c.version, got, c.want)
		}
	}
}
