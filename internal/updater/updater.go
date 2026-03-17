package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/evilsocket/opensnitch-web/internal/config"
	"github.com/evilsocket/opensnitch-web/internal/version"
	"github.com/evilsocket/opensnitch-web/internal/ws"
)

// ReleaseInfo holds metadata about a GitHub release.
type ReleaseInfo struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
	Body        string `json:"body"`
}

// UpdateStatus is the current state of the updater.
type UpdateStatus struct {
	CurrentVersion  string       `json:"current_version"`
	BuildTime       string       `json:"build_time,omitempty"`
	LatestVersion   string       `json:"latest_version,omitempty"`
	UpdateAvailable bool         `json:"update_available"`
	LastCheck       *time.Time   `json:"last_check,omitempty"`
	Checking        bool         `json:"checking"`
	Downloading     bool         `json:"downloading"`
	Error           string       `json:"error,omitempty"`
	Release         *ReleaseInfo `json:"release,omitempty"`
}

// githubRelease is the JSON response from the GitHub Releases API.
type githubRelease struct {
	TagName     string        `json:"tag_name"`
	PublishedAt string        `json:"published_at"`
	HTMLURL     string        `json:"html_url"`
	Body        string        `json:"body"`
	Assets      []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Updater manages checking for and applying binary self-updates.
type Updater struct {
	cfg        *config.UpdateConfig
	hub        *ws.Hub
	shutdownFn func()

	mu     sync.RWMutex
	status UpdateStatus
	client *http.Client
}

// New creates a new Updater. shutdownFn is called after a successful update
// to trigger a graceful restart (typically sends SIGTERM to the signal channel).
func New(cfg *config.UpdateConfig, hub *ws.Hub, shutdownFn func()) *Updater {
	return &Updater{
		cfg:        cfg,
		hub:        hub,
		shutdownFn: shutdownFn,
		status: UpdateStatus{
			CurrentVersion: version.Version,
			BuildTime:      version.BuildTime,
		},
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Status returns a thread-safe snapshot of the current update status.
func (u *Updater) Status() UpdateStatus {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.status
}

// CheckNow checks GitHub for the latest release. Safe to call from handlers.
func (u *Updater) CheckNow() (*UpdateStatus, error) {
	u.mu.Lock()
	u.status.Checking = true
	u.status.Error = ""
	u.mu.Unlock()

	defer func() {
		u.mu.Lock()
		u.status.Checking = false
		u.mu.Unlock()
	}()

	release, err := u.fetchLatestRelease()
	if err != nil {
		u.mu.Lock()
		u.status.Error = err.Error()
		u.mu.Unlock()
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}

	now := time.Now()
	newer := isNewer(version.Version, release.TagName)

	u.mu.Lock()
	u.status.LatestVersion = release.TagName
	u.status.UpdateAvailable = newer
	u.status.LastCheck = &now
	u.status.Release = &ReleaseInfo{
		TagName:     release.TagName,
		PublishedAt: release.PublishedAt,
		HTMLURL:     release.HTMLURL,
		Body:        release.Body,
	}
	status := u.status
	u.mu.Unlock()

	if newer {
		log.Printf("[updater] Update available: %s → %s", version.Version, release.TagName)
		u.hub.BroadcastEvent(ws.EventUpdateAvailable, map[string]interface{}{
			"latest_version": release.TagName,
			"html_url":       release.HTMLURL,
		})
	}

	return &status, nil
}

// Apply downloads the latest release binary, verifies its checksum,
// atomically replaces the current binary, and triggers a graceful restart.
func (u *Updater) Apply() error {
	u.mu.RLock()
	if !u.status.UpdateAvailable {
		u.mu.RUnlock()
		return fmt.Errorf("no update available")
	}
	u.mu.RUnlock()

	u.mu.Lock()
	u.status.Downloading = true
	u.status.Error = ""
	u.mu.Unlock()

	defer func() {
		u.mu.Lock()
		u.status.Downloading = false
		u.mu.Unlock()
	}()

	// Fetch the release to get asset URLs
	release, err := u.fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("fetch release: %w", err)
	}

	assetName := fmt.Sprintf("opensnitch-web-%s-%s", runtime.GOOS, runtime.GOARCH)

	// Find the binary asset and checksums asset
	var binaryURL, checksumsURL string
	for _, a := range release.Assets {
		switch a.Name {
		case assetName:
			binaryURL = a.BrowserDownloadURL
		case "checksums.txt":
			checksumsURL = a.BrowserDownloadURL
		}
	}

	if binaryURL == "" {
		return fmt.Errorf("no asset found for %s in release %s", assetName, release.TagName)
	}

	// Download and parse checksums
	var expectedHash string
	if checksumsURL != "" {
		expectedHash, err = u.fetchExpectedChecksum(checksumsURL, assetName)
		if err != nil {
			return fmt.Errorf("fetch checksums: %w", err)
		}
	}

	// Determine current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve executable symlinks: %w", err)
	}

	// Download binary to a temp file in the same directory (for atomic rename)
	tmpFile, err := os.CreateTemp(filepath.Dir(execPath), "opensnitch-web-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		// Clean up temp file on failure
		if _, err := os.Stat(tmpPath); err == nil {
			os.Remove(tmpPath)
		}
	}()

	log.Printf("[updater] Downloading %s from %s", assetName, release.TagName)
	resp, err := u.client.Get(binaryURL)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("download binary: HTTP %d", resp.StatusCode)
	}

	// Write to temp file and compute hash simultaneously
	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)
	if _, err := io.Copy(writer, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write binary: %w", err)
	}
	tmpFile.Close()

	// Verify checksum
	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if expectedHash != "" && actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	// Set executable permissions
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("chmod: %w", err)
	}

	// Atomic replacement
	log.Printf("[updater] Replacing %s with new binary", execPath)
	if err := os.Rename(tmpPath, execPath); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}

	log.Printf("[updater] Update applied: %s → %s. Triggering restart.", version.Version, release.TagName)
	u.shutdownFn()
	return nil
}

