package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go-syncit/internal/paths"

	"github.com/spf13/cobra"
)

const (
	upgradeRepoOwner = "incureforce"
	upgradeRepoName  = "syncit"
)

type githubRelease struct {
	TagName string             `json:"tag_name"`
	Assets  []githubAssetEntry `json:"assets"`
}

type githubAssetEntry struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func init() {
	rootCmd.AddCommand(cmdUpgrade())
}

func cmdUpgrade() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Download and install the latest release",
		RunE: func(cmd *cobra.Command, args []string) error {
			tag, assetName, assetURL, err := fetchLatestReleaseInfo()
			if err != nil {
				return err
			}

			fmt.Printf("Found latest release: %s (%s)\n", tag, assetName)
			archive, err := downloadReleaseAsset(assetURL)
			if err != nil {
				return err
			}

			binary, err := extractBinaryFromArchive(assetName, archive)
			if err != nil {
				return err
			}

			if err := installVersionedLayout(binary, tag); err != nil {
				return err
			}

			fmt.Println("Upgrade complete.")
			return nil
		},
	}
}

func fetchLatestReleaseInfo() (tag string, assetName string, assetURL string, err error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", upgradeRepoOwner, upgradeRepoName)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "syncit-upgrade")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("query latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", "", "", fmt.Errorf("github latest release request failed: %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", "", "", fmt.Errorf("decode latest release response: %w", err)
	}
	if rel.TagName == "" {
		return "", "", "", errors.New("latest release tag is empty")
	}

	platformSuffix := expectedPlatformArchiveSuffix()
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, platformSuffix) {
			return rel.TagName, a.Name, a.BrowserDownloadURL, nil
		}
	}

	return "", "", "", fmt.Errorf("no release asset found for %s/%s with suffix %q", runtime.GOOS, runtime.GOARCH, platformSuffix)
}

func expectedPlatformArchiveSuffix() string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("-%s-%s.zip", runtime.GOOS, runtime.GOARCH)
	}
	return fmt.Sprintf("-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
}

func downloadReleaseAsset(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "syncit-upgrade")

	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("download release asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download release asset failed: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read release asset: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("release asset is empty")
	}
	return data, nil
}

func extractBinaryFromArchive(assetName string, archive []byte) ([]byte, error) {
	if strings.HasSuffix(assetName, ".zip") {
		return extractBinaryFromZIP(archive)
	}
	if strings.HasSuffix(assetName, ".tar.gz") {
		return extractBinaryFromTarGz(archive)
	}
	return nil, fmt.Errorf("unsupported release asset format: %s", assetName)
}

func extractBinaryFromZIP(archive []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(f.Name)
		if !strings.HasSuffix(strings.ToLower(name), ".exe") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open file in zip: %w", err)
		}
		defer rc.Close()
		bin, err := io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("read file from zip: %w", err)
		}
		return bin, nil
	}
	return nil, errors.New("no executable binary found in zip archive")
}

func extractBinaryFromTarGz(archive []byte) ([]byte, error) {
	gzr, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open tar.gz archive: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("read tar.gz archive: %w", err)
		}
		if hdr == nil || hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := filepath.Base(hdr.Name)
		if name == "" || strings.Contains(name, ".") {
			continue
		}
		bin, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read binary from tar.gz: %w", err)
		}
		return bin, nil
	}
	return nil, errors.New("no executable binary found in tar.gz archive")
}

func installVersionedLayout(binary []byte, tag string) error {
	syncitDir, err := paths.SyncitDir()
	if err != nil {
		return fmt.Errorf("resolve syncit dir: %w", err)
	}

	binaryName := fmt.Sprintf("%s-%s-%s", upgradeRepoName, runtime.GOOS, runtime.GOARCH)
	launcherName := "syncit"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
		launcherName += ".exe"
	}

	versionDir := filepath.Join(syncitDir, tag)
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		return fmt.Errorf("create version dir: %w", err)
	}

	versionedPath := filepath.Join(versionDir, binaryName)
	if err := os.WriteFile(versionedPath, binary, 0o755); err != nil {
		return fmt.Errorf("write versioned binary: %w", err)
	}

	binDir := filepath.Join(syncitDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}

	launcherPath := filepath.Join(binDir, launcherName)
	if err := os.Remove(launcherPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove existing launcher: %w", err)
	}

	if runtime.GOOS == "windows" {
		if err := os.WriteFile(launcherPath, binary, 0o755); err != nil {
			return fmt.Errorf("write launcher executable: %w", err)
		}
		fmt.Printf("Installed %s to %s\n", tag, versionedPath)
		fmt.Printf("Updated launcher: %s\n", launcherPath)
		return nil
	}

	if err := os.Symlink(versionedPath, launcherPath); err != nil {
		return fmt.Errorf("create symlink %s -> %s: %w", launcherPath, versionedPath, err)
	}

	fmt.Printf("Installed %s to %s\n", tag, versionedPath)
	fmt.Printf("Updated symlink: %s -> %s\n", launcherPath, versionedPath)
	return nil
}
