package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	return nil, errors.New("create is not implemented yet")
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
	return nil, errors.New("diff is not implemented yet")
}

func diffSnapshots(before, after *SnapshotFile) DiffResult {
	return DiffResult{}
}

func printDiff(result DiffResult) {
	fmt.Println("Summary:")
	fmt.Printf("  Added:    %d\n", len(result.Added))
	fmt.Printf("  Deleted:  %d\n", len(result.Deleted))
	fmt.Printf("  Modified: %d\n", len(result.Modified))
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
