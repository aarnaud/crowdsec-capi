package models

import (
	"time"
)

type Machine struct {
	ID           int64      `json:"id"`
	MachineID    string     `json:"machine_id"`
	PasswordHash string     `json:"-"`
	Name         *string    `json:"name"`
	Tags         []string   `json:"tags"`
	Scenarios    []byte     `json:"scenarios"`
	IPAddress    *string    `json:"ip_address"`
	Status       string     `json:"status"`
	EnrolledAt   *time.Time `json:"enrolled_at"`
	LastSeenAt   *time.Time `json:"last_seen_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Decision struct {
	ID              int64         `json:"id"`
	UUID            string        `json:"uuid"`
	Origin          string        `json:"origin"`
	Type            string        `json:"type"`
	Scope           string        `json:"scope"`
	Value           string        `json:"value"`
	Duration        time.Duration `json:"duration"`
	Scenario        *string       `json:"scenario"`
	SourceMachineID *string       `json:"source_machine_id"`
	Simulated       bool          `json:"simulated"`
	IsDeleted       bool          `json:"is_deleted"`
	ExpiresAt       time.Time     `json:"expires_at"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	DeletedAt       *time.Time    `json:"deleted_at"`
}

type Signal struct {
	ID              int64      `json:"id"`
	UUID            string     `json:"uuid"`
	MachineID       string     `json:"machine_id"`
	Scenario        string     `json:"scenario"`
	ScenarioHash    *string    `json:"scenario_hash"`
	ScenarioVersion *string    `json:"scenario_version"`
	SourceScope     *string    `json:"source_scope"`
	SourceValue     *string    `json:"source_value"`
	SourceIP        *string    `json:"source_ip"`
	Labels          []byte     `json:"labels"`
	StartAt         *time.Time `json:"start_at"`
	StopAt          *time.Time `json:"stop_at"`
	AlertCount      int        `json:"alert_count"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Allowlist struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Label       *string   `json:"label"`
	Description *string   `json:"description"`
	Managed     bool      `json:"managed"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AllowlistEntry struct {
	ID          int64      `json:"id"`
	AllowlistID int64      `json:"allowlist_id"`
	Scope       string     `json:"scope"`
	Value       string     `json:"value"`
	Comment     *string    `json:"comment"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type EnrollmentKey struct {
	ID          int64      `json:"id"`
	Key         string     `json:"key"`
	Description *string    `json:"description"`
	Tags        []string   `json:"tags"`
	MaxUses     *int       `json:"max_uses"`
	UseCount    int        `json:"use_count"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type UpstreamSyncState struct {
	LastSyncAt     *time.Time `json:"last_sync_at"`
	LastStartupAt  *time.Time `json:"last_startup_at"`
	MachineID      *string    `json:"machine_id"`
	Token          *string    `json:"-"`
	TokenExpiresAt *time.Time `json:"token_expires_at"`
	DecisionCount  int        `json:"decision_count"`
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

// V3LoginRequest is the v3 variant where scenarios is a flat list of strings.
type V3LoginRequest struct {
	MachineID string   `json:"machine_id" validate:"required"`
	Password  string   `json:"password" validate:"required"`
	Scenarios []string `json:"scenarios"`
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

// Agent-facing allowlist wire types

type AllowlistItemWire struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	Expiration  string `json:"expiration,omitempty"`
}

type AllowlistResponseWire struct {
	AllowlistID    string              `json:"allowlist_id,omitempty"`
	ConsoleManaged bool                `json:"console_managed"`
	CreatedAt      string              `json:"created_at,omitempty"`
	Description    string              `json:"description,omitempty"`
	Name           string              `json:"name"`
	UpdatedAt      string              `json:"updated_at,omitempty"`
	Items          []AllowlistItemWire `json:"items"`
}

type AllowlistLinkWire struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
	URL         string `json:"url"`
}

type BulkCheckAllowlistRequest struct {
	Targets []string `json:"targets" validate:"required"`
}

type BulkCheckAllowlistResult struct {
	Target     string   `json:"target"`
	Allowlists []string `json:"allowlists"`
}

type BulkCheckAllowlistResponse struct {
	Results []BulkCheckAllowlistResult `json:"results"`
}

// V3 decisions stream response — decisions grouped by scenario+scope

type V3DecisionNew struct {
	Duration string `json:"duration"`
	Value    string `json:"value"`
}

type V3DecisionNewGroup struct {
	Scenario  string          `json:"scenario"`
	Scope     string          `json:"scope"`
	Decisions []V3DecisionNew `json:"decisions"`
}

type V3DecisionDeletedGroup struct {
	Scope     string   `json:"scope"`
	Decisions []string `json:"decisions"`
}

type V3DecisionStreamLinks struct {
	Allowlists []AllowlistLinkWire `json:"allowlists"`
	Blocklists []interface{}       `json:"blocklists"`
}

type V3DecisionStreamResponse struct {
	New     []V3DecisionNewGroup     `json:"new"`
	Deleted []V3DecisionDeletedGroup `json:"deleted"`
	Links   *V3DecisionStreamLinks   `json:"links,omitempty"`
}
