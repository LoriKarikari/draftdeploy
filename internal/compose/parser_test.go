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
	if err := os.WriteFile(composePath, []byte(yaml), 0644); err != nil {
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
	if err := os.WriteFile(composePath, []byte(yaml), 0644); err != nil {
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
