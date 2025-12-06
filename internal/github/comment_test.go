package github

import (
	"strings"
	"testing"
	"time"
)

const (
	errMarker   = "expected comment to contain marker"
	errTeardown = "expected comment to mention teardown"
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
		CommitSHA:  "abc1234567890",
	}

	body := formatDeploymentComment(info, "")

	if !strings.Contains(body, commentMarker) {
		t.Error(errMarker)
	}

	if !strings.Contains(body, "Visit Preview") {
		t.Error("expected comment to contain preview link")
	}

	if !strings.Contains(body, "myapp-pr123.eastus.azurecontainer.io") {
		t.Error("expected comment to contain URL")
	}

	if !strings.Contains(body, "frontend") {
		t.Error("expected comment to contain frontend service")
	}

	if !strings.Contains(body, "api") {
		t.Error("expected comment to contain api service")
	}

	if !strings.Contains(body, "45") {
		t.Error("expected comment to contain deploy time")
	}

	if !strings.Contains(body, "DraftDeploy") {
		t.Error("expected comment to contain branding")
	}

	if !strings.Contains(body, "abc1234") {
		t.Error("expected comment to contain commit SHA")
	}

	if !strings.Contains(body, "Deployment History") {
		t.Error("expected comment to contain deployment history")
	}
}

func TestFormatDeploymentCommentWithHistory(t *testing.T) {
	t.Parallel()

	existing := `<!-- draftdeploy -->
### ✅ Preview Ready

| Name | Link |
|------|------|
| **Preview** | [Visit Preview](http://test.io) |

<details><summary><b>Deployment History</b></summary>

| Commit | Status | Time |
|--------|--------|------|
| ` + "`abc1234`" + ` | ✅ Ready | 30s |

</details>`

	info := DeploymentInfo{
		FQDN:       "test.io",
		DeployTime: 45 * time.Second,
		CommitSHA:  "def5678",
	}

	body := formatDeploymentComment(info, existing)

	if !strings.Contains(body, "def5678") {
		t.Error("expected new commit in history")
	}

	if !strings.Contains(body, "abc1234") {
		t.Error("expected old commit preserved in history")
	}
}

func TestFormatTeardownFromExisting(t *testing.T) {
	t.Parallel()

	existing := `<!-- draftdeploy -->
### ✅ Preview Ready

| Name | Link |
|------|------|
| **Preview** | [Visit Preview](http://myapp-pr123.eastus.azurecontainer.io) |

<details><summary><b>Services</b></summary>

| Service | Ports |
|---------|-------|
| frontend | 80 |

</details>

---
*Deployed in 45s by [DraftDeploy](https://github.com/LoriKarikari/draftdeploy)*`

	body := formatTeardownFromExisting(existing)

	if !strings.Contains(body, commentMarker) {
		t.Error(errMarker)
	}

	if !strings.Contains(body, "Removed") {
		t.Error("expected comment to show removed status")
	}

	if !strings.Contains(body, "DraftDeploy") {
		t.Error("expected comment to contain branding")
	}
}

func TestFormatTeardownFromExistingEmpty(t *testing.T) {
	t.Parallel()

	body := formatTeardownFromExisting("")

	if !strings.Contains(body, commentMarker) {
		t.Error(errMarker)
	}

	if !strings.Contains(body, "torn down") {
		t.Error(errTeardown)
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
