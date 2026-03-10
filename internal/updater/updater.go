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
