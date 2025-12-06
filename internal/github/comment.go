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

	existingID, _, err := c.findExistingComment(ctx, client, prNumber)
	if err != nil {
		return fmt.Errorf("failed to find existing comment: %w", err)
	}

	body := formatDeploymentComment(info)

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

	existingID, _, err := c.findExistingComment(ctx, client, prNumber)
	if err != nil {
		return fmt.Errorf("failed to find existing comment: %w", err)
	}

	body := formatTeardown()

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

func formatDeploymentComment(info DeploymentInfo) string {
	var sb strings.Builder
	sb.Grow(1024)

	shortSHA := info.CommitSHA
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}

	sb.WriteString(commentMarker)
	sb.WriteString("\n| Name | Status | Preview | Updated |\n")
	sb.WriteString("|------|--------|---------|--------|\n")
	fmt.Fprintf(&sb, "| **%s** | ✅ Ready | [Visit](http://%s) | %s |\n", shortSHA, info.FQDN, formatDuration(info.DeployTime))

	sb.WriteString("\n---\n*Powered by [DraftDeploy](https://github.com/LoriKarikari/draftdeploy)*\n")

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

func formatTeardown() string {
	var sb strings.Builder
	sb.Grow(512)

	sb.WriteString(commentMarker)
	sb.WriteString("\n| Name | Status | Preview | Updated |\n")
	sb.WriteString("|------|--------|---------|--------|\n")
	sb.WriteString("| - | ⏹️ Removed | - | just now |\n")

	sb.WriteString("\n---\n*Powered by [DraftDeploy](https://github.com/LoriKarikari/draftdeploy)*\n")

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
