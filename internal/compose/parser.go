package compose

import (
	"context"
	"fmt"
	"path/filepath"

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
		return nil, fmt.Errorf("failed to load compose file: %w", err)
	}

	if len(project.Services) == 0 {
		return nil, fmt.Errorf("no services defined in compose file")
	}

	return &Project{Project: project}, nil
}

func (p *Project) GetServiceNames() []string {
	names := make([]string, 0, len(p.Services))
	for name := range p.Services {
		names = append(names, name)
	}
	return names
}

func (p *Project) GetExposedPorts(serviceName string) []uint32 {
	service, ok := p.Services[serviceName]
	if !ok {
		return nil
	}

	var ports []uint32
	for _, port := range service.Ports {
		if port.Published != "" {
			ports = append(ports, port.Target)
		}
	}
	return ports
}
