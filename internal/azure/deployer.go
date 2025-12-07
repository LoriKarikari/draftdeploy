package azure

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/appcontainers/armappcontainers/v3"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/cenkalti/backoff/v4"
)

const (
	DefaultCPU      = 0.5
	DefaultMemoryGB = 0.5
	maxRetryTime    = 10 * time.Minute // ACA environment creation can take 5+ minutes
)

type Deployer struct {
	envClient      *armappcontainers.ManagedEnvironmentsClient
	appClient      *armappcontainers.ContainerAppsClient
	rgClient       *armresources.ResourceGroupsClient
	subscriptionID string
}

type DeployConfig struct {
	ResourceGroup   string
	Name            string
	EnvironmentName string
	Location        string
	Containers      []ContainerConfig
}

type ContainerConfig struct {
	Name        string
	Image       string
	Ports       []int32
	Environment map[string]string
	CPU         float64
	MemoryGB    float64
}

func NewDeployer(credential azcore.TokenCredential, subscriptionID string) (*Deployer, error) {
	envClient, err := armappcontainers.NewManagedEnvironmentsClient(subscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create managed environments client: %w", err)
	}

	appClient, err := armappcontainers.NewContainerAppsClient(subscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create container apps client: %w", err)
	}

	rgClient, err := armresources.NewResourceGroupsClient(subscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource groups client: %w", err)
	}

	return &Deployer{
		envClient:      envClient,
		appClient:      appClient,
		rgClient:       rgClient,
		subscriptionID: subscriptionID,
	}, nil
}

func (d *Deployer) ensureResourceGroup(ctx context.Context, name, location string) error {
	operation := func() error {
		_, err := d.rgClient.CreateOrUpdate(ctx, name, armresources.ResourceGroup{
			Location: to.Ptr(location),
		}, nil)
		if err != nil {
			if isPermanentError(err) {
				return backoff.Permanent(err)
			}
			return err
		}
		return nil
	}

	if err := retryWithBackoff(ctx, operation); err != nil {
		return fmt.Errorf("failed to create resource group: %w", err)
	}
	return nil
}

func (d *Deployer) ensureEnvironment(ctx context.Context, resourceGroup, name, location string) (string, error) {
	var envID string

	operation := func() error {
		env := armappcontainers.ManagedEnvironment{
			Location: to.Ptr(location),
			Properties: &armappcontainers.ManagedEnvironmentProperties{
				ZoneRedundant: to.Ptr(false),
			},
		}

		poller, err := d.envClient.BeginCreateOrUpdate(ctx, resourceGroup, name, env, nil)
		if err != nil {
			if isPermanentError(err) {
				return backoff.Permanent(err)
			}
			return fmt.Errorf("failed to create environment: %w", err)
		}

		result, err := poller.PollUntilDone(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to wait for environment: %w", err)
		}

		if result.ID == nil {
			return fmt.Errorf("environment has no ID")
		}
		envID = *result.ID
		return nil
	}

	if err := retryWithBackoff(ctx, operation); err != nil {
		return "", err
	}
	return envID, nil
}

func (d *Deployer) Deploy(ctx context.Context, config DeployConfig) (string, error) {
	if err := d.ensureResourceGroup(ctx, config.ResourceGroup, config.Location); err != nil {
		return "", err
	}

	envName := config.EnvironmentName
	if envName == "" {
		envName = config.Name + "-env"
	}

	envID, err := d.ensureEnvironment(ctx, config.ResourceGroup, envName, config.Location)
	if err != nil {
		return "", err
	}

	containerApp := buildContainerApp(config, envID)

	var result armappcontainers.ContainerAppsClientCreateOrUpdateResponse

	operation := func() error {
		poller, err := d.appClient.BeginCreateOrUpdate(ctx, config.ResourceGroup, config.Name, containerApp, nil)
		if err != nil {
			if isPermanentError(err) {
				return backoff.Permanent(err)
			}
			return fmt.Errorf("failed to create container app: %w", err)
		}

		res, err := poller.PollUntilDone(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to wait for container app: %w", err)
		}
		result = res
		return nil
	}

	if err := retryWithBackoff(ctx, operation); err != nil {
		return "", err
	}

	return extractFQDN(result)
}

