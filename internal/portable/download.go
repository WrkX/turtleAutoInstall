package portable

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const userAgent = "tortoise-wow-portable"

func (j *Job) downloadFile(url, outFile, ua string) error {
	if ua == "" {
		ua = userAgent
	}
	if err := j.mkdirAll(filepath.Dir(outFile)); err != nil {
		return err
	}
	partial := outFile + ".partial"
	_ = os.Remove(partial)
	req, err := http.NewRequestWithContext(j.Ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", ua)
	res, err := j.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("download %s: HTTP %s", url, res.Status)
	}
	f, err := os.Create(partial)
	if err != nil {
		return err
	}
	var written int64
	buf := make([]byte, 32*1024)
	nextLog := int64(8 << 20)
	for {
		if err := j.errIfDone(); err != nil {
			f.Close()
			_ = os.Remove(partial)
			return err
		}
		n, readErr := res.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				f.Close()
				_ = os.Remove(partial)
				return err
			}
			written += int64(n)
			if written >= nextLog {
				j.logf("  %d MB", written/(1<<20))
				nextLog += 8 << 20
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			_ = os.Remove(partial)
			return readErr
		}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(partial)
		return err
	}
	_ = os.Remove(outFile)
	if err := os.Rename(partial, outFile); err != nil {
		_ = os.Remove(partial)
		return err
	}
	return nil
}

type releaseAsset struct {
	URL     string
	Name    string
	TagName string
}

func (j *Job) resolveReleaseAsset(pattern, overrideKey, fallbackName, kind string) (releaseAsset, error) {
	if override := j.get(overrideKey, ""); override != "" {
		name := path.Base(override)
		if name == "" || name == "." || name == "/" {
			name = fallbackName
		}
		return releaseAsset{URL: override, Name: name}, nil
	}
	repo := j.get("TORTOISE_WOW_REPO", "WrkX/tortoise-wow")
	release := j.get("TORTOISE_WOW_RELEASE", "latest")
	apiBase := "https://api.github.com/repos/" + repo + "/releases"
	apiURL := apiBase + "/latest"
	if release != "latest" {
		apiURL = apiBase + "/tags/" + release
	}
	j.logf("Resolving %s from %s release %s", kind, repo, release)
	req, err := http.NewRequestWithContext(j.Ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return releaseAsset{}, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return releaseAsset{}, fmt.Errorf("could not fetch release info from %s: %w", apiURL, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return releaseAsset{}, fmt.Errorf("could not fetch release info from %s: HTTP %s (set %s to override)", apiURL, res.Status, overrideKey)
	}
	var body struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return releaseAsset{}, err
	}
	var match struct {
		Name               string
		BrowserDownloadURL string
	}
	for _, a := range body.Assets {
		ok, _ := path.Match(pattern, a.Name)
		if ok {
			match.Name = a.Name
			match.BrowserDownloadURL = a.BrowserDownloadURL
			break
		}
	}
	if match.Name == "" {
		return releaseAsset{}, fmt.Errorf("release %s has no %s asset", body.TagName, pattern)
	}
	j.logf("Using %s: %s", body.TagName, match.Name)
	return releaseAsset{URL: match.BrowserDownloadURL, Name: match.Name, TagName: body.TagName}, nil
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func looksLikeZip(path string) bool {
	st, err := os.Stat(path)
	if err != nil || st.Size() < 64 {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var mag [2]byte
	if _, err := io.ReadFull(f, mag[:]); err != nil {
		return false
	}
	return mag[0] == 0x50 && mag[1] == 0x4B
}

func writeMarker(path, value string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(value+"\n"), 0644)
}

func readTrimmed(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
