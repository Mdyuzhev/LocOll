package docker

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type ContainerInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	State   string `json:"state"`
	Project string `json:"project"`
	Health  string `json:"health"`
	Ports   string `json:"ports"`
}

type Client struct {
	cli *client.Client
}

func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() error {
	return c.cli.Close()
}

func (c *Client) ListContainers(ctx context.Context) ([]ContainerInfo, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var result []ContainerInfo
	for _, ctr := range containers {
		name := ""
		if len(ctr.Names) > 0 {
			name = strings.TrimPrefix(ctr.Names[0], "/")
		}

		project := ctr.Labels["com.docker.compose.project"]
		health := ""
		if ctr.State == "running" {
			// Check health status from labels/state
			health = extractHealth(ctr)
		}

		ports := formatPorts(ctr.Ports)

		result = append(result, ContainerInfo{
			ID:      ctr.ID[:12],
			Name:    name,
			Image:   ctr.Image,
			Status:  ctr.Status,
			State:   ctr.State,
			Project: project,
			Health:  health,
			Ports:   ports,
		})
	}
	return result, nil
}

func (c *Client) RestartContainer(ctx context.Context, id string) error {
	timeout := 10
	return c.cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) StopContainer(ctx context.Context, id string) error {
	timeout := 10
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (c *Client) ContainerLogs(ctx context.Context, id string, tail string) (io.ReadCloser, error) {
	return c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       tail,
	})
}

func (c *Client) InspectContainer(ctx context.Context, id string) (types.ContainerJSON, error) {
	return c.cli.ContainerInspect(ctx, id)
}

// FindContainerID resolves a short ID or name to the full container ID
func (c *Client) FindContainerID(ctx context.Context, idOrName string) (string, error) {
	f := filters.NewArgs()
	f.Add("name", idOrName)
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return "", err
	}
	if len(containers) > 0 {
		return containers[0].ID, nil
	}
	// Try by ID prefix
	containers, err = c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "", err
	}
	for _, ctr := range containers {
		if strings.HasPrefix(ctr.ID, idOrName) {
			return ctr.ID, nil
		}
	}
	return idOrName, nil
}

func extractHealth(ctr types.Container) string {
	if ctr.Status == "" {
		return ""
	}
	s := strings.ToLower(ctr.Status)
	switch {
	case strings.Contains(s, "(healthy)"):
		return "healthy"
	case strings.Contains(s, "(unhealthy)"):
		return "unhealthy"
	case strings.Contains(s, "(health:"):
		return "starting"
	default:
		return ""
	}
}

func formatPorts(ports []types.Port) string {
	var parts []string
	seen := make(map[string]bool)
	for _, p := range ports {
		if p.PublicPort > 0 {
			s := fmt.Sprintf("%s:%s->%s/%s", p.IP, strconv.Itoa(int(p.PublicPort)), strconv.Itoa(int(p.PrivatePort)), p.Type)
			if !seen[s] {
				seen[s] = true
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, ", ")
}