// StartPeriodicCheck runs the background update checker. Call as a goroutine.
func (u *Updater) StartPeriodicCheck(ctx context.Context) {
	if !u.cfg.Enabled {
		log.Println("[updater] Auto-update check disabled")
		return
	}

	log.Printf("[updater] Periodic check enabled (every %s)", u.cfg.CheckInterval)

	// Initial check after a short delay to let the server start
	select {
	case <-time.After(30 * time.Second):
	case <-ctx.Done():
		return
	}

	if _, err := u.CheckNow(); err != nil {
		log.Printf("[updater] Initial check failed: %v", err)
	}

	ticker := time.NewTicker(u.cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if _, err := u.CheckNow(); err != nil {
				log.Printf("[updater] Periodic check failed: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// fetchLatestRelease calls the GitHub Releases API.
func (u *Updater) fetchLatestRelease() (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", u.cfg.GitHubRepo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &release, nil
}

// fetchExpectedChecksum downloads checksums.txt and finds the hash for the given asset.
func (u *Updater) fetchExpectedChecksum(url, assetName string) (string, error) {
	resp, err := u.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Parse sha256sum format: "hash  filename" or "hash filename"
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == assetName {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no checksum found for %s", assetName)
}

// isNewer returns true if latest is a newer semver than current.
func isNewer(current, latest string) bool {
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	if current == "dev" || current == "" {
		return latest != ""
	}
	if current == latest {
		return false
	}

	cParts := strings.Split(current, ".")
	lParts := strings.Split(latest, ".")

	maxLen := len(cParts)
	if len(lParts) > maxLen {
		maxLen = len(lParts)
	}

	for i := 0; i < maxLen; i++ {
		var c, l int
		if i < len(cParts) {
			c, _ = strconv.Atoi(cParts[i])
		}
		if i < len(lParts) {
			l, _ = strconv.Atoi(lParts[i])
		}
		if l > c {
			return true
		}
		if l < c {
			return false
		}
	}
	return false
}
