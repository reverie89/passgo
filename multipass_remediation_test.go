package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppSearchDirsFrom(t *testing.T) {
	dirs := appSearchDirsFrom("/tmp/passgo-bin/passgo", "/tmp/passgo-cwd")
	if len(dirs) != 2 {
		t.Fatalf("expected 2 dirs, got %d (%v)", len(dirs), dirs)
	}
	if dirs[0] != normalizePath("/tmp/passgo-bin") {
		t.Fatalf("unexpected first dir: %q", dirs[0])
	}
	if dirs[1] != normalizePath("/tmp/passgo-cwd") {
		t.Fatalf("unexpected second dir: %q", dirs[1])
	}

	// Duplicate paths should be de-duplicated.
	deduped := appSearchDirsFrom("/tmp/passgo-cwd/passgo", "/tmp/passgo-cwd")
	if len(deduped) != 1 {
		t.Fatalf("expected deduped dirs length 1, got %d (%v)", len(deduped), deduped)
	}
}

func TestScanCloudInitTemplateOptionsUsesBothDirsAndDedupes(t *testing.T) {
	root := t.TempDir()
	execDir := filepath.Join(root, "exec")
	cwdDir := filepath.Join(root, "cwd")
	if err := os.MkdirAll(execDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cwdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	write(filepath.Join(execDir, "exec-only.yml"), "#cloud-config\npackages: []\n")
	write(filepath.Join(cwdDir, "cwd-only.yaml"), "#cloud-config\npackages: []\n")
	write(filepath.Join(cwdDir, "ignored.yml"), "not-cloud-config\n")

	opts, err := scanCloudInitTemplateOptions([]string{execDir, cwdDir, cwdDir})
	if err != nil {
		t.Fatalf("scanCloudInitTemplateOptions returned error: %v", err)
	}
	if len(opts) != 2 {
		t.Fatalf("expected 2 templates, got %d (%v)", len(opts), opts)
	}

	if opts[0].Label != "exec-only.yml" {
		t.Fatalf("expected executable-dir template first, got %q", opts[0].Label)
	}
	if opts[1].Label != "cwd-only.yaml" {
		t.Fatalf("expected cwd template second, got %q", opts[1].Label)
	}
}

func TestReadConfigGithubRepoFromDirsPrefersExecutableDir(t *testing.T) {
	root := t.TempDir()
	execDir := filepath.Join(root, "exec")
	cwdDir := filepath.Join(root, "cwd")
	if err := os.MkdirAll(execDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cwdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(execDir, ".config"),
		[]byte("github-cloud-init-repo=https://github.com/exec/repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwdDir, ".config"),
		[]byte("github-cloud-init-repo=https://github.com/cwd/repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readConfigGithubRepoFromDirs([]string{execDir, cwdDir})
	if err != nil {
		t.Fatalf("readConfigGithubRepoFromDirs returned error: %v", err)
	}
	want := "https://github.com/exec/repo"
	if got != want {
		t.Fatalf("unexpected repo URL: got %q want %q", got, want)
	}
}

func TestReadConfigGithubRepoFromDirsFallsBackToCWD(t *testing.T) {
	root := t.TempDir()
	execDir := filepath.Join(root, "exec")
	cwdDir := filepath.Join(root, "cwd")
	if err := os.MkdirAll(execDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cwdDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Exec config exists but does not contain target key.
	if err := os.WriteFile(filepath.Join(execDir, ".config"),
		[]byte("other-key=value\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwdDir, ".config"),
		[]byte("github-cloud-init-repo=@https://github.com/cwd/repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readConfigGithubRepoFromDirs([]string{execDir, cwdDir})
	if err != nil {
		t.Fatalf("readConfigGithubRepoFromDirs returned error: %v", err)
	}
	want := "https://github.com/cwd/repo"
	if got != want {
		t.Fatalf("unexpected repo URL: got %q want %q", got, want)
	}
}
