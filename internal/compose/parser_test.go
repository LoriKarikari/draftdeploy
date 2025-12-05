package compose

import (
	"os"
	"path/filepath"
	"testing"
)

const composeFileName = "docker-compose.yml"

func loadTestCompose(t *testing.T, yaml string) *Project {
	t.Helper()
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, composeFileName)
	if err := os.WriteFile(composePath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	project, err := Load(composePath)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	return project
}

func TestLoad(t *testing.T) {
	t.Parallel()

	yaml := `
services:
  frontend:
    image: nginx:alpine
    ports:
      - "80:80"
  api:
    build: ./api
    ports:
      - "3000:3000"
    environment:
      - DATABASE_URL=postgres://localhost/db
    depends_on:
      - postgres
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: myapp
      POSTGRES_USER: user
      POSTGRES_PASSWORD: pass
`

	project := loadTestCompose(t, yaml)
	names := project.GetServiceNames()
	if len(names) != 3 {
		t.Errorf("expected 3 services, got %d", len(names))
	}
}

func TestGetServiceNames_Sorted(t *testing.T) {
	t.Parallel()

	yaml := `
services:
  zebra:
    image: nginx
  alpha:
    image: nginx
  middle:
    image: nginx
`

	project := loadTestCompose(t, yaml)
	names := project.GetServiceNames()

	expected := []string{"alpha", "middle", "zebra"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("expected names[%d] = %s, got %s", i, expected[i], name)
		}
	}
}

func TestGetExposedPorts(t *testing.T) {
	t.Parallel()

	yaml := `
services:
  web:
    image: nginx
    ports:
      - "80:80"
      - "443:443"
`

	project := loadTestCompose(t, yaml)
	ports := project.GetExposedPorts("web")
	if len(ports) != 2 {
		t.Errorf("expected 2 ports, got %d", len(ports))
	}
}

func TestGetExposedPorts_InvalidPort(t *testing.T) {
	t.Parallel()

	yaml := `
services:
  web:
    image: nginx
    ports:
      - "80:80"
`

	project := loadTestCompose(t, yaml)

	ports := project.GetExposedPorts("nonexistent")
	if ports != nil {
		t.Error("expected nil for nonexistent service")
	}
}

func TestGetServiceImage(t *testing.T) {
	t.Parallel()

	yaml := `
services:
  web:
    image: nginx:alpine
  api:
    build: ./api
`

	project := loadTestCompose(t, yaml)

	if img := project.GetServiceImage("web"); img != "nginx:alpine" {
		t.Errorf("expected nginx:alpine, got %s", img)
	}

	if img := project.GetServiceImage("api"); img != "" {
		t.Errorf("expected empty string for build service, got %s", img)
	}

	if img := project.GetServiceImage("nonexistent"); img != "" {
		t.Errorf("expected empty string for nonexistent service, got %s", img)
	}
}
