package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"ai-bot-chain/backend/internal/domain"
)

type GitHubImportResult struct {
	RepoURL string        `json:"repoUrl"`
	Ref     string        `json:"ref"`
	Path    string        `json:"path"`
	Count   int           `json:"count"`
	Created []domain.Skill `json:"created"`
}

// ImportSkillsFromGitHub downloads a GitHub repo (zip) and imports text-like files as Skills.
// Supported GitHub URLs:
// - https://github.com/{owner}/{repo}
// - https://github.com/{owner}/{repo}/tree/{ref}/{optional/path}
func (s *ChatService) ImportSkillsFromGitHub(ctx context.Context, botID string, githubURL string) (GitHubImportResult, error) {
	owner, repo, ref, subPath, err := parseGitHubRepoURL(githubURL)
	if err != nil {
		return GitHubImportResult{}, err
	}

	if ref == "" {
		ref, _ = githubDefaultBranch(ctx, owner, repo)
		if ref == "" {
			ref = "main"
		}
	}

	zipURL := fmt.Sprintf("https://codeload.github.com/%s/%s/zip/%s", owner, repo, url.PathEscape(ref))
	rawZip, err := fetchBytes(ctx, zipURL, 12<<20) // 12MB
	if err != nil {
		// fallback to master if default guess fails
		if ref == "main" {
			zipURL2 := fmt.Sprintf("https://codeload.github.com/%s/%s/zip/%s", owner, repo, "master")
			if raw2, err2 := fetchBytes(ctx, zipURL2, 12<<20); err2 == nil {
				rawZip = raw2
				ref = "master"
				err = nil
			}
		}
	}
	if err != nil {
		return GitHubImportResult{}, err
	}

	created, err := importSkillsFromZip(ctx, s, botID, rawZip, subPath)
	if err != nil {
		return GitHubImportResult{}, err
	}
	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	return GitHubImportResult{
		RepoURL: repoURL,
		Ref:     ref,
		Path:    subPath,
		Count:   len(created),
		Created: created,
	}, nil
}

func parseGitHubRepoURL(raw string) (owner string, repo string, ref string, subPath string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", "", errors.New("github url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", "", errors.New("invalid github url")
	}
	if u.Host != "github.com" && u.Host != "www.github.com" {
		return "", "", "", "", errors.New("only github.com urls are supported")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", "", "", errors.New("github url must include owner/repo")
	}
	owner = parts[0]
	repo = parts[1]
	repo = strings.TrimSuffix(repo, ".git")

	// /{owner}/{repo}/tree/{ref}/{path...}
	if len(parts) >= 4 && parts[2] == "tree" {
		ref = parts[3]
		if len(parts) > 4 {
			subPath = strings.Join(parts[4:], "/")
		}
	}

	subPath = strings.Trim(strings.TrimSpace(subPath), "/")
	return owner, repo, ref, subPath, nil
}

func githubDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	api := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("github api http %d", resp.StatusCode)
	}
	var data struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}
	return strings.TrimSpace(data.DefaultBranch), nil
}

func fetchBytes(ctx context.Context, u string, max int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %d fetching %s", resp.StatusCode, u)
	}
	return io.ReadAll(io.LimitReader(resp.Body, max))
}

func importSkillsFromZip(ctx context.Context, s *ChatService, botID string, rawZip []byte, subPath string) ([]domain.Skill, error) {
	zr, err := zip.NewReader(bytes.NewReader(rawZip), int64(len(rawZip)))
	if err != nil {
		return nil, errors.New("invalid zip from github")
	}

	subPath = strings.Trim(strings.TrimSpace(subPath), "/")
	subPath = path.Clean("/" + subPath)
	if subPath == "/." || subPath == "/" {
		subPath = ""
	} else {
		subPath = strings.TrimPrefix(subPath, "/")
	}

	const maxFileBytes = 512 << 10
	const maxTotalBytes = 4 << 20
	total := 0

	var created []domain.Skill
	seenName := map[string]int{}

	for _, f := range zr.File {
		if f == nil || f.FileInfo().IsDir() {
			continue
		}
		if f.UncompressedSize64 > maxFileBytes {
			continue
		}
		name := strings.TrimPrefix(f.Name, strings.Split(f.Name, "/")[0]+"/") // strip top-level dir
		name = strings.Trim(name, "/")
		if name == "" {
			continue
		}
		// filter by subPath if provided
		if subPath != "" {
			if !strings.HasPrefix(name, subPath+"/") && name != subPath {
				continue
			}
		}
		base := path.Base(name)
		if strings.HasPrefix(base, ".") {
			continue
		}

		ext := strings.ToLower(filepath.Ext(base))
		if ext != ".txt" && ext != ".md" && ext != ".json" && ext != ".yaml" && ext != ".yml" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(rc, maxFileBytes))
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		total += len(b)
		if total > maxTotalBytes {
			break
		}
		content := strings.TrimSpace(string(b))
		if content == "" {
			continue
		}

		// Name: relative path without extension (keep folder structure).
		skillName := strings.TrimSuffix(name, filepath.Ext(name))
		skillName = strings.Trim(skillName, "/")
		if skillName == "" {
			skillName = strings.TrimSuffix(base, filepath.Ext(base))
		}
		if skillName == "" {
			skillName = "skill-" + time.Now().Format("20060102150405")
		}
		if n := seenName[skillName]; n > 0 {
			seenName[skillName] = n + 1
			skillName = fmt.Sprintf("%s-%d", skillName, n+1)
		} else {
			seenName[skillName] = 1
		}

		sk, err := s.CreateSkill(botID, domain.Skill{
			Name:        skillName,
			Filename:    name,
			ContentType: "text/plain",
			Content:     content,
			SizeBytes:   len(b),
		})
		if err != nil {
			return nil, err
		}
		created = append(created, sk)
	}
	return created, nil
}

