package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/go-github/v60/github"
)

const (
	owner = "midnattsol"
	repo  = "docker-sweep"
)

// Release represents a GitHub release.
type Release struct {
	TagName string
	Body    string
	Assets  []Asset
}

// Asset represents a release asset.
type Asset struct {
	Name        string
	DownloadURL string
}

// CurrentVersion should be set by cmd.Execute.
var CurrentVersion = "dev"

// CheckForUpdate checks if a newer release is available.
func CheckForUpdate(ctx context.Context) (*Release, bool, error) {
	client := github.NewClient(nil)

	rel, _, err := client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		var ghErr *github.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response != nil && ghErr.Response.StatusCode == http.StatusNotFound {
			// No published release yet.
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to check for updates: %w", err)
	}

	latest := strings.TrimPrefix(rel.GetTagName(), "v")
	current := strings.TrimPrefix(CurrentVersion, "v")
	if latest == current || current == "dev" {
		return nil, false, nil
	}

	r := &Release{
		TagName: rel.GetTagName(),
		Body:    rel.GetBody(),
		Assets:  make([]Asset, 0, len(rel.Assets)),
	}

	for _, a := range rel.Assets {
		r.Assets = append(r.Assets, Asset{
			Name:        a.GetName(),
			DownloadURL: a.GetBrowserDownloadURL(),
		})
	}

	return r, true, nil
}

// GetAssetForPlatform returns the tar.gz URL for this OS/arch.
func (r *Release) GetAssetForPlatform() (string, error) {
	expected := fmt.Sprintf("docker-sweep-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	for _, a := range r.Assets {
		if a.Name == expected {
			return a.DownloadURL, nil
		}
	}
	return "", fmt.Errorf("no release found for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// DownloadAndInstall downloads the release asset and replaces the current binary.
func DownloadAndInstall(ctx context.Context, downloadURL string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "docker-sweep-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, "docker-sweep.tar.gz")
	if err := downloadFile(ctx, downloadURL, archivePath); err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}

	binaryPath := filepath.Join(tmpDir, "docker-sweep")
	if err := extractBinary(archivePath, binaryPath); err != nil {
		return fmt.Errorf("failed to extract update: %w", err)
	}

	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return fmt.Errorf("failed to set executable bit: %w", err)
	}

	backupPath := execPath + ".old"
	_ = os.Remove(backupPath)

	if err := os.Rename(execPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary (%s): %w", execPath, err)
	}

	if err := copyFile(binaryPath, execPath); err != nil {
		_ = os.Rename(backupPath, execPath)
		return fmt.Errorf("failed to install new binary (%s): %w", execPath, err)
	}

	if err := os.Chmod(execPath, 0o755); err != nil {
		_ = os.Rename(backupPath, execPath)
		return fmt.Errorf("failed to set binary permissions: %w", err)
	}

	_ = os.Remove(backupPath)
	return nil
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractBinary(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == "docker-sweep" {
			out, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer out.Close()

			_, err = io.Copy(out, tr)
			return err
		}
	}

	return fmt.Errorf("docker-sweep binary not found in archive")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
