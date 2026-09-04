package portable

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var neededMaps = []string{"dbc", "maps", "vmaps", "mmaps"}

func mapsPresent(dir string) bool {
	for _, name := range neededMaps {
		p := filepath.Join(dir, name)
		st, err := os.Stat(p)
		if err != nil || !st.IsDir() || !dirHasFiles(p) {
			return false
		}
	}
	return true
}

func googleDriveFileID(raw string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`drive\.google\.com/file/d/([A-Za-z0-9_-]+)`),
		regexp.MustCompile(`drive\.google\.com/open\?id=([A-Za-z0-9_-]+)`),
		regexp.MustCompile(`[?&]id=([A-Za-z0-9_-]+)`),
	}
	for _, re := range patterns {
		if m := re.FindStringSubmatch(raw); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

func (j *Job) datanodesDirectURL(key, fileCode string) (string, error) {
	j.logf("Resolving DataNodes direct link for %s", fileCode)
	u, _ := url.Parse("https://datanodes.to/api/file/direct_link")
	q := u.Query()
	q.Set("file_code", fileCode)
	q.Set("key", key)
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(j.Ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := j.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	var parsed struct {
		Status int `json:"status"`
		Result struct {
			URL string `json:"url"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.Status != 200 || parsed.Result.URL == "" {
		return "", fmt.Errorf("DataNodes direct_link failed. Use an API key, or host a raw .zip URL instead")
	}
	return parsed.Result.URL, nil
}

func (j *Job) resolveMapsZipURL() (string, error) {
	override := j.get("TORTOISE_WOW_MAPS_ZIP_URL", "")
	key := j.get("DATANODES_API_KEY", "")
	code := j.get("DATANODES_FILE_CODE", "")
	if code != "" {
		if key == "" {
			return "", fmt.Errorf("DATANODES_FILE_CODE is set but DATANODES_API_KEY is empty (portable.local.env)")
		}
		return j.datanodesDirectURL(key, code)
	}
	re := regexp.MustCompile(`(?i)https?://(?:www\.)?datanodes\.to/([A-Za-z0-9]+)(?:/.*)?$`)
	if m := re.FindStringSubmatch(override); len(m) > 1 {
		pageCode := m[1]
		if pageCode != "d" && pageCode != "pages" && pageCode != "api" {
			if key == "" {
				return "", fmt.Errorf("TORTOISE_WOW_MAPS_ZIP_URL is a DataNodes page. Set DATANODES_API_KEY, or put a direct zip URL in conf/maps-url.txt")
			}
			return j.datanodesDirectURL(key, pageCode)
		}
	}
	if override != "" {
		return override, nil
	}
	if u := readFirstURL(filepath.Join(j.Root, "conf", "maps-url.txt")); u != "" {
		return u, nil
	}
	remote := j.get("MAPS_URL_REMOTE", "https://raw.githubusercontent.com/WrkX/turtleAutoInstall/main/conf/maps-url.txt")
	cached := filepath.Join(j.cacheDir(), "maps-url.txt")
	j.log("Refreshing maps URL from GitHub")
	if err := j.downloadFile(remote, cached, userAgent); err != nil {
		j.warn("could not refresh maps-url.txt (" + err.Error() + "). Using local conf/maps-url.txt")
		return readFirstURL(cached), nil
	}
	return readFirstURL(cached), nil
}

func (j *Job) saveGoogleDrive(fileID, outFile string) error {
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) tortoise-wow-portable"
	candidate := outFile + ".download"
	j.logf("Google Drive file id %s", fileID)
	defer func() {
		_ = os.Remove(candidate)
		_ = os.Remove(candidate + ".partial")
	}()
	candidates := []string{
		"https://drive.usercontent.google.com/download?id=" + fileID + "&export=download&confirm=t",
		"https://drive.google.com/uc?export=download&confirm=t&id=" + fileID,
	}
	for _, u := range candidates {
		j.log("Trying " + u)
		if err := j.downloadFile(u, candidate, ua); err != nil {
			j.warn(err.Error())
			continue
		}
		if looksLikeZip(candidate) {
			_ = os.Remove(outFile)
			return os.Rename(candidate, outFile)
		}
	}
	j.log("Drive served a confirm page (usual for big zips) - retrying with token")
	scan := "https://drive.google.com/uc?export=download&id=" + fileID
	if err := j.downloadFile(scan, candidate, ua); err != nil {
		return err
	}
	if looksLikeZip(candidate) {
		_ = os.Remove(outFile)
		return os.Rename(candidate, outFile)
	}
	htmlb, err := os.ReadFile(candidate)
	if err != nil {
		return err
	}
	html := string(htmlb)
	confirm := "t"
	uuid := ""
	if m := regexp.MustCompile(`name="confirm"\s+value="([^"]+)"`).FindStringSubmatch(html); len(m) > 1 {
		confirm = m[1]
	} else if m := regexp.MustCompile(`confirm=([0-9A-Za-z_-]+)`).FindStringSubmatch(html); len(m) > 1 {
		confirm = m[1]
	}
	if m := regexp.MustCompile(`name="uuid"\s+value="([^"]+)"`).FindStringSubmatch(html); len(m) > 1 {
		uuid = m[1]
	}
	final := "https://drive.usercontent.google.com/download?id=" + fileID + "&export=download&confirm=" + url.QueryEscape(confirm)
	if uuid != "" {
		final += "&uuid=" + url.QueryEscape(uuid)
	}
	j.log("Trying " + final)
	if err := j.downloadFile(final, candidate, ua); err != nil {
		return err
	}
	if !looksLikeZip(candidate) {
		return fmt.Errorf("Google Drive did not return a zip. Share as \"Anyone with the link can view\"")
	}
	_ = os.Remove(outFile)
	return os.Rename(candidate, outFile)
}

func (j *Job) saveMapsURL(raw, outFile string) error {
	j.log("Downloading maps zip")
	j.log(raw)
	if err := j.mkdirAll(filepath.Dir(outFile)); err != nil {
		return err
	}
	if id := googleDriveFileID(raw); id != "" {
		return j.saveGoogleDrive(id, outFile)
	}
	if err := j.downloadFile(raw, outFile, userAgent); err != nil {
		return err
	}
	if !looksLikeZip(outFile) {
		_ = os.Remove(outFile)
		return fmt.Errorf("downloaded file is not a zip (got a webpage). Need a direct zip URL or a public Google Drive share link")
	}
	return nil
}

func findMapRoot(extract string) string {
	ok := func(d string) bool {
		for _, n := range neededMaps {
			if !fileExists(filepath.Join(d, n)) {
				return false
			}
		}
		return true
	}
	if ok(extract) {
		return extract
	}
	nested := filepath.Join(extract, "maps")
	if ok(nested) {
		return nested
	}
	entries, _ := os.ReadDir(extract)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		d := filepath.Join(extract, e.Name())
		if ok(d) {
			return d
		}
		inner := filepath.Join(d, "maps")
		if ok(inner) {
			return inner
		}
	}
	return ""
}

func (j *Job) FetchMaps(force bool) error {
	mapsDir := filepath.Join(j.Root, "maps")
	cache := j.cacheDir()
	if err := j.mkdirAll(cache); err != nil {
		return err
	}
	if err := j.mkdirAll(mapsDir); err != nil {
		return err
	}
	configuredOverride := j.get("TORTOISE_WOW_MAPS_ZIP_URL", "")
	configuredCode := j.get("DATANODES_FILE_CODE", "")
	raw, err := j.resolveMapsZipURL()
	if err != nil {
		return err
	}
	if raw == "" {
		j.log("no maps URL in conf/maps-url.txt - skip. Put a direct https zip there, or TORTOISE_WOW_MAPS_ZIP_URL in portable.local.env.")
		return nil
	}
	sourceIdentity := raw
	if configuredCode != "" {
		sourceIdentity = "datanodes:" + configuredCode
	} else if m := regexp.MustCompile(`(?i)https?://(?:www\.)?datanodes\.to/([A-Za-z0-9]+)(?:/.*)?$`).FindStringSubmatch(configuredOverride); len(m) > 1 {
		sourceIdentity = "datanodes:" + m[1]
	}
	urlHash := sha256Hex(sourceIdentity)
	zipName := path.Base(raw)
	ext := strings.ToLower(filepath.Ext(zipName))
	if ext != ".zip" {
		ext = ".zip"
	}
	zipPath := filepath.Join(cache, "maps-"+urlHash+ext)
	sourceMarker := filepath.Join(j.Root, "data", ".maps-url-sha256")
	installedHash := readTrimmed(sourceMarker)
	if mapsPresent(mapsDir) && !force && installedHash == urlHash {
		j.logf("Maps already at %s (source %s)", mapsDir, urlHash)
		return nil
	}
	if force || !fileExists(zipPath) {
		if err := j.saveMapsURL(raw, zipPath); err != nil {
			return err
		}
	}
	j.log("Unpacking into maps\\...")
	extract := filepath.Join(cache, "maps-extract")
	backup := filepath.Join(cache, "maps-backup-"+urlHash)
	_ = os.RemoveAll(extract)
	if err := os.MkdirAll(extract, 0755); err != nil {
		return err
	}
	var backedUp, installed []string
	keepBackup := false
	defer func() {
		_ = os.RemoveAll(extract)
		if !keepBackup {
			_ = os.RemoveAll(backup)
		}
	}()
	if err := unzip(zipPath, extract); err != nil {
		_ = os.Remove(zipPath)
		return err
	}
	src := findMapRoot(extract)
	if src == "" {
		_ = os.Remove(zipPath)
		return fmt.Errorf("zip unpacked but dbc/maps/vmaps/mmaps were not found")
	}
	if err := os.MkdirAll(backup, 0755); err != nil {
		return err
	}
	rollback := func(cause error) error {
		for _, name := range installed {
			_ = os.RemoveAll(filepath.Join(mapsDir, name))
		}
		for _, name := range backedUp {
			old := filepath.Join(backup, name)
			to := filepath.Join(mapsDir, name)
			if fileExists(old) {
				if err := os.Rename(old, to); err != nil {
					keepBackup = true
					j.warn("could not restore maps\\" + name + " from rollback backup: " + err.Error())
				}
			}
		}
		if keepBackup {
			j.warn("rollback was incomplete; preserving backup at " + backup)
		}
		_ = os.Remove(zipPath)
		return cause
	}
	for _, name := range neededMaps {
		to := filepath.Join(mapsDir, name)
		if fileExists(to) {
			if err := os.Rename(to, filepath.Join(backup, name)); err != nil {
				return rollback(err)
			}
			backedUp = append(backedUp, name)
		}
	}
	for _, name := range neededMaps {
		from := filepath.Join(src, name)
		to := filepath.Join(mapsDir, name)
		if err := os.Rename(from, to); err != nil {
			return rollback(err)
		}
		installed = append(installed, name)
	}
	if !mapsPresent(mapsDir) {
		return rollback(fmt.Errorf("unpack finished but maps\\ is still incomplete"))
	}
	if err := writeMarker(sourceMarker, urlHash); err != nil {
		return err
	}
	j.log("OK: " + mapsDir)
	return nil
}
