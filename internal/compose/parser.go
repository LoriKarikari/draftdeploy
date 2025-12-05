package compose

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/compose-spec/compose-go/v2/cli"
	"github.com/compose-spec/compose-go/v2/types"
)

type Project struct {
	*types.Project
}

func Load(path string) (*Project, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	opts, err := cli.NewProjectOptions(
		[]string{absPath},
		cli.WithOsEnv,
		cli.WithDotEnv,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create project options: %w", err)
	}

	project, err := cli.ProjectFromOptions(context.Background(), opts)
	if err != nil {
		return nil, fmt.Errorf("failed to load compose file %s: %w", path, err)
	}

	if len(project.Services) == 0 {
		return nil, fmt.Errorf("no services defined in compose file %s", path)
	}

	return &Project{Project: project}, nil
}

func (p *Project) GetServiceNames() []string {
	names := make([]string, 0, len(p.Services))
	for name := range p.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (p *Project) GetExposedPorts(serviceName string) []int32 {
	service, ok := p.Services[serviceName]
	if !ok {
		return nil
	}

	var ports []int32
	for _, port := range service.Ports {
		if port.Published == "" {
			continue
		}
		if port.Target < 1 || port.Target > 65535 {
			continue
		}
		ports = append(ports, int32(port.Target))
	}
	return ports
}

func (p *Project) GetServiceImage(serviceName string) string {
	service, ok := p.Services[serviceName]
	if !ok {
		return ""
	}
	return service.Image
}
