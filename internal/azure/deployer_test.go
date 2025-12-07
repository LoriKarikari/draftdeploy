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
		ResourceGroup:   "test-rg",
		Name:            "test-app",
		EnvironmentName: "test-env",
		Location:        "eastus",
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

	if config.Name != "test-app" {
		t.Errorf("expected name test-app, got %s", config.Name)
	}

	if config.ResourceGroup != "test-rg" {
		t.Errorf("expected resource group test-rg, got %s", config.ResourceGroup)
	}

	if config.Location != "eastus" {
		t.Errorf("expected location eastus, got %s", config.Location)
	}

	if config.EnvironmentName != "test-env" {
		t.Errorf("expected environment name test-env, got %s", config.EnvironmentName)
	}

	if len(config.Containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(config.Containers))
	}
}

func TestBuildContainerApp(t *testing.T) {
	config := DeployConfig{
		Name:     "test-app",
		Location: "eastus",
		Containers: []ContainerConfig{
			{
				Name:  "web",
				Image: "nginx:alpine",
				Ports: []int32{80},
				Environment: map[string]string{
					"FOO": "bar",
				},
			},
		},
	}

	app := buildContainerApp(config, "/subscriptions/xxx/resourceGroups/rg/providers/Microsoft.App/managedEnvironments/env")

	if app.Location == nil || *app.Location != "eastus" {
		t.Error("expected location eastus")
	}

	if app.Properties == nil {
		t.Fatal("expected properties to be non-nil")
	}

	if app.Properties.Template == nil {
		t.Fatal("expected template to be non-nil")
	}

	if len(app.Properties.Template.Containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(app.Properties.Template.Containers))
	}

	container := app.Properties.Template.Containers[0]
	if container.Name == nil || *container.Name != "web" {
		t.Error("expected container name web")
	}

	if container.Image == nil || *container.Image != "nginx:alpine" {
		t.Error("expected image nginx:alpine")
	}

	// Check ingress
	if app.Properties.Configuration == nil || app.Properties.Configuration.Ingress == nil {
		t.Fatal("expected ingress to be configured")
	}

	ingress := app.Properties.Configuration.Ingress
	if ingress.External == nil || !*ingress.External {
		t.Error("expected external ingress")
	}

	if ingress.TargetPort == nil || *ingress.TargetPort != 80 {
		t.Errorf("expected target port 80, got %v", ingress.TargetPort)
	}

	// Check scale
	if app.Properties.Template.Scale == nil {
		t.Fatal("expected scale to be configured")
	}

	if app.Properties.Template.Scale.MinReplicas == nil || *app.Properties.Template.Scale.MinReplicas != 0 {
		t.Error("expected min replicas 0")
	}

	if app.Properties.Template.Scale.MaxReplicas == nil || *app.Properties.Template.Scale.MaxReplicas != 1 {
		t.Error("expected max replicas 1")
	}
}

func TestBuildEnvVars(t *testing.T) {
	env := map[string]string{
		"B_VAR": "b",
		"A_VAR": "a",
		"C_VAR": "c",
	}

	vars := buildEnvVars(env)

	if len(vars) != 3 {
		t.Fatalf("expected 3 env vars, got %d", len(vars))
	}

	// Should be sorted alphabetically
	if vars[0].Name == nil || *vars[0].Name != "A_VAR" {
		t.Error("expected first var to be A_VAR")
	}
	if vars[1].Name == nil || *vars[1].Name != "B_VAR" {
		t.Error("expected second var to be B_VAR")
	}
	if vars[2].Name == nil || *vars[2].Name != "C_VAR" {
		t.Error("expected third var to be C_VAR")
	}
}

func TestBuildEnvVarsEmpty(t *testing.T) {
	vars := buildEnvVars(nil)
	if vars != nil {
		t.Errorf("expected nil for empty env, got %v", vars)
	}

	vars = buildEnvVars(map[string]string{})
	if vars != nil {
		t.Errorf("expected nil for empty map, got %v", vars)
	}
}
