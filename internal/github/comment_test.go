package github

import (
	"strings"
	"testing"
	"time"
)

func TestFormatDeploymentComment(t *testing.T) {
	c := &Commenter{}

	info := DeploymentInfo{
		FQDN: "myapp-pr123.eastus.azurecontainer.io",
		Services: []ServiceInfo{
			{Name: "frontend", Ports: []int32{80}},
			{Name: "api", Ports: []int32{3000, 3001}},
		},
		DeployTime: 45 * time.Second,
	}

	body := c.formatDeploymentComment(info)

	if !strings.Contains(body, commentMarker) {
		t.Error("expected comment to contain marker")
	}

	if !strings.Contains(body, "http://myapp-pr123.eastus.azurecontainer.io") {
		t.Error("expected comment to contain URL")
	}

	if !strings.Contains(body, "`frontend`") {
		t.Error("expected comment to contain frontend service")
	}

	if !strings.Contains(body, "`api`") {
		t.Error("expected comment to contain api service")
	}

	if !strings.Contains(body, "45s") {
		t.Error("expected comment to contain deploy time")
	}
}

func TestFormatTeardownComment(t *testing.T) {
	c := &Commenter{}

	info := DeploymentInfo{
		FQDN: "myapp-pr123.eastus.azurecontainer.io",
		Services: []ServiceInfo{
			{Name: "frontend", Ports: []int32{80}},
		},
		DeployTime: 45 * time.Second,
	}

	body := c.formatTeardownComment(info)

	if !strings.Contains(body, commentMarker) {
		t.Error("expected comment to contain marker")
	}

	if !strings.Contains(body, "torn down") {
		t.Error("expected comment to mention teardown")
	}

	if !strings.Contains(body, "~~**URL:**") {
		t.Error("expected URL to be struck through")
	}

	if !strings.Contains(body, "`frontend`") {
		t.Error("expected comment to preserve service info")
	}
}

func TestNewCommenter(t *testing.T) {
	c := NewCommenter("fake-token", "owner", "repo")

	if c == nil {
		t.Error("expected commenter to be non-nil")
	}

	if c.owner != "owner" {
		t.Errorf("expected owner to be 'owner', got %s", c.owner)
	}

	if c.repo != "repo" {
		t.Errorf("expected repo to be 'repo', got %s", c.repo)
	}
}
