package prx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

type GHCLI struct{}

func NewGHCLI() GHCLI {
	return GHCLI{}
}

func (g GHCLI) CurrentLogin() (string, error) {
	var user struct {
		Login string `json:"login"`
	}
	if err := g.apiJSON(&user, "user"); err != nil {
		return "", err
	}
	return user.Login, nil
}

func (g GHCLI) GetPullRequest(number int) (PullRequest, error) {
	var pr struct {
		Number  int    `json:"number"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
	}
	if err := g.apiJSON(&pr, "repos/{owner}/{repo}/pulls/"+strconv.Itoa(number)); err != nil {
		return PullRequest{}, err
	}
	return PullRequest{Number: pr.Number, Body: pr.Body, URL: pr.HTMLURL}, nil
}

func (g GHCLI) UpdatePullRequestBody(number int, body string) (PullRequest, error) {
	var pr struct {
		Number  int    `json:"number"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
	}
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return PullRequest{}, err
	}
	if err := g.apiJSON(&pr, "repos/{owner}/{repo}/pulls/"+strconv.Itoa(number), "--method", "PATCH", "--input", "-",
		"--header", "Content-Type: application/json", string(payload)); err != nil {
		return PullRequest{}, err
	}
	return PullRequest{Number: pr.Number, Body: pr.Body, URL: pr.HTMLURL}, nil
}

func (g GHCLI) ListComments(number int) ([]Comment, error) {
	type rawComment struct {
		ID        int64  `json:"id"`
		Body      string `json:"body"`
		HTMLURL   string `json:"html_url"`
		UpdatedAt string `json:"updated_at"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	var pages [][]rawComment
	if err := g.apiJSON(&pages, "repos/{owner}/{repo}/issues/"+strconv.Itoa(number)+"/comments?per_page=100", "--paginate", "--slurp"); err != nil {
		return nil, err
	}
	var comments []Comment
	for _, page := range pages {
		for _, c := range page {
			comments = append(comments, Comment{ID: c.ID, Body: c.Body, URL: c.HTMLURL, Author: c.User.Login, UpdatedAt: c.UpdatedAt})
		}
	}
	return comments, nil
}

func (g GHCLI) UpdateComment(id int64, body string) (Comment, error) {
	var raw struct {
		ID        int64  `json:"id"`
		Body      string `json:"body"`
		HTMLURL   string `json:"html_url"`
		UpdatedAt string `json:"updated_at"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return Comment{}, err
	}
	if err := g.apiJSON(&raw, "repos/{owner}/{repo}/issues/comments/"+strconv.FormatInt(id, 10), "--method", "PATCH", "--input", "-",
		"--header", "Content-Type: application/json", string(payload)); err != nil {
		return Comment{}, err
	}
	return Comment{ID: raw.ID, Body: raw.Body, URL: raw.HTMLURL, Author: raw.User.Login, UpdatedAt: raw.UpdatedAt}, nil
}

func (g GHCLI) CreateComment(number int, body string) (Comment, error) {
	var raw struct {
		ID        int64  `json:"id"`
		Body      string `json:"body"`
		HTMLURL   string `json:"html_url"`
		UpdatedAt string `json:"updated_at"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
	}
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return Comment{}, err
	}
	if err := g.apiJSON(&raw, "repos/{owner}/{repo}/issues/"+strconv.Itoa(number)+"/comments", "--method", "POST", "--input", "-",
		"--header", "Content-Type: application/json", string(payload)); err != nil {
		return Comment{}, err
	}
	return Comment{ID: raw.ID, Body: raw.Body, URL: raw.HTMLURL, Author: raw.User.Login, UpdatedAt: raw.UpdatedAt}, nil
}

func (g GHCLI) apiJSON(out any, endpoint string, args ...string) error {
	ghArgs := []string{"api", endpoint}
	var stdin []byte
	for i := 0; i < len(args); i++ {
		if i == len(args)-1 && looksLikeJSON(args[i]) {
			stdin = []byte(args[i])
			continue
		}
		ghArgs = append(ghArgs, args[i])
	}
	cmd := exec.Command("gh", ghArgs...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh api failed: %w: %s", err, stderr.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), out); err != nil {
		return fmt.Errorf("decode gh api response: %w", err)
	}
	return nil
}

func looksLikeJSON(value string) bool {
	return len(value) > 0 && (value[0] == '{' || value[0] == '[')
}
