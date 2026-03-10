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
		{"1.0.0", "v1.0.0", false},
	}
	for _, tt := range tests {
		got := needsUpdate(tt.current, tt.latest)
		if got != tt.want {
			t.Errorf("needsUpdate(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

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
