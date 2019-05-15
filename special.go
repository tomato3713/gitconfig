package gitconfig

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// User takes git user name
func (c *Config) User() (string, error) {
	return c.Get("user.name")
}

// Email takes git email
func (c *Config) Email() (string, error) {
	return c.Get("user.email")
}

// GitHubToken takes API token for GitHub
func (c *Config) GitHubToken() (string, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		return token, nil
	}
	return c.Get("github.token")
}

// GitHubUser detects user name of GitHub from various informations
func (c *Config) GitHubUser(host string) (string, error) {
	if host == "" {
		host = os.Getenv("GITHUB_HOST")
		if host == "" {
			host = "github.com"
		}
	}
	if user := os.Getenv("GITHUB_USER"); user != "" {
		return user, nil
	}
	if user, err := c.Get(fmt.Sprintf("credential.https://%s.username", host)); err == nil {
		return user, nil
	}
	if user, err := getGHUserFromHub(host); err == nil {
		return user, nil
	}
	if user, err := c.Get("github.user"); err == nil {
		return user, nil
	}
	if email, err := c.Email(); err == nil {
		apiHost := os.Getenv("GITHUB_API")
		if apiHost == "" {
			apiHost = host
		}
		if apiHost == "github.com" {
			apiHost = "api.github.com"
		}
		token, _ := c.GitHubToken()
		if user, err := getGHUserFromGHAPI(apiHost, email, token); err == nil {
			return user, nil
		}
	}
	return c.Get("user.username")
}

func getGHUserFromHub(host string) (string, error) {
	xdgHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		xdgHome = filepath.Join(home, ".config")
	}
	f, err := os.Open(filepath.Join(xdgHome, "hub"))
	if err != nil {
		return "", err
	}
	defer f.Close()

	var s map[string][]struct {
		Protocol, User string
	}
	if err := yaml.NewDecoder(f).Decode(&s); err != nil {
		return "", err
	}
	var u string
	for _, st := range s[host] {
		if st.Protocol == "https" {
			u = st.User
			break
		}
	}
	if u != "" {
		return u, nil
	}
	return "", fmt.Errorf("user not found from hub config")
}

func getGHUserFromGHAPI(apiHost, email, token string) (string, error) {
	v := url.Values{}
	v.Add("q", fmt.Sprintf("%s in:email", email))
	v.Add("per_page", "2")
	u := &url.URL{
		Scheme:   "https",
		Host:     apiHost,
		Path:     "/search/users",
		RawQuery: v.Encode(),
	}
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	req.Header.Add("User-Agent", fmt.Sprintf("Songmu-gitconfig/%s", version))
	if token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", token))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var s struct {
		TotalCount int `json:"total_count"`
		Items      []struct {
			Login string
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return "", err
	}
	switch s.TotalCount {
	case 0:
		return "", fmt.Errorf("no users found from GitHub")
	case 1:
		return s.Items[0].Login, nil
	}
	return getGHUserFromGHCommit(apiHost, email, token)
}

func getGHUserFromGHCommit(apiHost, email, token string) (string, error) {
	v := url.Values{}
	v.Add("q", fmt.Sprintf("author-email:%s", email))
	v.Add("sort", "author-date")
	v.Add("per_page", "1")
	u := &url.URL{
		Scheme:   "https",
		Host:     apiHost,
		Path:     "/search/commits",
		RawQuery: v.Encode(),
	}
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	req.Header.Add("User-Agent", fmt.Sprintf("Songmu-gitconfig/%s", version))
	req.Header.Add("Accept", "application/vnd.github.cloak-preview")
	if token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", token))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var s struct {
		TotalCount int `json:"total_count"`
		Items      []struct {
			Author struct {
				Login string
			}
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return "", err
	}
	if s.TotalCount < 1 {
		return "", fmt.Errorf("no commits found")
	}
	return s.Items[0].Author.Login, nil
}
