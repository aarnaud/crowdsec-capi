package models

import (
	"time"
)

type Machine struct {
	ID           int64
	MachineID    string
	PasswordHash string
	Name         *string
	Tags         []string
	Scenarios    []byte
	IPAddress    *string
	Status       string
	EnrolledAt   *time.Time
	LastSeenAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Decision struct {
	ID              int64
	UUID            string
	Origin          string
	Type            string
	Scope           string
	Value           string
	Duration        time.Duration
	Scenario        *string
	SourceMachineID *string
	Simulated       bool
	IsDeleted       bool
	ExpiresAt       time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

type Signal struct {
	ID              int64
	UUID            string
	MachineID       string
	Scenario        string
	ScenarioHash    *string
	ScenarioVersion *string
	SourceScope     *string
	SourceValue     *string
	SourceIP        *string
	Labels          []byte
	StartAt         *time.Time
	StopAt          *time.Time
	AlertCount      int
	CreatedAt       time.Time
}

type Allowlist struct {
	ID          int64
	Name        string
	Label       *string
	Description *string
	Managed     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type AllowlistEntry struct {
	ID          int64
	AllowlistID int64
	Scope       string
	Value       string
	Comment     *string
	ExpiresAt   *time.Time
	CreatedAt   time.Time
}

type EnrollmentKey struct {
	ID          int64
	Key         string
	Description *string
	Tags        []string
	MaxUses     *int
	UseCount    int
	ExpiresAt   *time.Time
	CreatedAt   time.Time
}

type UpstreamSyncState struct {
	LastSyncAt     *time.Time
	LastStartupAt  *time.Time
	MachineID      *string
	Token          *string
	TokenExpiresAt *time.Time
	DecisionCount  int
}

// Wire types (JSON request/response)

type RegisterRequest struct {
	MachineID string `json:"machine_id" validate:"required,max=48,alphanum"`
	Password  string `json:"password" validate:"required,min=8"`
}

type LoginRequest struct {
	MachineID string `json:"machine_id" validate:"required"`
	Password  string `json:"password" validate:"required"`
	Scenarios []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Hash    string `json:"hash"`
	} `json:"scenarios"`
}

type LoginResponse struct {
	Code   int    `json:"code"`
	Expire string `json:"expire"`
	Token  string `json:"token"`
}

type EnrollRequest struct {
	EnrollmentKey string   `json:"attachment_key" validate:"required"`
	Name          string   `json:"name"`
	Tags          []string `json:"tags"`
	Overwrite     bool     `json:"overwrite"`
}

type ResetPasswordRequest struct {
	MachineID string `json:"machine_id" validate:"required"`
	Password  string `json:"password" validate:"required,min=8"`
}

type SignalSource struct {
	Scope     string  `json:"scope"`
	Value     string  `json:"value"`
	IP        string  `json:"ip"`
	Range     string  `json:"range"`
	AsName    string  `json:"as_name"`
	AsNumber  int     `json:"as_number"`
	Country   string  `json:"country"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type SignalDecision struct {
	UUID     string `json:"uuid"`
	Scenario string `json:"scenario"`
	Scope    string `json:"scope"`
	Value    string `json:"value"`
	Type     string `json:"type"`
	Duration string `json:"duration"`
	Origin   string `json:"origin"`
}

type SignalItem struct {
	UUID            string            `json:"uuid"`
	MachineID       string            `json:"machine_id"`
	Scenario        string            `json:"scenario"`
	ScenarioHash    string            `json:"scenario_hash"`
	ScenarioVersion string            `json:"scenario_version"`
	Source          SignalSource      `json:"source"`
	Decisions       []SignalDecision  `json:"decisions"`
	Labels          map[string]string `json:"labels"`
	StartAt         string            `json:"start_at"`
	StopAt          string            `json:"stop_at"`
	AlertCount      int               `json:"alert_count"`
	CreatedAt       string            `json:"created_at"`
	Message         string            `json:"message"`
	MachineIDPrefix string            `json:"machine_id_prefix"`
}

type DecisionWire struct {
	UUID      string `json:"uuid"`
	Origin    string `json:"origin"`
	Type      string `json:"type"`
	Scope     string `json:"scope"`
	Value     string `json:"value"`
	Duration  string `json:"duration"`
	Scenario  string `json:"scenario"`
	ID        int64  `json:"id"`
	Simulated bool   `json:"simulated"`
}

type DecisionStreamResponse struct {
	New     []DecisionWire `json:"new"`
	Deleted []DecisionWire `json:"deleted"`
}
