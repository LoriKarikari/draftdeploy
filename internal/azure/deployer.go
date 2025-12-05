package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerinstance/armcontainerinstance/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
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
	_, err := d.rgClient.CreateOrUpdate(ctx, name, armresources.ResourceGroup{
		Location: to.Ptr(location),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group: %w", err)
	}
	return nil
}

func (d *Deployer) Deploy(ctx context.Context, config DeployConfig) (string, error) {
	if err := d.ensureResourceGroup(ctx, config.ResourceGroup, config.Location); err != nil {
		return "", err
	}

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

		envVars := make([]*armcontainerinstance.EnvironmentVariable, 0, len(c.Environment))
		for k, v := range c.Environment {
			envVars = append(envVars, &armcontainerinstance.EnvironmentVariable{
				Name:  to.Ptr(k),
				Value: to.Ptr(v),
			})
		}

		cpu := c.CPU
		if cpu == 0 {
			cpu = 0.5
		}
		mem := c.MemoryGB
		if mem == 0 {
			mem = 0.5
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

	containerGroup := armcontainerinstance.ContainerGroup{
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

	poller, err := d.containerClient.BeginCreateOrUpdate(ctx, config.ResourceGroup, config.Name, containerGroup, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create container group: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("failed to wait for container group: %w", err)
	}

	if result.Properties.IPAddress != nil && result.Properties.IPAddress.Fqdn != nil {
		return *result.Properties.IPAddress.Fqdn, nil
	}

	return "", nil
}

func (d *Deployer) Delete(ctx context.Context, resourceGroup, name string) error {
	poller, err := d.containerClient.BeginDelete(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("failed to delete container group: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to wait for container group deletion: %w", err)
	}

	return nil
}

func (d *Deployer) DeleteResourceGroup(ctx context.Context, name string) error {
	poller, err := d.rgClient.BeginDelete(ctx, name, nil)
	if err != nil {
		return fmt.Errorf("failed to delete resource group: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to wait for resource group deletion: %w", err)
	}

	return nil
}
