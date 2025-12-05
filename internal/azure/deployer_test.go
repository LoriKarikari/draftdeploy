package azure

import (
	"testing"
)

func TestNewDeployer(t *testing.T) {
	cred, err := NewCredential()
	if err != nil {
		t.Skipf("skipping test without Azure credentials: %v", err)
	}

	deployer, err := NewDeployer(cred, "test-subscription")
	if err != nil {
		t.Fatalf("failed to create deployer: %v", err)
	}

	if deployer == nil {
		t.Error("expected deployer to be non-nil")
	}
}

func TestDeployConfig(t *testing.T) {
	config := DeployConfig{
		ResourceGroup: "test-rg",
		Name:          "test-container",
		Location:      "eastus",
		DNSNameLabel:  "test-dns",
		Containers: []ContainerConfig{
			{
				Name:  "web",
				Image: "nginx:alpine",
				Ports: []int32{80},
				Environment: map[string]string{
					"ENV": "test",
				},
				CPU:      0.5,
				MemoryGB: 0.5,
			},
		},
	}

	if config.Name != "test-container" {
		t.Errorf("expected name test-container, got %s", config.Name)
	}

	if config.ResourceGroup != "test-rg" {
		t.Errorf("expected resource group test-rg, got %s", config.ResourceGroup)
	}

	if config.Location != "eastus" {
		t.Errorf("expected location eastus, got %s", config.Location)
	}

	if config.DNSNameLabel != "test-dns" {
		t.Errorf("expected dns label test-dns, got %s", config.DNSNameLabel)
	}

	if len(config.Containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(config.Containers))
	}
}
