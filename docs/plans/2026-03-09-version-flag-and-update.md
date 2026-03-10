# `cubit --version` + `cubit update` Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `--version` root flag and a self-updating `cubit update` command that downloads the latest release from GitHub.

**Architecture:** `--version` uses Cobra's built-in version support. `cubit update` lives in `internal/updater/` — calls GitHub Releases API, compares semver, downloads + extracts tar.gz, atomically replaces the running binary.

**Tech Stack:** Go stdlib only (`net/http`, `archive/tar`, `compress/gzip`, `encoding/json`, `os`, `runtime`)

---

### Task 1: Add `--version` flag to root command

**Files:**
- Modify: `cmd/root.go:19-22`

**Step 1: Add Version to rootCmd**

In `cmd/root.go`, add the `Version` field to the root command definition:

```go
var rootCmd = &cobra.Command{
	Use:     "cubit",
	Short:   "Control plane for a single agent instance",
	Long:    "Cubit manages identity, sessions, tasks, and memory for an agent.",
	Version: Version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
```

**Step 2: Set version template to match existing format**

Add this line inside `init()` in `root.go`, before any `AddCommand` calls:

```go
rootCmd.SetVersionTemplate(fmt.Sprintf("cubit %s (commit: %s, built: %s)\n", Version, Commit, Date))
```

**Step 3: Verify**

Run:
```bash
go build -o cubit . && ./cubit --version
```
Expected: `cubit dev (commit: none, built: unknown)`

Run:
```bash
./cubit version
```
Expected: same output (existing subcommand still works).

**Step 4: Commit**

```bash
git add cmd/root.go
git commit -m "feat: add --version flag to root command"
```

---

### Task 2: Create `internal/updater/updater.go` — GitHub release fetcher

**Files:**
- Create: `internal/updater/updater.go`

**Step 1: Write the test**

Create `internal/updater/updater_test.go`:

```go
package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchLatestRelease(t *testing.T) {
	release := ghRelease{
		TagName: "v1.2.3",
		Assets: []ghAsset{
			{Name: "cubit_darwin_arm64.tar.gz", DownloadURL: "https://example.com/cubit_darwin_arm64.tar.gz"},
			{Name: "cubit_linux_amd64.tar.gz", DownloadURL: "https://example.com/cubit_linux_amd64.tar.gz"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	rel, err := fetchRelease(srv.URL)
	if err != nil {
		t.Fatalf("fetchRelease: %v", err)
	}
	if rel.TagName != "v1.2.3" {
		t.Errorf("tag = %q, want v1.2.3", rel.TagName)
	}
	if len(rel.Assets) != 2 {
		t.Errorf("assets = %d, want 2", len(rel.Assets))
	}
}

func TestFindAsset(t *testing.T) {
	assets := []ghAsset{
		{Name: "cubit_darwin_arm64.tar.gz", DownloadURL: "https://example.com/darwin"},
		{Name: "cubit_linux_amd64.tar.gz", DownloadURL: "https://example.com/linux"},
		{Name: "checksums.txt", DownloadURL: "https://example.com/checksums"},
	}

	asset, err := findAsset(assets, "darwin", "arm64")
	if err != nil {
		t.Fatalf("findAsset: %v", err)
	}
	if asset.Name != "cubit_darwin_arm64.tar.gz" {
		t.Errorf("name = %q, want cubit_darwin_arm64.tar.gz", asset.Name)
	}

	_, err = findAsset(assets, "windows", "amd64")
	if err == nil {
		t.Error("expected error for windows/amd64, got nil")
	}
}

func TestNeedsUpdate(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
	}{
		{"dev", "v1.0.0", true},
		{"v1.0.0", "v1.0.0", false},
		{"v1.0.0", "v1.1.0", true},
		{"1.0.0", "v1.0.0", false}, // strip v prefix
	}
	for _, tt := range tests {
		got := needsUpdate(tt.current, tt.latest)
		if got != tt.want {
			t.Errorf("needsUpdate(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/seanochang/dev/projects/agents/cubit && go test ./internal/updater/ -v
```
Expected: FAIL — package doesn't exist yet.

**Step 3: Write the implementation**

Create `internal/updater/updater.go`:

