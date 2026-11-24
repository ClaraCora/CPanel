package domain

import "time"

type AgentUpgradeBatch struct {
	ID            string                  `json:"id"`
	TargetVersion string                  `json:"target_version"`
	CreatedAt     time.Time               `json:"created_at"`
	Items         []AgentUpgradeBatchItem `json:"items"`
}

type AgentUpgradeBatchItem struct {
	MachineID    string    `json:"machine_id"`
	MachineName  string    `json:"machine_name"`
	AgentVersion string    `json:"agent_version"`
	TaskID       string    `json:"task_id,omitempty"`
	Status       string    `json:"status"`
	Error        string    `json:"error,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}
