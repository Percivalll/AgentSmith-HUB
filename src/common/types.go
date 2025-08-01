package common

import (
	"sync"
	"time"
)

type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusStopping Status = "stopping"
	StatusError    Status = "error"
)

// CheckCoreCache for rule engine
type CheckCoreCache struct {
	Exist     bool
	Data      string      // String representation (for backward compatibility)
	TypedData interface{} // Original typed data (for type-preserving access)
}

type HubConfig struct {
	Redis         string `yaml:"redis"`
	RedisPassword string `yaml:"redis_password,omitempty"`
	PprofEnable   bool   `yaml:"pprof_enable"`
	PprofPort     string `yaml:"pprof_port"`
	SIMDEnabled   bool   `yaml:"simd_enabled"`
	ConfigRoot    string
	Leader        string
	LocalIP       string
	Token         string
}

// Operation types for project operations
type OperationType string

const (
	OpTypeChangePush      OperationType = "change_push"
	OpTypeLocalPush       OperationType = "local_push"
	OpTypeComponentDelete OperationType = "component_delete"
	OpTypeComponentAdd    OperationType = "component_add"    // New: for component addition
	OpTypeComponentUpdate OperationType = "component_update" // New: for component update
	OpTypeProjectStart    OperationType = "project_start"
	OpTypeProjectStop     OperationType = "project_stop"
	OpTypeProjectRestart  OperationType = "project_restart"
	// Cluster instruction operations
	OpTypeInstructionPublish OperationType = "instruction_publish" // Leader发布指令
)

// OperationRecord represents a single operation record
type OperationRecord struct {
	Type          OperationType          `json:"type"`
	Timestamp     time.Time              `json:"timestamp"`
	ComponentType string                 `json:"component_type,omitempty"`
	ComponentID   string                 `json:"component_id,omitempty"`
	ProjectID     string                 `json:"project_id,omitempty"`
	Diff          string                 `json:"diff,omitempty"`
	OldContent    string                 `json:"old_content,omitempty"`
	NewContent    string                 `json:"new_content,omitempty"`
	Status        string                 `json:"status"`
	Error         string                 `json:"error,omitempty"`
	Details       map[string]interface{} `json:"details,omitempty"`
}

// Project state Redis keys - IMPORTANT: Separate expected vs actual states
const (
	// Project real state (actual runtime status) - stores the real current status
	// This represents what the project actually is (running, stopped, error, starting, stopping)
	// Format: cluster:proj_real:{nodeID} -> {projectID: "running|stopped|error|starting|stopping"}
	ProjectRealStateKeyPrefix = "cluster:proj_real:" // + nodeID

	// Project state change timestamps
	// Format: cluster:proj_ts:{nodeID} -> {projectID: "2023-12-01T10:00:00Z"}
	ProjectStateTimestampKeyPrefix = "cluster:proj_ts:" // + nodeID

	// Legacy key for user intention (what user wants the project to be)
	// This represents user's expected state (from .project_status file or API calls)
	// Format: cluster:proj_states:{nodeID} -> {projectID: "running"} (only "running" is stored, "stopped" projects have their keys removed)
	ProjectLegacyStateKeyPrefix = "cluster:proj_states:" // + nodeID
)

// StartupCoordinator manages cluster startup coordination
type StartupCoordinator struct {
	isLeader     bool
	leaderReady  bool
	startupMutex sync.RWMutex
}

// Component update states
type ComponentUpdateState int

const (
	UpdateStateIdle ComponentUpdateState = iota
	UpdateStatePreparing
	UpdateStateUpdating
	UpdateStateCompleting
	UpdateStateFailed
)

// ComponentUpdateManager manages component update operations
type ComponentUpdateManager struct {
	activeUpdates map[string]*ComponentUpdateOperation
	mutex         sync.RWMutex
}

// ComponentUpdateOperation represents an ongoing component update
type ComponentUpdateOperation struct {
	ComponentType    string
	ComponentID      string
	State            ComponentUpdateState
	StartTime        time.Time
	LastUpdate       time.Time
	AffectedProjects []string
	Lock             *DistributedLock
	mutex            sync.RWMutex
}
