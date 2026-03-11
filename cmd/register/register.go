package register

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	flagURL           string
	flagEnrollmentKey string
	flagOutput        string
	flagName          string
	flagInsecure      bool
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register and enroll a CrowdSec agent with the self-hosted CAPI",
		Long: `Generates credentials, registers the machine, logs in, and enrolls it
using an enrollment key. Writes online_api_credentials.yaml on success.

Environment variables:
  CAPI_URL             Base URL of the self-hosted CAPI (e.g. https://capi.example.com)
  CAPI_ENROLLMENT_KEY  Enrollment key created in the admin UI
  CAPI_OUTPUT          Output file path (default: online_api_credentials.yaml)
  CAPI_MACHINE_NAME    Human-readable name for this machine (optional)`,
		RunE: run,
	}

	cmd.Flags().StringVar(&flagURL, "url", "", "CAPI base URL (env: CAPI_URL)")
	cmd.Flags().StringVar(&flagEnrollmentKey, "enrollment-key", "", "Enrollment key (env: CAPI_ENROLLMENT_KEY)")
	cmd.Flags().StringVar(&flagOutput, "output", "", "Output file path (env: CAPI_OUTPUT, default: online_api_credentials.yaml)")
	cmd.Flags().StringVar(&flagName, "name", "", "Machine name (env: CAPI_MACHINE_NAME)")
	cmd.Flags().BoolVar(&flagInsecure, "insecure", false, "Skip TLS certificate verification")

	return cmd
}

func envOr(flag, envKey, defaultVal string) string {
	if flag != "" {
		return flag
	}
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return defaultVal
}

type registerRequest struct {
	MachineID string `json:"machine_id"`
	Password  string `json:"password"`
}

type loginRequest struct {
	MachineID string   `json:"machine_id"`
	Password  string   `json:"password"`
	Scenarios []string `json:"scenarios"`
}

type loginResponse struct {
	Code   int    `json:"code"`
	Expire string `json:"expire"`
	Token  string `json:"token"`
}

type enrollRequest struct {
	AttachmentKey string   `json:"attachment_key"`
	Name          string   `json:"name"`
	Tags          []string `json:"tags"`
	Overwrite     bool     `json:"overwrite"`
}

type credentials struct {
	URL      string `yaml:"url"`
	Login    string `yaml:"login"`
	Password string `yaml:"password"`
	PapiURL  string `yaml:"papi_url"`
}

func run(_ *cobra.Command, _ []string) error {
	capiURL := envOr(flagURL, "CAPI_URL", "")
	enrollmentKey := envOr(flagEnrollmentKey, "CAPI_ENROLLMENT_KEY", "")
	output := envOr(flagOutput, "CAPI_OUTPUT", "online_api_credentials.yaml")
	machineName := envOr(flagName, "CAPI_MACHINE_NAME", "")

	if capiURL == "" {
		return fmt.Errorf("CAPI URL is required (--url or CAPI_URL)")
	}
	if enrollmentKey == "" {
		return fmt.Errorf("enrollment key is required (--enrollment-key or CAPI_ENROLLMENT_KEY)")
	}

	// Strip trailing slash
	for len(capiURL) > 0 && capiURL[len(capiURL)-1] == '/' {
		capiURL = capiURL[:len(capiURL)-1]
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if flagInsecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	client := &http.Client{Transport: transport}

	// Generate credentials (machine_id: 48 hex chars, password: 64 hex chars)
	machineIDBytes := make([]byte, 24)
	if _, err := rand.Read(machineIDBytes); err != nil {
		return fmt.Errorf("generating machine_id: %w", err)
	}
	machineID := hex.EncodeToString(machineIDBytes)

	passwordBytes := make([]byte, 32)
	if _, err := rand.Read(passwordBytes); err != nil {
		return fmt.Errorf("generating password: %w", err)
	}
	password := hex.EncodeToString(passwordBytes)

	fmt.Printf("machine_id: %s\n", machineID)

	// Step 1: Register
	fmt.Print("Registering machine... ")
	if err := doJSON(client, http.MethodPost, capiURL+"/v3/watchers", nil,
		registerRequest{MachineID: machineID, Password: password}, nil); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}
	fmt.Println("ok")

	// Step 2: Login
	fmt.Print("Logging in... ")
	var loginResp loginResponse
	if err := doJSON(client, http.MethodPost, capiURL+"/v3/watchers/login", nil,
		loginRequest{MachineID: machineID, Password: password, Scenarios: []string{}},
		&loginResp); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	if loginResp.Token == "" {
		return fmt.Errorf("login returned empty token")
	}
	fmt.Println("ok")

	// Step 3: Enroll
	fmt.Print("Enrolling machine... ")
	if err := doJSON(client, http.MethodPost, capiURL+"/v3/watchers/enroll",
		map[string]string{"Authorization": "Bearer " + loginResp.Token},
		enrollRequest{AttachmentKey: enrollmentKey, Name: machineName, Tags: []string{}, Overwrite: true},
		nil); err != nil {
		return fmt.Errorf("enrollment failed: %w", err)
	}
	fmt.Println("ok")

	// Write credentials file
	creds := credentials{
		URL:      capiURL + "/",
		Login:    machineID,
		Password: password,
		PapiURL:  capiURL + "/",
	}
	data, err := yaml.Marshal(creds)
	if err != nil {
		return fmt.Errorf("marshaling credentials: %w", err)
	}
	if err := os.WriteFile(output, data, 0600); err != nil {
		return fmt.Errorf("writing %s: %w", output, err)
	}
	fmt.Printf("Credentials written to %s\n", output)

	return nil
}

// doJSON sends a JSON request and optionally decodes the response into out.
func doJSON(client *http.Client, method, url string, headers map[string]string, body, out any) error {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(body); err != nil {
		return err
	}

	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var errBody map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		msg := errBody["message"]
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
	}

	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
