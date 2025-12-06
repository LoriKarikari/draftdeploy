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
	body := formatDeploymentComment(info)
	return c.postComment(ctx, prNumber, body)
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

func (c *Commenter) postComment(ctx context.Context, prNumber int, body string) error {
	client := c.getClient(ctx)

	existingID, _, err := c.findExistingComment(ctx, client, prNumber)
	if err != nil {
		return fmt.Errorf("failed to find existing comment: %w", err)
	}

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
	sb.Grow(512)

	sb.WriteString(commentMarker)
	sb.WriteString("\n### ✅ Preview Ready\n\n")
	fmt.Fprintf(&sb, "| Name | Link |\n")
	sb.WriteString("|------|------|\n")
	fmt.Fprintf(&sb, "| **Preview** | [Visit Preview](http://%s) |\n\n", info.FQDN)

	if len(info.Services) > 0 {
		sb.WriteString("<details><summary><b>Services</b></summary>\n\n")
		sb.WriteString("| Service | Ports |\n|---------|-------|\n")
		for _, svc := range info.Services {
			fmt.Fprintf(&sb, "| %s | %s |\n", svc.Name, formatPorts(svc.Ports))
		}
		sb.WriteString("\n</details>\n\n")
	}

	fmt.Fprintf(&sb, "---\n*Deployed in %s by [DraftDeploy](https://github.com/LoriKarikari/draftdeploy)*\n", info.DeployTime.Round(time.Second))

	return sb.String()
}

func formatTeardownFromExisting(existingBody string) string {
	if existingBody == "" {
		return commentMarker + "\n### 🛑 Preview Removed\n\nThe preview environment has been torn down.\n\n---\n*Powered by [DraftDeploy](https://github.com/LoriKarikari/draftdeploy)*\n"
	}

	lines := strings.Split(existingBody, "\n")
	var sb strings.Builder
	sb.Grow(len(existingBody) + 100)

	for i, line := range lines {
		if strings.Contains(line, "✅ Preview Ready") {
			sb.WriteString("### 🛑 Preview Removed\n")
			continue
		}
		if strings.Contains(line, "[Visit Preview]") {
			sb.WriteString("| **Preview** | ~~Removed~~ |\n")
			continue
		}
		if strings.Contains(line, "Deployed in") {
			continue
		}
		sb.WriteString(line)
		if i < len(lines)-1 {
			sb.WriteString("\n")
		}
	}

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
