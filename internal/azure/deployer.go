package azure

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerinstance/armcontainerinstance/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/cenkalti/backoff/v4"
)

const (
	DefaultCPU      = 0.5
	DefaultMemoryGB = 0.5
	maxRetryTime    = 2 * time.Minute
)

type Deployer struct {
	containerClient *armcontainerinstance.ContainerGroupsClient
	rgClient        *armresources.ResourceGroupsClient
	subscriptionID  string
}

type DeployConfig struct {
	ResourceGroup string
	Name          string
	Location      string
	Containers    []ContainerConfig
	DNSNameLabel  string
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
	containerClient, err := armcontainerinstance.NewContainerGroupsClient(subscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create container groups client: %w", err)
	}

	rgClient, err := armresources.NewResourceGroupsClient(subscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource groups client: %w", err)
	}

	return &Deployer{
		containerClient: containerClient,
		rgClient:        rgClient,
		subscriptionID:  subscriptionID,
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

func (d *Deployer) Deploy(ctx context.Context, config DeployConfig) (string, error) {
	if err := d.ensureResourceGroup(ctx, config.ResourceGroup, config.Location); err != nil {
		return "", err
	}

	containerGroup := buildContainerGroup(config)

	var result armcontainerinstance.ContainerGroupsClientCreateOrUpdateResponse

	operation := func() error {
		poller, err := d.containerClient.BeginCreateOrUpdate(ctx, config.ResourceGroup, config.Name, containerGroup, nil)
		if err != nil {
			if isPermanentError(err) {
				return backoff.Permanent(err)
			}
			return fmt.Errorf("failed to create container group: %w", err)
		}

		res, err := poller.PollUntilDone(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to wait for container group: %w", err)
		}
		result = res
		return nil
	}

	if err := retryWithBackoff(ctx, operation); err != nil {
		return "", err
	}

	return extractFQDN(result)
}

func buildContainerGroup(config DeployConfig) armcontainerinstance.ContainerGroup {
	containers := make([]*armcontainerinstance.Container, 0, len(config.Containers))
	exposedPorts := make([]*armcontainerinstance.Port, 0)

	for _, c := range config.Containers {
		ports := make([]*armcontainerinstance.ContainerPort, 0, len(c.Ports))
		for _, p := range c.Ports {
			ports = append(ports, &armcontainerinstance.ContainerPort{
				Port:     to.Ptr(p),
				Protocol: to.Ptr(armcontainerinstance.ContainerNetworkProtocolTCP),
			})
			exposedPorts = append(exposedPorts, &armcontainerinstance.Port{
				Port:     to.Ptr(p),
				Protocol: to.Ptr(armcontainerinstance.ContainerGroupNetworkProtocolTCP),
			})
		}

		envVars := buildEnvVars(c.Environment)

		cpu := c.CPU
		if cpu == 0 {
			cpu = DefaultCPU
		}
		mem := c.MemoryGB
		if mem == 0 {
			mem = DefaultMemoryGB
		}

		containers = append(containers, &armcontainerinstance.Container{
			Name: to.Ptr(c.Name),
			Properties: &armcontainerinstance.ContainerProperties{
				Image:                to.Ptr(c.Image),
				Ports:                ports,
				EnvironmentVariables: envVars,
				Resources: &armcontainerinstance.ResourceRequirements{
					Requests: &armcontainerinstance.ResourceRequests{
						CPU:        to.Ptr(cpu),
						MemoryInGB: to.Ptr(mem),
					},
				},
			},
		})
	}

	return armcontainerinstance.ContainerGroup{
		Location: to.Ptr(config.Location),
		Properties: &armcontainerinstance.ContainerGroupPropertiesProperties{
			Containers:    containers,
			OSType:        to.Ptr(armcontainerinstance.OperatingSystemTypesLinux),
			RestartPolicy: to.Ptr(armcontainerinstance.ContainerGroupRestartPolicyAlways),
			IPAddress: &armcontainerinstance.IPAddress{
				Type:         to.Ptr(armcontainerinstance.ContainerGroupIPAddressTypePublic),
				Ports:        exposedPorts,
				DNSNameLabel: to.Ptr(config.DNSNameLabel),
			},
		},
	}
}

func buildEnvVars(env map[string]string) []*armcontainerinstance.EnvironmentVariable {
	if len(env) == 0 {
		return nil
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	envVars := make([]*armcontainerinstance.EnvironmentVariable, 0, len(env))
	for _, k := range keys {
		envVars = append(envVars, &armcontainerinstance.EnvironmentVariable{
			Name:  to.Ptr(k),
			Value: to.Ptr(env[k]),
		})
	}
	return envVars
}

func extractFQDN(result armcontainerinstance.ContainerGroupsClientCreateOrUpdateResponse) (string, error) {
	if result.Properties == nil {
		return "", fmt.Errorf("container group has no properties")
	}
	if result.Properties.IPAddress == nil {
		return "", fmt.Errorf("container group has no IP address")
	}
	if result.Properties.IPAddress.Fqdn == nil {
		return "", fmt.Errorf("container group has no FQDN")
	}
	return *result.Properties.IPAddress.Fqdn, nil
}

func (d *Deployer) Delete(ctx context.Context, resourceGroup, name string) error {
	operation := func() error {
		poller, err := d.containerClient.BeginDelete(ctx, resourceGroup, name, nil)
		if err != nil {
			if isPermanentError(err) {
				return backoff.Permanent(err)
			}
			return fmt.Errorf("failed to delete container group: %w", err)
		}

		_, err = poller.PollUntilDone(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to wait for container group deletion: %w", err)
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
