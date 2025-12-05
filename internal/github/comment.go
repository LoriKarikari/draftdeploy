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
	client *github.Client
	owner  string
	repo   string
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
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)

	return &Commenter{
		client: client,
		owner:  owner,
		repo:   repo,
	}
}

func (c *Commenter) PostDeployment(ctx context.Context, prNumber int, info DeploymentInfo) error {
	body := c.formatDeploymentComment(info)

	existingID, err := c.findExistingComment(ctx, prNumber)
	if err != nil {
		return fmt.Errorf("failed to find existing comment: %w", err)
	}

	if existingID != 0 {
		_, _, err = c.client.Issues.EditComment(ctx, c.owner, c.repo, existingID, &github.IssueComment{
			Body: github.String(body),
		})
		if err != nil {
			return fmt.Errorf("failed to update comment: %w", err)
		}
		return nil
	}

	_, _, err = c.client.Issues.CreateComment(ctx, c.owner, c.repo, prNumber, &github.IssueComment{
		Body: github.String(body),
	})
	if err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}

	return nil
}

func (c *Commenter) PostTeardown(ctx context.Context, prNumber int) error {
	body := c.formatTeardownComment()

	existingID, err := c.findExistingComment(ctx, prNumber)
	if err != nil {
		return fmt.Errorf("failed to find existing comment: %w", err)
	}

	if existingID != 0 {
		_, _, err = c.client.Issues.EditComment(ctx, c.owner, c.repo, existingID, &github.IssueComment{
			Body: github.String(body),
		})
		if err != nil {
			return fmt.Errorf("failed to update comment: %w", err)
		}
		return nil
	}

	_, _, err = c.client.Issues.CreateComment(ctx, c.owner, c.repo, prNumber, &github.IssueComment{
		Body: github.String(body),
	})
	if err != nil {
		return fmt.Errorf("failed to create comment: %w", err)
	}

	return nil
}

func (c *Commenter) findExistingComment(ctx context.Context, prNumber int) (int64, error) {
	comments, _, err := c.client.Issues.ListComments(ctx, c.owner, c.repo, prNumber, nil)
	if err != nil {
		return 0, err
	}

	for _, comment := range comments {
		if comment.Body != nil && strings.Contains(*comment.Body, commentMarker) {
			return *comment.ID, nil
		}
	}

	return 0, nil
}

func (c *Commenter) formatDeploymentComment(info DeploymentInfo) string {
	var sb strings.Builder

	sb.WriteString(commentMarker)
	sb.WriteString("\n## DraftDeploy Preview\n\n")
	sb.WriteString(fmt.Sprintf("**URL:** http://%s\n\n", info.FQDN))

	if len(info.Services) > 0 {
		sb.WriteString("**Services:**\n")
		for _, svc := range info.Services {
			ports := make([]string, len(svc.Ports))
			for i, p := range svc.Ports {
				ports[i] = fmt.Sprintf("%d", p)
			}
			sb.WriteString(fmt.Sprintf("- `%s` (ports: %s)\n", svc.Name, strings.Join(ports, ", ")))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("**Deploy time:** %s\n", info.DeployTime.Round(time.Second)))

	return sb.String()
}

func (c *Commenter) formatTeardownComment() string {
	var sb strings.Builder

	sb.WriteString(commentMarker)
	sb.WriteString("\n## DraftDeploy Preview\n\n")
	sb.WriteString("Preview environment has been torn down.\n")

	return sb.String()
}
