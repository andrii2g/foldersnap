package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const toolName = "foldersnap"
const toolVersion = "0.1.0"
const schemaVersion = 1

type SnapshotFile struct {
	SchemaVersion int              `json:"schemaVersion"`
	Tool          ToolMetadata     `json:"tool"`
	Snapshot      SnapshotMetadata `json:"snapshot"`
	System        SystemMetadata   `json:"system"`
	Options       SnapshotOptions  `json:"options"`
	Entries       []Entry          `json:"entries"`
}

type ToolMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type SnapshotMetadata struct {
	CreatedUTC         time.Time `json:"createdUtc"`
	RootPath           string    `json:"rootPath"`
	PathStyle          string    `json:"pathStyle"`
	EntryCount         int       `json:"entryCount"`
	FileCount          int       `json:"fileCount"`
	DirectoryCount     int       `json:"directoryCount"`
	SymlinkCount       int       `json:"symlinkCount"`
	OtherCount         int       `json:"otherCount"`
	TotalFileSizeBytes int64     `json:"totalFileSizeBytes"`
	DurationMs         int64     `json:"durationMs"`
}

type SystemMetadata struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname"`
}

type SnapshotOptions struct {
	IncludeDirectories bool     `json:"includeDirectories"`
	FollowSymlinks     bool     `json:"followSymlinks"`
	Hash               bool     `json:"hash"`
	Excluded           []string `json:"excluded"`
}

type Entry struct {
	Path        string     `json:"path"`
	Type        string     `json:"type"`
	SizeBytes   int64      `json:"sizeBytes"`
	CreatedUTC  *time.Time `json:"createdUtc"`
	ModifiedUTC time.Time  `json:"modifiedUtc"`
	Permissions *string    `json:"permissions"`
}

type CreateArgs struct {
	Root     string
	Out      string
	Excludes []string
}

type FieldChange struct {
	Field  string
	Before string
	After  string
}

type ModifiedEntry struct {
	Path    string
	Changes []FieldChange
}

type DiffResult struct {
	Added    []Entry
	Deleted  []Entry
	Modified []ModifiedEntry
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError("missing command")
	}

	switch args[0] {
	case "create":
		return runCreate(args[1:])
	case "diff":
		return runDiff(args[1:])
	default:
		return usageError(fmt.Sprintf("unknown command: %s", args[0]))
	}
}

func runCreate(args []string) error {
	createArgs, err := parseCreateArgs(args)
	if err != nil {
		return err
	}

	snapshot, err := createSnapshot(createArgs.Root, createArgs.Excludes)
	if err != nil {
		return err
	}

	return writeSnapshot(createArgs.Out, snapshot)
}

func runDiff(args []string) error {
	if len(args) != 2 {
		return usageError("diff requires exactly two snapshot file arguments")
	}

	before, err := readSnapshot(args[0])
	if err != nil {
		return err
	}

	after, err := readSnapshot(args[1])
	if err != nil {
		return err
	}

	printDiff(diffSnapshots(before, after))
	return nil
}

func parseCreateArgs(args []string) (CreateArgs, error) {
	var result CreateArgs
	var positionals []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.HasPrefix(arg, "--out="):
			result.Out = strings.TrimPrefix(arg, "--out=")
		case arg == "--out":
			i++
			if i >= len(args) {
				return CreateArgs{}, usageError("--out requires a value")
			}
			result.Out = args[i]
		case strings.HasPrefix(arg, "--exclude="):
			result.Excludes = parseExcludes(strings.TrimPrefix(arg, "--exclude="))
		case arg == "--exclude":
			i++
			if i >= len(args) {
				return CreateArgs{}, usageError("--exclude requires a value")
			}
			result.Excludes = parseExcludes(args[i])
		case strings.HasPrefix(arg, "--"):
			return CreateArgs{}, usageError(fmt.Sprintf("unknown flag: %s", arg))
		default:
			positionals = append(positionals, arg)
		}
	}

	if len(positionals) != 1 {
		return CreateArgs{}, usageError("create requires exactly one folder argument")
	}
	if result.Out == "" {
		return CreateArgs{}, usageError("--out is required")
	}

	root := positionals[0]
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return CreateArgs{}, fmt.Errorf("root path does not exist: %s", root)
		}
		return CreateArgs{}, fmt.Errorf("failed to stat root path %s: %w", root, err)
	}
	if !info.IsDir() {
		return CreateArgs{}, fmt.Errorf("root path is not a directory: %s", root)
	}

	result.Root = root
	return result, nil
}

