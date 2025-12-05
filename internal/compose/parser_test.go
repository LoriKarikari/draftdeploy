package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
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

	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	project, err := Load(composePath)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	names := project.GetServiceNames()
	if len(names) != 3 {
		t.Errorf("expected 3 services, got %d", len(names))
	}
}

func TestGetExposedPorts(t *testing.T) {
	yaml := `
services:
  web:
    image: nginx
    ports:
      - "80:80"
      - "443:443"
`

	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	project, err := Load(composePath)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	ports := project.GetExposedPorts("web")
	if len(ports) != 2 {
		t.Errorf("expected 2 ports, got %d", len(ports))
	}
}

func TestGetServiceImage(t *testing.T) {
	yaml := `
services:
  web:
    image: nginx:alpine
  api:
    build: ./api
`

	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	project, err := Load(composePath)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

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

func TestHasBuildConfig(t *testing.T) {
	yaml := `
services:
  web:
    image: nginx:alpine
  api:
    build: ./api
`

	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	project, err := Load(composePath)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if project.HasBuildConfig("web") {
		t.Error("expected web to not have build config")
	}

	if !project.HasBuildConfig("api") {
		t.Error("expected api to have build config")
	}

	if project.HasBuildConfig("nonexistent") {
		t.Error("expected nonexistent to not have build config")
	}
}
