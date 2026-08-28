package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type latestMsg struct {
	tag string
	err error
}

func fetchLatestTag(repo string) (string, error) {
	if repo == "" {
		repo = "WrkX/tortoise-wow"
	}
	client := &http.Client{Timeout: 8 * time.Second}
	url := "https://api.github.com/repos/" + repo + "/releases/latest"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "tortoise-wow-portable")
	req.Header.Set("Accept", "application/vnd.github+json")

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub %s", res.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.TagName == "" {
		return "", fmt.Errorf("no tag on latest release")
	}
	return body.TagName, nil
}

func checkLatest(root string) tea.Cmd {
	return func() tea.Msg {
		env := loadEnv(root)
		tag, err := fetchLatestTag(envOr(env, "TORTOISE_WOW_REPO", "WrkX/tortoise-wow"))
		return latestMsg{tag: tag, err: err}
	}
}