func parseExcludes(raw string) []string {
	parts := strings.Split(raw, ",")
	excludes := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		excludes = append(excludes, part)
	}
	return excludes
}

func usageError(message string) error {
	return fmt.Errorf("%s\n\nUsage:\n  %s create <folder> --out <snapshot-file> [--exclude comma,separated,names]\n  %s diff <before-snapshot-file> <after-snapshot-file>", message, toolName, toolName)
}

func createSnapshot(root string, excludes []string) (*SnapshotFile, error) {
	start := time.Now()
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute root path: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	excludeSet := make(map[string]struct{}, len(excludes))
	for _, exclude := range excludes {
		excludeSet[exclude] = struct{}{}
	}

	snapshot := newBaseSnapshot()
	snapshot.System.Hostname = hostname
	snapshot.Options.Excluded = append([]string(nil), excludes...)
	snapshot.Snapshot.CreatedUTC = start.UTC()
	snapshot.Snapshot.RootPath = absRoot
	snapshot.Snapshot.PathStyle = "slash"

	if err := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == absRoot {
			return nil
		}

		relPath, err := relativeSlashPath(absRoot, path)
		if err != nil {
			return fmt.Errorf("failed to derive relative path for %s: %w", path, err)
		}

		if shouldExclude(relPath, excludeSet) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("failed to read file info for %s: %w", path, err)
		}

		entry := entryFromInfo(relPath, info)
		snapshot.Entries = append(snapshot.Entries, entry)

		switch entry.Type {
		case "file":
			snapshot.Snapshot.FileCount++
			snapshot.Snapshot.TotalFileSizeBytes += entry.SizeBytes
		case "directory":
			snapshot.Snapshot.DirectoryCount++
		case "symlink":
			snapshot.Snapshot.SymlinkCount++
		default:
			snapshot.Snapshot.OtherCount++
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to walk root path %s: %w", root, err)
	}

	sort.Slice(snapshot.Entries, func(i, j int) bool {
		return snapshot.Entries[i].Path < snapshot.Entries[j].Path
	})

	snapshot.Snapshot.EntryCount = len(snapshot.Entries)
	snapshot.Snapshot.DurationMs = time.Since(start).Milliseconds()
	return &snapshot, nil
}

func writeSnapshot(path string, snapshot *SnapshotFile) error {
	if snapshot == nil {
		return errors.New("snapshot is nil")
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write snapshot %s: %w", path, err)
	}

	return nil
}

func readSnapshot(path string) (*SnapshotFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot: %s", path)
	}

	var snapshot SnapshotFile
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("invalid snapshot JSON: %s", path)
	}

	if err := validateSnapshot(&snapshot); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

func diffSnapshots(before, after *SnapshotFile) DiffResult {
	beforeIndex := make(map[string]Entry, len(before.Entries))
	for _, entry := range before.Entries {
		beforeIndex[entry.Path] = entry
	}

	afterIndex := make(map[string]Entry, len(after.Entries))
	for _, entry := range after.Entries {
		afterIndex[entry.Path] = entry
	}

	result := DiffResult{}

	for path, afterEntry := range afterIndex {
		beforeEntry, ok := beforeIndex[path]
		if !ok {
			result.Added = append(result.Added, afterEntry)
			continue
		}

		changes := diffEntryFields(beforeEntry, afterEntry)
		if len(changes) > 0 {
			result.Modified = append(result.Modified, ModifiedEntry{
				Path:    path,
				Changes: changes,
			})
		}
	}

	for path, beforeEntry := range beforeIndex {
		if _, ok := afterIndex[path]; !ok {
			result.Deleted = append(result.Deleted, beforeEntry)
		}
	}

	sort.Slice(result.Added, func(i, j int) bool {
		return result.Added[i].Path < result.Added[j].Path
	})
	sort.Slice(result.Deleted, func(i, j int) bool {
		return result.Deleted[i].Path < result.Deleted[j].Path
	})
	sort.Slice(result.Modified, func(i, j int) bool {
		return result.Modified[i].Path < result.Modified[j].Path
	})

	return result
}

