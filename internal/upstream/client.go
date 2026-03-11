package upstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	mu         sync.Mutex
	baseURL    string
	httpClient *http.Client
	token      string
	tokenExp   time.Time
	machineID  string
	password   string
}

func NewClient(baseURL, machineID, password string) *Client {
	return &Client{
		baseURL:   baseURL,
		machineID: machineID,
		password:  password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Login(ctx context.Context) error {
	payload := map[string]string{
		"machine_id": c.machineID,
		"password":   c.password,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/watchers/login", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed: status %d", resp.StatusCode)
	}

	var result struct {
		Token  string `json:"token"`
		Expire string `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding login response: %w", err)
	}

	c.mu.Lock()
	c.token = result.Token
	c.tokenExp, _ = time.Parse(time.RFC3339, result.Expire)
	c.mu.Unlock()
	return nil
}

func (c *Client) ensureToken(ctx context.Context) error {
	c.mu.Lock()
	valid := c.token != "" && time.Now().Before(c.tokenExp.Add(-5*time.Minute))
	c.mu.Unlock()
	if valid {
		return nil
	}
	return c.Login(ctx)
}

type DecisionStreamResponse struct {
	New     []DecisionItem `json:"new"`
	Deleted []DecisionItem `json:"deleted"`
}

type DecisionItem struct {
	UUID     string `json:"uuid"`
	Origin   string `json:"origin"`
	Type     string `json:"type"`
	Scope    string `json:"scope"`
	Value    string `json:"value"`
	Duration string `json:"duration"`
	Scenario string `json:"scenario"`
}

func (c *Client) GetDecisions(ctx context.Context, startup bool) (*DecisionStreamResponse, error) {
	if err := c.ensureToken(ctx); err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	url := fmt.Sprintf("%s/v2/decisions/stream?startup=%v", c.baseURL, startup)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get decisions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get decisions failed: status %d: %s", resp.StatusCode, body)
	}

	var result DecisionStreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding decisions: %w", err)
	}
	return &result, nil
}
