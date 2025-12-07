package github

import (
	"strings"
	"testing"
	"time"
)

func TestFormatDeploymentComment(t *testing.T) {
	t.Parallel()

	info := DeploymentInfo{
		FQDN: "myapp-pr123.eastus.azurecontainer.io",
		Services: []ServiceInfo{
			{Name: "frontend", Ports: []int32{80}},
			{Name: "api", Ports: []int32{3000, 3001}},
		},
		DeployTime: 45 * time.Second,
	}

	body := formatDeploymentComment(info)

	if !strings.Contains(body, commentMarker) {
		t.Error("expected comment to contain marker")
	}

	if !strings.Contains(body, "https://myapp-pr123.eastus.azurecontainer.io") {
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
	t.Parallel()

	info := DeploymentInfo{
		FQDN: "myapp-pr123.eastus.azurecontainer.io",
		Services: []ServiceInfo{
			{Name: "frontend", Ports: []int32{80}},
		},
		DeployTime: 45 * time.Second,
	}

	body := formatTeardownComment(info)

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
	t.Parallel()

	c := NewCommenter("fake-token", "owner", "repo")

	if c == nil {
		t.Fatal("expected commenter to be non-nil")
	}

	if c.owner != "owner" {
		t.Errorf("expected owner to be 'owner', got %s", c.owner)
	}

	if c.repo != "repo" {
		t.Errorf("expected repo to be 'repo', got %s", c.repo)
	}

	if c.tokenSource == nil {
		t.Error("expected token source to be non-nil")
	}
}

func TestFormatPorts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		ports []int32
		want  string
	}{
		{"empty", nil, "none"},
		{"single", []int32{80}, "80"},
		{"multiple", []int32{80, 443, 8080}, "80, 443, 8080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatPorts(tt.ports)
			if got != tt.want {
				t.Errorf("formatPorts(%v) = %q, want %q", tt.ports, got, tt.want)
			}
		})
	}
}
