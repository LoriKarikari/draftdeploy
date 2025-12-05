package azure

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerinstance/armcontainerinstance/v2"
)

type Deployer struct {
	client         *armcontainerinstance.ContainerGroupsClient
	subscriptionID string
}

func NewDeployer(credential azcore.TokenCredential, subscriptionID string) (*Deployer, error) {
	client, err := armcontainerinstance.NewContainerGroupsClient(subscriptionID, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create container groups client: %w", err)
	}

	return &Deployer{
		client:         client,
		subscriptionID: subscriptionID,
	}, nil
}