func buildContainerApp(config DeployConfig, envID string) armappcontainers.ContainerApp {
	containers := make([]*armappcontainers.Container, 0, len(config.Containers))

	// Find the first exposed port for ingress
	var ingressPort int32 = 80
	for _, c := range config.Containers {
		if len(c.Ports) > 0 {
			ingressPort = c.Ports[0]
			break
		}
	}

	for _, c := range config.Containers {
		envVars := buildEnvVars(c.Environment)

		cpu := c.CPU
		if cpu == 0 {
			cpu = DefaultCPU
		}
		mem := c.MemoryGB
		if mem == 0 {
			mem = DefaultMemoryGB
		}

		containers = append(containers, &armappcontainers.Container{
			Name:  to.Ptr(c.Name),
			Image: to.Ptr(c.Image),
			Env:   envVars,
			Resources: &armappcontainers.ContainerResources{
				CPU:    to.Ptr(cpu),
				Memory: to.Ptr(fmt.Sprintf("%.1fGi", mem)),
			},
		})
	}

	return armappcontainers.ContainerApp{
		Location: to.Ptr(config.Location),
		Properties: &armappcontainers.ContainerAppProperties{
			ManagedEnvironmentID: to.Ptr(envID),
			Configuration: &armappcontainers.Configuration{
				Ingress: &armappcontainers.Ingress{
					External:   to.Ptr(true),
					TargetPort: to.Ptr(ingressPort),
					Transport:  to.Ptr(armappcontainers.IngressTransportMethodAuto),
				},
			},
			Template: &armappcontainers.Template{
				Containers: containers,
				Scale: &armappcontainers.Scale{
					MinReplicas: to.Ptr[int32](0),
					MaxReplicas: to.Ptr[int32](1),
				},
			},
		},
	}
}

func buildEnvVars(env map[string]string) []*armappcontainers.EnvironmentVar {
	if len(env) == 0 {
		return nil
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	envVars := make([]*armappcontainers.EnvironmentVar, 0, len(env))
	for _, k := range keys {
		envVars = append(envVars, &armappcontainers.EnvironmentVar{
			Name:  to.Ptr(k),
			Value: to.Ptr(env[k]),
		})
	}
	return envVars
}

func extractFQDN(result armappcontainers.ContainerAppsClientCreateOrUpdateResponse) (string, error) {
	if result.Properties == nil {
		return "", fmt.Errorf("container app has no properties")
	}
	if result.Properties.Configuration == nil {
		return "", fmt.Errorf("container app has no configuration")
	}
	if result.Properties.Configuration.Ingress == nil {
		return "", fmt.Errorf("container app has no ingress")
	}
	if result.Properties.Configuration.Ingress.Fqdn == nil {
		return "", fmt.Errorf("container app has no FQDN")
	}
	return *result.Properties.Configuration.Ingress.Fqdn, nil
}

func (d *Deployer) Delete(ctx context.Context, resourceGroup, name string) error {
	operation := func() error {
		poller, err := d.appClient.BeginDelete(ctx, resourceGroup, name, nil)
		if err != nil {
			if isPermanentError(err) {
				return backoff.Permanent(err)
			}
			return fmt.Errorf("failed to delete container app: %w", err)
		}

		_, err = poller.PollUntilDone(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to wait for container app deletion: %w", err)
		}
		return nil
	}

	return retryWithBackoff(ctx, operation)
}

func (d *Deployer) DeleteResourceGroup(ctx context.Context, name string) error {
	operation := func() error {
		poller, err := d.rgClient.BeginDelete(ctx, name, nil)
		if err != nil {
			if isPermanentError(err) {
				return backoff.Permanent(err)
			}
			return fmt.Errorf("failed to delete resource group: %w", err)
		}

		_, err = poller.PollUntilDone(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to wait for resource group deletion: %w", err)
		}
		return nil
	}

	return retryWithBackoff(ctx, operation)
}

func retryWithBackoff(ctx context.Context, operation func() error) error {
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.MaxElapsedTime = maxRetryTime
	return backoff.Retry(operation, backoff.WithContext(expBackoff, ctx))
}

func isPermanentError(err error) bool {
	errStr := err.Error()
	permanentErrors := []string{
		"InvalidParameter",
		"InvalidResourceGroup",
		"AuthorizationFailed",
		"InvalidSubscriptionId",
	}
	for _, pe := range permanentErrors {
		if strings.Contains(errStr, pe) {
			return true
		}
	}
	return false
}
