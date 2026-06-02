package main

import (
	"encoding/json"
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

func TestCreateSnapshotCollectsEntriesAndExcludesSubtrees(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "keep"))
	mustMkdirAll(t, filepath.Join(root, ".git"))
	mustWriteFile(t, filepath.Join(root, "keep", "file.txt"), "hello")
	mustWriteFile(t, filepath.Join(root, ".git", "config"), "ignored")

	snapshot, err := createSnapshot(root, []string{".git"})
	if err != nil {
		t.Fatalf("createSnapshot() error = %v", err)
	}

	gotPaths := make([]string, 0, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		gotPaths = append(gotPaths, entry.Path)
		if strings.Contains(entry.Path, ".git") {
			t.Fatalf("snapshot contains excluded path %q", entry.Path)
		}
	}

	wantPaths := []string{"keep", "keep/file.txt"}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("paths = %#v, want %#v", gotPaths, wantPaths)
	}
	if snapshot.Snapshot.EntryCount != 2 {
		t.Fatalf("EntryCount = %d, want 2", snapshot.Snapshot.EntryCount)
	}
	if snapshot.Snapshot.DirectoryCount != 1 {
		t.Fatalf("DirectoryCount = %d, want 1", snapshot.Snapshot.DirectoryCount)
	}
	if snapshot.Snapshot.FileCount != 1 {
		t.Fatalf("FileCount = %d, want 1", snapshot.Snapshot.FileCount)
	}
	if snapshot.Snapshot.TotalFileSizeBytes != 5 {
		t.Fatalf("TotalFileSizeBytes = %d, want 5", snapshot.Snapshot.TotalFileSizeBytes)
	}
	if snapshot.Snapshot.RootPath == root {
		if snapshot.Snapshot.RootPath == "" {
			t.Fatal("RootPath is empty")
		}
	}
}

func TestReadSnapshotValidatesSchemaAndDuplicates(t *testing.T) {
	tempDir := t.TempDir()

	valid := newBaseSnapshot()
	valid.Entries = []Entry{}

	validPath := filepath.Join(tempDir, "valid.json")
	writeJSONFile(t, validPath, valid)

	if _, err := readSnapshot(validPath); err != nil {
		t.Fatalf("readSnapshot(valid) error = %v", err)
	}

	invalidSchema := valid
	invalidSchema.SchemaVersion = 2
	invalidSchemaPath := filepath.Join(tempDir, "invalid-schema.json")
	writeJSONFile(t, invalidSchemaPath, invalidSchema)

	if _, err := readSnapshot(invalidSchemaPath); err == nil || !strings.Contains(err.Error(), "invalid snapshot schema version: 2") {
		t.Fatalf("readSnapshot(invalid schema) error = %v, want schema version error", err)
	}

	duplicate := valid
	duplicate.Entries = []Entry{
		{Path: "same.txt", Type: "file", ModifiedUTC: time.Now().UTC()},
		{Path: "same.txt", Type: "file", ModifiedUTC: time.Now().UTC()},
	}
	duplicatePath := filepath.Join(tempDir, "duplicate.json")
	writeJSONFile(t, duplicatePath, duplicate)

	if _, err := readSnapshot(duplicatePath); err == nil || !strings.Contains(err.Error(), "duplicate entry path in snapshot: same.txt") {
		t.Fatalf("readSnapshot(duplicate) error = %v, want duplicate path error", err)
	}
}

func TestDiffSnapshotsDetectsAndSortsChanges(t *testing.T) {
	t1 := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	t2 := t1.Add(5 * time.Minute)
	perm0644 := "0644"
	perm0755 := "0755"

	before := &SnapshotFile{
		SchemaVersion: schemaVersion,
		Entries: []Entry{
			{Path: "z-deleted.txt", Type: "file", SizeBytes: 10, ModifiedUTC: t1, Permissions: &perm0644},
			{Path: "b-modified.txt", Type: "file", SizeBytes: 10, ModifiedUTC: t1, Permissions: &perm0644},
		},
	}
	after := &SnapshotFile{
		SchemaVersion: schemaVersion,
		Entries: []Entry{
			{Path: "a-added.txt", Type: "file", SizeBytes: 1, ModifiedUTC: t1, Permissions: &perm0644},
			{Path: "b-modified.txt", Type: "file", SizeBytes: 12, ModifiedUTC: t2, Permissions: &perm0755},
		},
	}

	diff := diffSnapshots(before, after)

	if len(diff.Added) != 1 || diff.Added[0].Path != "a-added.txt" {
		t.Fatalf("Added = %#v, want a-added.txt", diff.Added)
	}
	if len(diff.Deleted) != 1 || diff.Deleted[0].Path != "z-deleted.txt" {
		t.Fatalf("Deleted = %#v, want z-deleted.txt", diff.Deleted)
	}
	if len(diff.Modified) != 1 || diff.Modified[0].Path != "b-modified.txt" {
		t.Fatalf("Modified = %#v, want b-modified.txt", diff.Modified)
	}

	gotFields := make([]string, 0, len(diff.Modified[0].Changes))
	for _, change := range diff.Modified[0].Changes {
		gotFields = append(gotFields, change.Field)
	}
	wantFields := []string{"sizeBytes", "modifiedUtc", "permissions"}
	if !reflect.DeepEqual(gotFields, wantFields) {
		t.Fatalf("Modified fields = %#v, want %#v", gotFields, wantFields)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
