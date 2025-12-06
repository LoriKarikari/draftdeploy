package github

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

type Commenter struct {
	tokenSource oauth2.TokenSource
	owner       string
	repo        string
}

type DeploymentInfo struct {
	FQDN       string
	Services   []ServiceInfo
	DeployTime time.Duration
	CommitSHA  string
}

type ServiceInfo struct {
	Name  string
	Ports []int32
}

const commentMarker = "<!-- draftdeploy -->"

func NewCommenter(token, owner, repo string) *Commenter {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	return &Commenter{
		tokenSource: ts,
		owner:       owner,
		repo:        repo,
	}
}

func (c *Commenter) getClient(ctx context.Context) *github.Client {
	tc := oauth2.NewClient(ctx, c.tokenSource)
	return github.NewClient(tc)
}

func (c *Commenter) PostDeployment(ctx context.Context, prNumber int, info DeploymentInfo) error {
	client := c.getClient(ctx)

	existingID, existingBody, err := c.findExistingComment(ctx, client, prNumber)
	if err != nil {
		return fmt.Errorf("failed to find existing comment: %w", err)
	}

	body := formatDeploymentComment(info, existingBody)

	if existingID != 0 {
		_, _, err = client.Issues.EditComment(ctx, c.owner, c.repo, existingID, &github.IssueComment{
			Body: github.String(body),
		})
		if err != nil {
			return fmt.Errorf("failed to update comment: %w", err)
		}
		return nil
	}

	_, _, err = client.Issues.CreateComment(ctx, c.owner, c.repo, prNumber, &github.IssueComment{
		Body: github.String(body),
	})
	if err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}

	return nil
}

func (c *Commenter) PostTeardown(ctx context.Context, prNumber int) error {
	client := c.getClient(ctx)

	existingID, existingBody, err := c.findExistingComment(ctx, client, prNumber)
	if err != nil {
		return fmt.Errorf("failed to find existing comment: %w", err)
	}

	body := formatTeardownFromExisting(existingBody)

	if existingID != 0 {
		_, _, err = client.Issues.EditComment(ctx, c.owner, c.repo, existingID, &github.IssueComment{
			Body: github.String(body),
		})
		if err != nil {
			return fmt.Errorf("failed to update comment: %w", err)
		}
		return nil
	}

	_, _, err = client.Issues.CreateComment(ctx, c.owner, c.repo, prNumber, &github.IssueComment{
		Body: github.String(body),
	})
	if err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}

	return nil
}

func (c *Commenter) findExistingComment(ctx context.Context, client *github.Client, prNumber int) (int64, string, error) {
	comments, _, err := client.Issues.ListComments(ctx, c.owner, c.repo, prNumber, nil)
	if err != nil {
		return 0, "", err
	}

	for _, comment := range comments {
		if comment.Body != nil && strings.Contains(*comment.Body, commentMarker) {
			if comment.ID != nil {
				return *comment.ID, *comment.Body, nil
			}
		}
	}

	return 0, "", nil
}

func formatDeploymentComment(info DeploymentInfo, existingBody string) string {
	var sb strings.Builder
	sb.Grow(1024)

	shortSHA := info.CommitSHA
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}

	sb.WriteString(commentMarker)
	sb.WriteString("\n### 🚀 DraftDeploy\n\n")
	sb.WriteString("| Name | Status | Preview | Updated |\n")
	sb.WriteString("|------|--------|---------|--------|\n")
	fmt.Fprintf(&sb, "| **%s** | ✅ Ready | [Visit](http://%s) | %s |\n\n", shortSHA, info.FQDN, formatDuration(info.DeployTime))

	history := extractDeployHistory(existingBody)
	if len(history) > 0 {
		sb.WriteString("<details><summary>Previous deployments</summary>\n\n")
		sb.WriteString("| Name | Status | Updated |\n")
		sb.WriteString("|------|--------|--------|\n")
		for _, h := range history {
			sb.WriteString(h)
			sb.WriteString("\n")
		}
		sb.WriteString("\n</details>\n\n")
	}

	return sb.String()
}

func formatDuration(d time.Duration) string {
	s := int(d.Seconds())
	if s < 60 {
		return fmt.Sprintf("%ds ago", s)
	}
	m := s / 60
	if m < 60 {
		return fmt.Sprintf("%dm ago", m)
	}
	h := m / 60
	return fmt.Sprintf("%dh ago", h)
}

func extractDeployHistory(body string) []string {
	if body == "" {
		return nil
	}

	var history []string
	lines := strings.Split(body, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "| **") && strings.Contains(line, "Ready") {
			parts := strings.Split(line, "|")
			if len(parts) >= 4 {
				name := strings.TrimSpace(parts[1])
				status := strings.TrimSpace(parts[2])
				history = append(history, fmt.Sprintf("| %s | %s | - |", name, status))
			}
		}
	}

	existingHistory := extractPreviousHistory(body)
	history = append(history, existingHistory...)

	if len(history) > 9 {
		history = history[:9]
	}

	return history
}

func extractPreviousHistory(body string) []string {
	var history []string
	lines := strings.Split(body, "\n")
	inHistory := false

	for _, line := range lines {
		if strings.Contains(line, "Previous deployments") {
			inHistory = true
			continue
		}
		if inHistory && strings.HasPrefix(line, "| **") {
			history = append(history, line)
		}
		if inHistory && strings.HasPrefix(line, "</details>") {
			break
		}
	}

	return history
}

func formatTeardownFromExisting(existingBody string) string {
	var sb strings.Builder
	sb.Grow(512)

	sb.WriteString(commentMarker)
	sb.WriteString("\n### 🛑 DraftDeploy\n\n")
	sb.WriteString("| Name | Status | Preview | Updated |\n")
	sb.WriteString("|------|--------|---------|--------|\n")
	sb.WriteString("| - | ⏹️ Removed | - | just now |\n")

	history := extractDeployHistory(existingBody)
	if len(history) > 0 {
		sb.WriteString("\n<details><summary>Previous deployments</summary>\n\n")
		sb.WriteString("| Name | Status | Updated |\n")
		sb.WriteString("|------|--------|--------|\n")
		for _, h := range history {
			sb.WriteString(h)
			sb.WriteString("\n")
		}
		sb.WriteString("\n</details>\n")
	}

	return sb.String()
}

func formatPorts(ports []int32) string {
	if len(ports) == 0 {
		return "none"
	}
	strs := make([]string, len(ports))
	for i, p := range ports {
		strs[i] = fmt.Sprintf("%d", p)
	}
	return strings.Join(strs, ", ")
}
