package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LoriKarikari/draftdeploy/internal/azure"
	"github.com/LoriKarikari/draftdeploy/internal/compose"
	"github.com/LoriKarikari/draftdeploy/internal/github"
)

type GitHubEvent struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest struct {
		Number int `json:"number"`
	} `json:"pull_request"`
	Repository struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
	} `json:"repository"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		return fmt.Errorf("GITHUB_EVENT_PATH not set")
	}

	cleanPath := filepath.Clean(eventPath)
	eventData, err := os.ReadFile(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to read event file: %w", err)
	}

	var event GitHubEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to parse event: %w", err)
	}

	prNumber := event.PullRequest.Number
	if prNumber == 0 {
		prNumber = event.Number
	}

	owner := event.Repository.Owner.Login
	repo := event.Repository.Name

	fmt.Printf("PR #%d action: %s\n", prNumber, event.Action)

	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	location := os.Getenv("AZURE_LOCATION")
	composeFile := os.Getenv("COMPOSE_FILE")
	githubToken := os.Getenv("GITHUB_TOKEN")

	if subscriptionID == "" {
		return fmt.Errorf("AZURE_SUBSCRIPTION_ID not set")
	}
	if location == "" {
		location = "eastus"
	}
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}

	resourceGroup := fmt.Sprintf("draftdeploy-%s-%s-pr%d", owner, repo, prNumber)
	containerName := fmt.Sprintf("dd-pr%d", prNumber)
	dnsLabel := fmt.Sprintf("dd-%s-%s-pr%d", strings.ToLower(owner), strings.ToLower(repo), prNumber)

	switch event.Action {
	case "opened", "synchronize", "reopened":
		return deploy(ctx, deployConfig{
			subscriptionID: subscriptionID,
			location:       location,
			composeFile:    composeFile,
			githubToken:    githubToken,
			owner:          owner,
			repo:           repo,
			prNumber:       prNumber,
			resourceGroup:  resourceGroup,
			containerName:  containerName,
			dnsLabel:       dnsLabel,
		})
	case "closed":
		return teardown(ctx, teardownConfig{
			subscriptionID: subscriptionID,
			githubToken:    githubToken,
			owner:          owner,
			repo:           repo,
			prNumber:       prNumber,
			resourceGroup:  resourceGroup,
		})
	default:
		fmt.Printf("Ignoring action: %s\n", event.Action)
		return nil
	}
}

type deployConfig struct {
	subscriptionID string
	location       string
	composeFile    string
	githubToken    string
	owner          string
	repo           string
	prNumber       int
	resourceGroup  string
	containerName  string
	dnsLabel       string
}

func deploy(ctx context.Context, cfg deployConfig) error {
	start := time.Now()

	project, err := compose.Load(cfg.composeFile)
	if err != nil {
		return fmt.Errorf("failed to load compose file: %w", err)
	}

	var containers []azure.ContainerConfig
	var services []github.ServiceInfo

	for _, name := range project.GetServiceNames() {
		image := project.GetServiceImage(name)
		if image == "" {
			fmt.Printf("Skipping service %s (has build config, no image)\n", name)
			continue
		}

		ports := project.GetExposedPorts(name)

		containers = append(containers, azure.ContainerConfig{
			Name:     name,
			Image:    image,
			Ports:    ports,
			CPU:      0.5,
			MemoryGB: 0.5,
		})

		services = append(services, github.ServiceInfo{
			Name:  name,
			Ports: ports,
		})
	}

	if len(containers) == 0 {
		return fmt.Errorf("no deployable services found (all have build configs)")
	}

	cred, err := azure.NewCredential()
	if err != nil {
		return fmt.Errorf("failed to create Azure credential: %w", err)
	}

	deployer, err := azure.NewDeployer(cred, cfg.subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to create deployer: %w", err)
	}

	fmt.Printf("Deploying to Azure...\n")
	fqdn, err := deployer.Deploy(ctx, azure.DeployConfig{
		ResourceGroup: cfg.resourceGroup,
		Name:          cfg.containerName,
		Location:      cfg.location,
		Containers:    containers,
		DNSNameLabel:  cfg.dnsLabel,
	})
	if err != nil {
		return fmt.Errorf("failed to deploy: %w", err)
	}

	deployTime := time.Since(start)
	fmt.Printf("Deployed to: http://%s (took %s)\n", fqdn, deployTime.Round(time.Second))

	if cfg.githubToken != "" {
		commenter := github.NewCommenter(cfg.githubToken, cfg.owner, cfg.repo)
		err = commenter.PostDeployment(ctx, cfg.prNumber, github.DeploymentInfo{
			FQDN:       fqdn,
			Services:   services,
			DeployTime: deployTime,
		})
		if err != nil {
			fmt.Printf("Warning: failed to post comment: %v\n", err)
		}
	}

	fmt.Printf("::set-output name=url::http://%s\n", fqdn)
	fmt.Printf("::set-output name=resource-group::%s\n", cfg.resourceGroup)

	return nil
}

type teardownConfig struct {
	subscriptionID string
	githubToken    string
	owner          string
	repo           string
	prNumber       int
	resourceGroup  string
}

func teardown(ctx context.Context, cfg teardownConfig) error {
	cred, err := azure.NewCredential()
	if err != nil {
		return fmt.Errorf("failed to create Azure credential: %w", err)
	}

	deployer, err := azure.NewDeployer(cred, cfg.subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to create deployer: %w", err)
	}

	fmt.Printf("Tearing down resource group: %s\n", cfg.resourceGroup)
	err = deployer.DeleteResourceGroup(ctx, cfg.resourceGroup)
	if err != nil {
		return fmt.Errorf("failed to delete resource group: %w", err)
	}

	fmt.Println("Teardown complete")

	if cfg.githubToken != "" {
		commenter := github.NewCommenter(cfg.githubToken, cfg.owner, cfg.repo)
		err = commenter.PostTeardown(ctx, cfg.prNumber, github.DeploymentInfo{})
		if err != nil {
			fmt.Printf("Warning: failed to post teardown comment: %v\n", err)
		}
	}

	return nil
}
