package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
}

type tagsResponse struct {
	Models []modelInfo `json:"models"`
}

type modelInfo struct {
	Name string `json:"name"`
}

func (c *Client) Generate(ctx context.Context, model, prompt string) (string, error) {
	body, _ := json.Marshal(generateRequest{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama status %d: %s", resp.StatusCode, string(b))
	}

	var result generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return result.Response, nil
}

func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var tags tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}

	var names []string
	for _, m := range tags.Models {
		names = append(names, m.Name)
	}
	return names, nil
}

func (c *Client) BuildAnalysisPrompt(containerName, status, health string, events []string, metrics []string) string {
	var sb strings.Builder
	sb.WriteString("You are a DevOps assistant analyzing a home lab server.\n\n")
	sb.WriteString(fmt.Sprintf("Container: %s\n", containerName))
	sb.WriteString(fmt.Sprintf("Current status: %s\n", status))
	sb.WriteString(fmt.Sprintf("Health: %s\n\n", health))

	if len(events) > 0 {
		sb.WriteString("Recent events (last 50):\n")
		for _, e := range events {
			sb.WriteString(e + "\n")
		}
		sb.WriteString("\n")
	}

	if len(metrics) > 0 {
		sb.WriteString("Server metrics last hour (CPU%, RAM used GB):\n")
		for _, m := range metrics {
			sb.WriteString(m + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Briefly explain what might be wrong and suggest what to check.\n")
	return sb.String()
}
