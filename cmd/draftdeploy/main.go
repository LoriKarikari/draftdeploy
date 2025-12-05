package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/LoriKarikari/draftdeploy/internal/azure"
	"github.com/LoriKarikari/draftdeploy/internal/compose"
	"github.com/LoriKarikari/draftdeploy/internal/github"
)

const (
	defaultCPU      = 0.5
	defaultMemoryGB = 0.5
	deployTimeout   = 15 * time.Minute
	teardownTimeout = 5 * time.Minute
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

type teardownConfig struct {
	subscriptionID string
	githubToken    string
	owner          string
	repo           string
	prNumber       int
	resourceGroup  string
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
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

	slog.Info("processing PR event",
		"pr_number", prNumber,
		"action", event.Action,
		"owner", owner,
		"repo", repo)

	subscriptionID := strings.TrimSpace(os.Getenv("AZURE_SUBSCRIPTION_ID"))
	location := strings.TrimSpace(os.Getenv("AZURE_LOCATION"))
	composeFile := strings.TrimSpace(os.Getenv("COMPOSE_FILE"))
	githubToken := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))

	if subscriptionID == "" {
		return fmt.Errorf("AZURE_SUBSCRIPTION_ID not set")
	}
	if location == "" {
		location = "eastus"
	}
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}

	resourceGroup, err := sanitizeResourceGroupName(owner, repo, prNumber)
	if err != nil {
		return fmt.Errorf("invalid resource group name: %w", err)
	}
	containerName := fmt.Sprintf("dd-pr%d", prNumber)
	dnsLabel, err := sanitizeDNSLabel(owner, repo, prNumber)
	if err != nil {
		return fmt.Errorf("invalid DNS label: %w", err)
	}

	switch event.Action {
	case "opened", "synchronize", "reopened":
		ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
		defer cancel()
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
		ctx, cancel := context.WithTimeout(context.Background(), teardownTimeout)
		defer cancel()
		return teardown(ctx, teardownConfig{
			subscriptionID: subscriptionID,
			githubToken:    githubToken,
			owner:          owner,
			repo:           repo,
			prNumber:       prNumber,
			resourceGroup:  resourceGroup,
		})
	default:
		slog.Info("ignoring action", "action", event.Action)
		return nil
	}
}

func sanitizeResourceGroupName(owner, repo string, prNumber int) (string, error) {
	re := regexp.MustCompile(`[^a-zA-Z0-9_.-]`)
	cleanOwner := re.ReplaceAllString(owner, "-")
	cleanRepo := re.ReplaceAllString(repo, "-")

	name := fmt.Sprintf("draftdeploy-%s-%s-pr%d", cleanOwner, cleanRepo, prNumber)
	if len(name) > 90 {
		return "", fmt.Errorf("resource group name too long: %d chars (max 90)", len(name))
	}
	return name, nil
}

func sanitizeDNSLabel(owner, repo string, prNumber int) (string, error) {
	re := regexp.MustCompile(`[^a-z0-9-]`)
	cleanOwner := re.ReplaceAllString(strings.ToLower(owner), "-")
	cleanRepo := re.ReplaceAllString(strings.ToLower(repo), "-")

	label := fmt.Sprintf("dd-%s-%s-pr%d", cleanOwner, cleanRepo, prNumber)
	label = strings.Trim(label, "-")

	if len(label) < 3 {
		return "", fmt.Errorf("DNS label too short: %d chars (min 3)", len(label))
	}
	if len(label) > 63 {
		label = fmt.Sprintf("dd-pr%d", prNumber)
	}
	return label, nil
}

func setGitHubOutput(name, value string) error {
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		slog.Info("github output", name, value)
		return nil
	}

	// #nosec G304 G302 -- GITHUB_OUTPUT is a trusted path from GitHub Actions runtime
	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open GITHUB_OUTPUT: %w", err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%s=%s\n", name, value); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

func parseComposeServices(project *compose.Project) ([]azure.ContainerConfig, []github.ServiceInfo) {
	var containers []azure.ContainerConfig
	var services []github.ServiceInfo

	for _, name := range project.GetServiceNames() {
		image := project.GetServiceImage(name)
		if image == "" {
			slog.Info("skipping service with build config", "service", name)
			continue
		}

		ports := project.GetExposedPorts(name)

		containers = append(containers, azure.ContainerConfig{
			Name:     name,
			Image:    image,
			Ports:    ports,
			CPU:      defaultCPU,
			MemoryGB: defaultMemoryGB,
		})

		services = append(services, github.ServiceInfo{
			Name:  name,
			Ports: ports,
		})
	}

	return containers, services
}

func deploy(ctx context.Context, cfg deployConfig) error {
	start := time.Now()

	project, err := compose.Load(cfg.composeFile)
	if err != nil {
		return fmt.Errorf("failed to load compose file: %w", err)
	}

	containers, services := parseComposeServices(project)
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

	var deploymentSucceeded bool
	defer func() {
		if !deploymentSucceeded {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			slog.Warn("deployment failed, attempting cleanup", "resource_group", cfg.resourceGroup)
			if err := deployer.DeleteResourceGroup(cleanupCtx, cfg.resourceGroup); err != nil {
				slog.Error("failed to cleanup resource group", "error", err)
			}
		}
	}()

	slog.Info("deploying to Azure", "resource_group", cfg.resourceGroup, "location", cfg.location)
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
	slog.Info("deployment complete",
		"fqdn", fqdn,
		"deploy_time", deployTime.Round(time.Second))

	if cfg.githubToken != "" {
		commenter := github.NewCommenter(cfg.githubToken, cfg.owner, cfg.repo)
		if err := commenter.PostDeployment(ctx, cfg.prNumber, github.DeploymentInfo{
			FQDN:       fqdn,
			Services:   services,
			DeployTime: deployTime,
		}); err != nil {
			slog.Warn("failed to post comment", "error", err)
		}
	}

	if err := setGitHubOutput("url", fmt.Sprintf("http://%s", fqdn)); err != nil {
		slog.Warn("failed to set url output", "error", err)
	}
	if err := setGitHubOutput("resource-group", cfg.resourceGroup); err != nil {
		slog.Warn("failed to set resource-group output", "error", err)
	}

	deploymentSucceeded = true
	return nil
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

	slog.Info("tearing down resource group", "resource_group", cfg.resourceGroup)
	if err := deployer.DeleteResourceGroup(ctx, cfg.resourceGroup); err != nil {
		return fmt.Errorf("failed to delete resource group: %w", err)
	}

	slog.Info("teardown complete")

	if cfg.githubToken != "" {
		commenter := github.NewCommenter(cfg.githubToken, cfg.owner, cfg.repo)
		if err := commenter.PostTeardown(ctx, cfg.prNumber, github.DeploymentInfo{}); err != nil {
			slog.Warn("failed to post teardown comment", "error", err)
		}
	}

	return nil
}
