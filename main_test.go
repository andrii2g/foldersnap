package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseExcludes(t *testing.T) {
	got := parseExcludes(" .git, node_modules ,, bin ,obj ")
	want := []string{".git", "node_modules", "bin", "obj"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseExcludes() = %#v, want %#v", got, want)
	}
}

func TestParseCreateArgsAcceptsFlagsBeforeAndAfterRoot(t *testing.T) {
	root := t.TempDir()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "flags after root",
			args: []string{root, "--out", "snapshot.json", "--exclude", ".git,node_modules"},
		},
		{
			name: "flags before root",
			args: []string{"--out=snapshot.json", "--exclude=.git,node_modules", root},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCreateArgs(tt.args)
			if err != nil {
				t.Fatalf("parseCreateArgs() error = %v", err)
			}
			if got.Root != root {
				t.Fatalf("Root = %q, want %q", got.Root, root)
			}
			if got.Out != "snapshot.json" {
				t.Fatalf("Out = %q, want snapshot.json", got.Out)
			}
			wantExcludes := []string{".git", "node_modules"}
			if !reflect.DeepEqual(got.Excludes, wantExcludes) {
				t.Fatalf("Excludes = %#v, want %#v", got.Excludes, wantExcludes)
			}
		})
	}
}

func TestParseCreateArgsValidation(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing out",
			args: []string{root},
			want: "--out is required",
		},
		{
			name: "unknown flag",
			args: []string{root, "--out", "snapshot.json", "--bad-flag"},
			want: "unknown flag: --bad-flag",
		},
		{
			name: "missing root",
			args: []string{"--out", "snapshot.json"},
			want: "create requires exactly one folder argument",
		},
		{
			name: "root is file",
			args: []string{filePath, "--out", "snapshot.json"},
			want: "root path is not a directory",
		},
		{
			name: "root missing",
			args: []string{filepath.Join(root, "missing"), "--out", "snapshot.json"},
			want: "root path does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCreateArgs(tt.args)
			if err == nil {
				t.Fatal("parseCreateArgs() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseCreateArgs() error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestRelativeSlashPath(t *testing.T) {
	root := filepath.Join("C:", "work", "repo")
	fullPath := filepath.Join(root, "dir", "file.txt")

	got, err := relativeSlashPath(root, fullPath)
	if err != nil {
		t.Fatalf("relativeSlashPath() error = %v", err)
	}
	if got != "dir/file.txt" {
		t.Fatalf("relativeSlashPath() = %q, want %q", got, "dir/file.txt")
	}
}

func TestShouldExclude(t *testing.T) {
	excludes := map[string]struct{}{
		".git":         {},
		"node_modules": {},
	}

	if !shouldExclude("src/node_modules/pkg/index.js", excludes) {
		t.Fatal("shouldExclude() = false, want true for nested excluded segment")
	}
	if shouldExclude("src/pkg/index.js", excludes) {
		t.Fatal("shouldExclude() = true, want false for allowed path")
	}
}

func TestEntryFromInfoDirectoryHasZeroSize(t *testing.T) {
	dir := t.TempDir()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	entry := entryFromInfo("tmp", info)
	if entry.Type != "directory" {
		t.Fatalf("Type = %q, want directory", entry.Type)
	}
	if entry.SizeBytes != 0 {
		t.Fatalf("SizeBytes = %d, want 0", entry.SizeBytes)
	}
	if entry.ModifiedUTC.Location() != time.UTC {
		t.Fatalf("ModifiedUTC location = %v, want UTC", entry.ModifiedUTC.Location())
	}
	if entry.Permissions == nil || len(*entry.Permissions) != 4 {
		t.Fatalf("Permissions = %#v, want 4-digit octal string", entry.Permissions)
	}
}
