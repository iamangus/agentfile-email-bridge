package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AgentfileClient is an HTTP client for the agentfile REST API.
type AgentfileClient struct {
	baseURL    string
	httpClient *http.Client
}

// agentRequest is the JSON body sent to the agentfile API.
type agentRequest struct {
	Message string `json:"message"`
}

// agentResponse is the JSON body returned by the agentfile API.
type agentResponse struct {
	Agent    string `json:"agent"`
	Response string `json:"response"`
}

// agentErrorResponse is the JSON body returned on error.
type agentErrorResponse struct {
	Error string `json:"error"`
}

// NewAgentfileClient creates a new client for the agentfile REST API.
func NewAgentfileClient(baseURL string, timeout time.Duration) *AgentfileClient {
	return &AgentfileClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// RunAgent sends a message to the specified agent and returns its response.
// It calls POST /api/v1/agents/{agent}/run.
func (c *AgentfileClient) RunAgent(ctx context.Context, agent, message string) (string, error) {
	url := fmt.Sprintf("%s/api/v1/agents/%s/run", c.baseURL, agent)

	reqBody, err := json.Marshal(agentRequest{Message: message})
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending request to agentfile: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp agentErrorResponse
		if jsonErr := json.Unmarshal(body, &errResp); jsonErr == nil && errResp.Error != "" {
			return "", fmt.Errorf("agentfile API error (HTTP %d): %s", resp.StatusCode, errResp.Error)
		}
		return "", fmt.Errorf("agentfile API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result agentResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decoding response JSON: %w", err)
	}

	return result.Response, nil
}