func printDiff(result DiffResult) {
	fmt.Println("Summary:")
	fmt.Printf("  Added:    %d\n", len(result.Added))
	fmt.Printf("  Deleted:  %d\n", len(result.Deleted))
	fmt.Printf("  Modified: %d\n", len(result.Modified))

	printEntryList("Added", result.Added)
	printEntryList("Deleted", result.Deleted)

	if len(result.Modified) > 0 {
		fmt.Println()
		fmt.Println("Modified:")
		for _, modified := range result.Modified {
			fmt.Printf("  %s\n", modified.Path)
			for _, change := range modified.Changes {
				fmt.Printf("    %s: %s -> %s\n", change.Field, change.Before, change.After)
			}
			fmt.Println()
		}
	}
}

func relativeSlashPath(root string, fullPath string) (string, error) {
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func shouldExclude(relSlashPath string, excludes map[string]struct{}) bool {
	for _, segment := range strings.Split(relSlashPath, "/") {
		if _, ok := excludes[segment]; ok {
			return true
		}
	}
	return false
}

func entryFromInfo(relPath string, info os.FileInfo) Entry {
	modified := info.ModTime().UTC()
	perm := fmt.Sprintf("%04o", info.Mode().Perm())
	entryType := "other"
	size := info.Size()

	switch {
	case info.Mode().IsRegular():
		entryType = "file"
	case info.IsDir():
		entryType = "directory"
		size = 0
	case info.Mode()&os.ModeSymlink != 0:
		entryType = "symlink"
	}

	return Entry{
		Path:        relPath,
		Type:        entryType,
		SizeBytes:   size,
		CreatedUTC:  getCreatedUTC(info),
		ModifiedUTC: modified,
		Permissions: &perm,
	}
}

func newBaseSnapshot() SnapshotFile {
	return SnapshotFile{
		SchemaVersion: schemaVersion,
		Tool: ToolMetadata{
			Name:    toolName,
			Version: toolVersion,
		},
		System: SystemMetadata{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
		Options: SnapshotOptions{
			IncludeDirectories: true,
			FollowSymlinks:     false,
			Hash:               false,
		},
		Entries: []Entry{},
	}
}

func validateSnapshot(snapshot *SnapshotFile) error {
	if snapshot.SchemaVersion != schemaVersion {
		return fmt.Errorf("invalid snapshot schema version: %d", snapshot.SchemaVersion)
	}
	if snapshot.Entries == nil {
		return errors.New("snapshot entries must not be nil")
	}

	seen := make(map[string]struct{}, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		if _, exists := seen[entry.Path]; exists {
			return fmt.Errorf("duplicate entry path in snapshot: %s", entry.Path)
		}
		seen[entry.Path] = struct{}{}
	}

	return nil
}

func diffEntryFields(before, after Entry) []FieldChange {
	fields := []struct {
		name   string
		before string
		after  string
	}{
		{name: "type", before: before.Type, after: after.Type},
		{name: "sizeBytes", before: fmt.Sprintf("%d", before.SizeBytes), after: fmt.Sprintf("%d", after.SizeBytes)},
		{name: "createdUtc", before: formatOptionalTime(before.CreatedUTC), after: formatOptionalTime(after.CreatedUTC)},
		{name: "modifiedUtc", before: before.ModifiedUTC.UTC().Format(time.RFC3339), after: after.ModifiedUTC.UTC().Format(time.RFC3339)},
		{name: "permissions", before: formatOptionalString(before.Permissions), after: formatOptionalString(after.Permissions)},
	}

	changes := make([]FieldChange, 0, len(fields))
	for _, field := range fields {
		if field.before == field.after {
			continue
		}
		changes = append(changes, FieldChange{
			Field:  field.name,
			Before: field.before,
			After:  field.after,
		})
	}
	return changes
}

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return "null"
	}
	return t.UTC().Format(time.RFC3339)
}

func formatOptionalString(s *string) string {
	if s == nil {
		return "null"
	}
	return *s
}

func printEntryList(name string, entries []Entry) {
	if len(entries) == 0 {
		return
	}

	fmt.Println()
	fmt.Println(name + ":")
	for _, entry := range entries {
		fmt.Printf("  %s\n", entry.Path)
	}
}
