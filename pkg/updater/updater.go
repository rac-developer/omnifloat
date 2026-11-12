package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	// GitHubLatestURL is the official API endpoint for the latest release.
	GitHubLatestURL = "https://api.github.com/repos/rac-developer/omnifloat/releases/latest"
	// UserAgent identifies OmniFloat updater in HTTP requests.
	UserAgent = "OmniFloat-NativeLauncher/1.0"
)

// GitHubAsset represents a downloadable file in a GitHub Release.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

// GitHubRelease represents the release object returned by GitHub API.
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt string        `json:"published_at"`
	Assets      []GitHubAsset `json:"assets"`
}

// UpdateInfo holds details about an available update.
type UpdateInfo struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseNotes   string `json:"releaseNotes"`
	DownloadURL    string `json:"downloadURL"`
	AssetName      string `json:"assetName"`
	ReleaseURL     string `json:"releaseURL"`
	AssetSize      int64  `json:"assetSize"`
}

// Updater handles checking and applying software updates.
type Updater struct {
	currentVersion string
	client         *http.Client
}

// NewUpdater creates a new updater for the given current version.
func NewUpdater(currentVersion string) *Updater {
	return &Updater{
		currentVersion: strings.TrimPrefix(currentVersion, "v"),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CheckForUpdates queries the GitHub API and determines if a newer release is published.
func (u *Updater) CheckForUpdates() (*UpdateInfo, error) {
	req, err := http.NewRequest("GET", GitHubLatestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build update request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to contact GitHub release server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// No release has been published yet
		return &UpdateInfo{
			Available:      false,
			CurrentVersion: u.currentVersion,
			LatestVersion:  u.currentVersion,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from release API: %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release metadata: %w", err)
	}

	latestVer := strings.TrimPrefix(release.TagName, "v")
	isNewer := isVersionNewer(u.currentVersion, latestVer)

	info := &UpdateInfo{
		Available:      isNewer,
		CurrentVersion: u.currentVersion,
		LatestVersion:  latestVer,
		ReleaseNotes:   release.Body,
		ReleaseURL:     release.HTMLURL,
	}

	// Match appropriate asset for current OS and architecture
	matchedAsset := u.selectAsset(release.Assets)
	if matchedAsset != nil {
		info.DownloadURL = matchedAsset.BrowserDownloadURL
		info.AssetName = matchedAsset.Name
		info.AssetSize = matchedAsset.Size
	}

	return info, nil
}

// CheckAndNotify runs a non-blocking check against GitHub Releases at startup
// and displays a native OS alert dialog if a newer version is available.
func (u *Updater) CheckAndNotify(ctx context.Context) {
	go func() {
		// Wait 3 seconds after startup to not compete for initial rendering bandwidth
		time.Sleep(3 * time.Second)

		info, err := u.CheckForUpdates()
		if err != nil {
			return // Silently skip network error on background check
		}

		if info != nil && info.Available && ctx != nil {
			wailsRuntime.EventsEmit(ctx, "updater:available", info)
			selection, dlgErr := wailsRuntime.MessageDialog(ctx, wailsRuntime.MessageDialogOptions{
				Type:          wailsRuntime.QuestionDialog,
				Title:         "Nueva versión disponible — OmniFloat",
				Message:       fmt.Sprintf("Se encontró una nueva versión de OmniFloat en el repositorio (v%s).\nVersión actual instalada: v%s\n\n¿Deseas abrir la página de descarga?", info.LatestVersion, info.CurrentVersion),
				Buttons:       []string{"Yes", "No"},
				DefaultButton: "Yes",
				CancelButton:  "No",
			})
			if dlgErr == nil && (selection == "Yes" || selection == "Sí" || selection == "OK") {
				targetURL := info.DownloadURL
				if targetURL == "" {
					targetURL = info.ReleaseURL
				}
				if targetURL != "" {
					wailsRuntime.BrowserOpenURL(ctx, targetURL)
				}
			}
		}
	}()
}

// DownloadAsset downloads the update file to the destination path.
func (u *Updater) DownloadAsset(downloadURL, targetFile string) error {
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download update file, HTTP status %d", resp.StatusCode)
	}

	out, err := os.Create(targetFile)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (u *Updater) selectAsset(assets []GitHubAsset) *GitHubAsset {
	osTarget := runtime.GOOS // "windows" or "linux"
	archTarget := runtime.GOARCH

	for _, a := range assets {
		lower := strings.ToLower(a.Name)
		if osTarget == "windows" {
			if strings.HasSuffix(lower, ".exe") && (strings.Contains(lower, archTarget) || strings.Contains(lower, "installer")) {
				return &a
			}
		} else if osTarget == "linux" {
			if strings.HasSuffix(lower, ".deb") || strings.HasSuffix(lower, ".tar.gz") {
				return &a
			}
		}
	}

	// Fallback to first asset if any
	if len(assets) > 0 {
		return &assets[0]
	}
	return nil
}

// isVersionNewer compares two semver strings (e.g. "1.0.0" and "1.1.0").
func isVersionNewer(current, latest string) bool {
	cParts := parseSemver(current)
	lParts := parseSemver(latest)

	for i := 0; i < 3; i++ {
		if lParts[i] > cParts[i] {
			return true
		}
		if lParts[i] < cParts[i] {
			return false
		}
	}
	return false
}

func parseSemver(v string) [3]int {
	var parts [3]int
	chunks := strings.Split(v, ".")
	for i := 0; i < len(chunks) && i < 3; i++ {
		val, _ := strconv.Atoi(chunks[i])
		parts[i] = val
	}
	return parts
}
