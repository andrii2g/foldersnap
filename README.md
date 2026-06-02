# Folder Snaphot

`foldersnap` is a simple Go CLI that snapshots folder metadata to JSON and compares two snapshots to show what changed.

Current Version is intentionally simple. The snapshot records lightweight metadata only:

- relative path
- entry type
- size in bytes
- created time in UTC, when available
- modified time in UTC
- permissions (if applicable)

## Build

Run directly:

```bash
go run . create ./my-folder --out snapshot.json
```

Build a binary:

POSIX:

```bash
go build -o foldersnap
./foldersnap create ./my-folder --out snapshot.json
```

PowerShell:

```powershell
go build -o foldersnap.exe
.\foldersnap.exe create .\my-folder --out snapshot.json
```

## Create A Snapshot

```bash
go run . create ./my-folder --out snapshot.json
```

With excludes:

```bash
go run . create ./my-folder --out snapshot.json --exclude .git,node_modules,bin,obj
```

## Compare Two Snapshots

```bash
go run . diff before.json after.json
```

Example output:

```text
Summary:
  Added:    2
  Deleted:  1
  Modified: 3

Added:
  dist/app.js
  logs/app.log

Deleted:
  tmp/old.txt

Modified:
  src/main.go
    sizeBytes: 1200 -> 1472
    modifiedUtc: 2026-06-02T10:00:00Z -> 2026-06-02T10:05:11Z

  deploy.sh
    permissions: 0644 -> 0755
```
