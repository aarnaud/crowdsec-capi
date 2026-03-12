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

// Register creates the machine account on the upstream CAPI. It is safe to
// call multiple times — a 409 Conflict response means the machine already
// exists and is treated as success.
func (c *Client) Register(ctx context.Context) error {
	payload := map[string]string{
		"machine_id": c.machineID,
		"password":   c.password,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/watchers", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("register request: %w", err)
	}
	defer resp.Body.Close()

	// 200 = registered, 400/409 = already exists — all acceptable.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register failed: status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// Enroll attaches the machine to a CrowdSec console organisation using the
// provided enrollment key. It must be called after a successful Login.
// Re-enrolling with the same key is idempotent.
func (c *Client) Enroll(ctx context.Context, enrollmentKey string) error {
	c.mu.Lock()
	token := c.token
	c.mu.Unlock()
	if token == "" {
		return fmt.Errorf("enroll: not authenticated, call Login first")
	}

	payload := map[string]interface{}{
		"attachment_key": enrollmentKey,
		"name":           c.machineID,
		"tags":           []string{"crowdsec-capi"},
		"overwrite":      false,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v2/watchers/enroll", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("enroll request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("enroll failed: status %d: %s", resp.StatusCode, b)
	}
	return nil
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

// MachineID returns the machine ID used for upstream authentication.
func (c *Client) MachineID() string {
	return c.machineID
}

// SetToken pre-loads a token (e.g. restored from DB) to avoid re-authenticating on startup.
func (c *Client) SetToken(token string, exp time.Time) {
	c.mu.Lock()
	c.token = token
	c.tokenExp = exp
	c.mu.Unlock()
}

// GetToken returns the current token and expiry for persistence to DB.
func (c *Client) GetToken() (string, time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token, c.tokenExp
}

// DecisionStreamResponse is the actual (not swagger-documented) response from the
// upstream CAPI /v2/decisions/stream endpoint. Despite the swagger showing a grouped
// structure, the real API returns flat arrays of full decision objects.
type DecisionStreamResponse struct {
	New     []DecisionStreamItem `json:"new"`
	Deleted []DecisionStreamItem `json:"deleted"`
}

type DecisionStreamItem struct {
	Value       string `json:"value"`
	Scope       string `json:"scope"`
	Origin      string `json:"origin"`
	Duration    string `json:"duration"`
	Scenario    string `json:"scenario"`
	Type        string `json:"type"`
	BlocklistID string `json:"blocklist_id"`
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