```go
package updater

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const defaultAPIURL = "https://api.github.com/repos/SeanoChang/cubit/releases/latest"

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

func fetchRelease(url string) (*ghRelease, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &rel, nil
}

func findAsset(assets []ghAsset, goos, goarch string) (*ghAsset, error) {
	target := fmt.Sprintf("cubit_%s_%s.tar.gz", goos, goarch)
	for _, a := range assets {
		if a.Name == target {
			return &a, nil
		}
	}
	var names []string
	for _, a := range assets {
		if strings.HasSuffix(a.Name, ".tar.gz") {
			names = append(names, a.Name)
		}
	}
	return nil, fmt.Errorf("no asset for %s/%s; available: %s", goos, goarch, strings.Join(names, ", "))
}

func needsUpdate(current, latest string) bool {
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")
	if current == "dev" || current == "" {
		return true
	}
	return current != latest
}

func downloadAndExtract(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if filepath.Base(hdr.Name) == "cubit" && hdr.Typeflag == tar.TypeReg {
			out, err := os.CreateTemp(filepath.Dir(destPath), "cubit-update-*")
			if err != nil {
				return fmt.Errorf("temp file: %w", err)
			}
			tmpPath := out.Name()
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				os.Remove(tmpPath)
				return fmt.Errorf("extract: %w", err)
			}
			out.Close()
			if err := os.Chmod(tmpPath, 0755); err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf("chmod: %w", err)
			}
			if err := os.Rename(tmpPath, destPath); err != nil {
				os.Remove(tmpPath)
				return fmt.Errorf("replace binary: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("cubit binary not found in archive")
}

// Update checks for a new release and replaces the current binary.
// Returns (newVersion, error). If already up-to-date, newVersion is empty.
func Update(currentVersion string) (string, error) {
	return updateFrom(defaultAPIURL, currentVersion)
}

func updateFrom(apiURL, currentVersion string) (string, error) {
	rel, err := fetchRelease(apiURL)
	if err != nil {
		return "", err
	}
	if !needsUpdate(currentVersion, rel.TagName) {
		return "", nil
	}

	asset, err := findAsset(rel.Assets, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}

	if err := downloadAndExtract(asset.DownloadURL, exe); err != nil {
		return "", err
	}
	return rel.TagName, nil
}
```

**Step 4: Run tests**

```bash
go test ./internal/updater/ -v
```
Expected: all 3 tests PASS.

**Step 5: Commit**

```bash
git add internal/updater/
git commit -m "feat: add internal/updater package for self-update via GitHub releases"
```

---

### Task 3: Add `cubit update` command

**Files:**
- Create: `cmd/update.go`
- Modify: `cmd/root.go` (register command)

**Step 1: Create the command**

Create `cmd/update.go`:

```go
package cmd

import (
	"fmt"

	"github.com/SeanoChang/cubit/internal/updater"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update cubit to the latest release",
	Long:  "Downloads the latest release from GitHub and replaces the current binary.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("current: %s\nchecking for updates...\n", Version)

		newVersion, err := updater.Update(Version)
		if err != nil {
			return fmt.Errorf("update failed: %w", err)
		}
		if newVersion == "" {
			fmt.Println("already up-to-date.")
			return nil
		}

		fmt.Printf("updated: %s → %s\n", Version, newVersion)
		return nil
	},
}
```

**Step 2: Register in root.go**

Add to `init()` in `root.go`:

```go
// cubit update
rootCmd.AddCommand(updateCmd)
```

**Step 3: Verify it builds**

```bash
go build -o cubit . && ./cubit update
```
Expected: prints current version, checks GitHub, either updates or says up-to-date.

**Step 4: Commit**

```bash
git add cmd/update.go cmd/root.go
git commit -m "feat: add cubit update command for self-update from GitHub releases"
```

---

### Task 4: End-to-end integration test for updater

**Files:**
- Modify: `internal/updater/updater_test.go`

**Step 1: Add integration test with fake server**

Append to `internal/updater/updater_test.go`:

```go
func TestUpdateFrom_AlreadyCurrent(t *testing.T) {
	release := ghRelease{TagName: "v1.0.0", Assets: []ghAsset{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	newVer, err := updateFrom(srv.URL, "v1.0.0")
	if err != nil {
		t.Fatalf("updateFrom: %v", err)
	}
	if newVer != "" {
		t.Errorf("expected empty version (up-to-date), got %q", newVer)
	}
}

func TestUpdateFrom_NoAssetForPlatform(t *testing.T) {
	release := ghRelease{
		TagName: "v2.0.0",
		Assets: []ghAsset{
			{Name: "cubit_windows_amd64.tar.gz", DownloadURL: "https://example.com/win"},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	_, err := updateFrom(srv.URL, "v1.0.0")
	if err == nil {
		t.Fatal("expected error for missing platform asset")
	}
}
```

**Step 2: Run all tests**

```bash
go test ./internal/updater/ -v
```
Expected: all 5 tests PASS.

**Step 3: Run full test suite**

```bash
go test ./... -count=1
```
Expected: all tests PASS across all packages.

**Step 4: Commit**

```bash
git add internal/updater/updater_test.go
git commit -m "test: add integration tests for updater edge cases"
```
