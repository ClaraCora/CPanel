package domain

import (
	"encoding/json"
	"time"
)

type Admin struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	LastLogin *time.Time `json:"last_login_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type AdminAuth struct {
	Admin
	PasswordHash string
}

type AdminSession struct {
	ID        string
	Admin     Admin
	CSRFToken string
	ExpiresAt time.Time
}

type Machine struct {
	ID                       string          `json:"id"`
	Name                     string          `json:"name"`
	Region                   string          `json:"region"`
	Host                     string          `json:"host"`
	Labels                   json.RawMessage `json:"labels"`
	Notes                    string          `json:"notes,omitempty"`
	Status                   string          `json:"status"`
	AgentVersion             string          `json:"agent_version"`
	LatestAgentVersion       string          `json:"latest_agent_version"`
	KernelType               string          `json:"kernel_type"`
	Capabilities             json.RawMessage `json:"capabilities"`
	LastHeartbeat            *time.Time      `json:"last_heartbeat_at"`
	Metrics                  json.RawMessage `json:"metrics"`
	MetricsSampledAt         *time.Time      `json:"metrics_sampled_at"`
	AgentUpgradeTaskID       string          `json:"agent_upgrade_task_id,omitempty"`
	AgentUpgradeRequestedAt  *time.Time      `json:"agent_upgrade_requested_at,omitempty"`
	AgentUpgradeDispatchedAt *time.Time      `json:"agent_upgrade_dispatched_at,omitempty"`
	AgentProtocol            string          `json:"agent_protocol"`
	AgentV2LastSeenAt        *time.Time      `json:"agent_v2_last_seen_at"`
	NodeCount                int             `json:"node_count"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
}

type AgentUpgradeTask struct {
	ID           string     `json:"id"`
	MachineID    string     `json:"machine_id"`
	RequestedAt  time.Time  `json:"requested_at"`
	DispatchedAt *time.Time `json:"dispatched_at,omitempty"`
}

type AgentCommand struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type MachineCreate struct {
	Name       string          `json:"name"`
	Region     string          `json:"region"`
	Host       string          `json:"host"`
	Labels     json.RawMessage `json:"labels"`
	Notes      string          `json:"notes"`
	KernelType string          `json:"kernel_type"`
}

type MachineUpdate struct {
	Name       *string          `json:"name"`
	Region     *string          `json:"region"`
	Host       *string          `json:"host"`
	Labels     *json.RawMessage `json:"labels"`
	Notes      *string          `json:"notes"`
	KernelType *string          `json:"kernel_type"`
	Status     *string          `json:"status"`
}

type MachineCredential struct {
	ID          string     `json:"id"`
	MachineID   string     `json:"machine_id"`
	Token       string     `json:"token"`
	TokenPrefix string     `json:"token_prefix"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Node struct {
	ID              string          `json:"id"`
	AgentID         int64           `json:"agent_id"`
	MachineID       string          `json:"machine_id"`
	MachineName     string          `json:"machine_name"`
	RoutePolicyID   *string         `json:"route_policy_id"`
	Name            string          `json:"name"`
	Protocol        string          `json:"protocol"`
	ListenIP        string          `json:"listen_ip"`
	ServerPort      int             `json:"server_port"`
	KernelType      string          `json:"kernel_type"`
	Config          json.RawMessage `json:"config"`
	Status          string          `json:"status"`
	CurrentRevision int             `json:"current_revision"`
	AppliedRevision int             `json:"applied_revision"`
	LastReport      *time.Time      `json:"last_report_at"`
	LastError       string          `json:"last_error,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	Endpoints       []NodeEndpoint  `json:"endpoints"`
}

type NodeEndpoint struct {
	ID        string    `json:"id,omitempty"`
	NodeID    string    `json:"node_id,omitempty"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Status    string    `json:"status"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type NodeCreate struct {
	MachineID     string          `json:"machine_id"`
	RoutePolicyID *string         `json:"route_policy_id"`
	Name          string          `json:"name"`
	Protocol      string          `json:"protocol"`
	ListenIP      string          `json:"listen_ip"`
	ServerPort    int             `json:"server_port"`
	KernelType    string          `json:"kernel_type"`
	Config        json.RawMessage `json:"config"`
	Endpoints     []NodeEndpoint  `json:"endpoints"`
}

type NodeUpdate struct {
	MachineID     *string          `json:"machine_id"`
	RoutePolicyID *string          `json:"route_policy_id"`
	Name          *string          `json:"name"`
	Protocol      *string          `json:"protocol"`
	ListenIP      *string          `json:"listen_ip"`
	ServerPort    *int             `json:"server_port"`
	KernelType    *string          `json:"kernel_type"`
	Config        *json.RawMessage `json:"config"`
	Status        *string          `json:"status"`
	Endpoints     *[]NodeEndpoint  `json:"endpoints"`
}

type AccessGroup struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Notes     string    `json:"notes,omitempty"`
	NodeCount int       `json:"node_count"`
	UserCount int       `json:"user_count"`
	NodeIDs   []string  `json:"node_ids"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AccessGroupUpdate struct {
	Name    *string   `json:"name"`
	Notes   *string   `json:"notes"`
	NodeIDs *[]string `json:"node_ids"`
	Status  *string   `json:"status"`
}

type Plan struct {
	ID                string    `json:"id"`
	AccessGroupID     string    `json:"access_group_id"`
	AccessGroupName   string    `json:"access_group_name"`
	Name              string    `json:"name"`
	Status            string    `json:"status"`
	TrafficLimitBytes int64     `json:"traffic_limit_bytes"`
	SpeedLimitMbps    int       `json:"speed_limit_mbps"`
	DeviceLimit       int       `json:"device_limit"`
	ResetStrategy     string    `json:"reset_strategy"`
	DefaultValidDays  int       `json:"default_valid_days"`
	Notes             string    `json:"notes,omitempty"`
	UserCount         int       `json:"user_count"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type PlanCreate struct {
	AccessGroupID     string `json:"access_group_id"`
	Name              string `json:"name"`
	TrafficLimitBytes int64  `json:"traffic_limit_bytes"`
	SpeedLimitMbps    int    `json:"speed_limit_mbps"`
	DeviceLimit       int    `json:"device_limit"`
	ResetStrategy     string `json:"reset_strategy"`
	DefaultValidDays  int    `json:"default_valid_days"`
	Notes             string `json:"notes"`
}

type PlanUpdate struct {
	AccessGroupID     *string `json:"access_group_id"`
	Name              *string `json:"name"`
	TrafficLimitBytes *int64  `json:"traffic_limit_bytes"`
	SpeedLimitMbps    *int    `json:"speed_limit_mbps"`
	DeviceLimit       *int    `json:"device_limit"`
	ResetStrategy     *string `json:"reset_strategy"`
	DefaultValidDays  *int    `json:"default_valid_days"`
	Notes             *string `json:"notes"`
	Status            *string `json:"status"`
}

type User struct {
	ID                        string     `json:"id"`
	AgentID                   int64      `json:"agent_id"`
	Role                      string     `json:"role"`
	PlanID                    *string    `json:"plan_id"`
	PlanName                  *string    `json:"plan_name"`
	AccessGroupOverrideID     *string    `json:"access_group_override_id"`
	Name                      string     `json:"name"`
	Email                     *string    `json:"email"`
	UUID                      string     `json:"uuid"`
	SubscriptionTokenPrefix   string     `json:"subscription_token_prefix"`
	SubscriptionAvailable     bool       `json:"subscription_available"`
	Status                    string     `json:"status"`
	TrafficLimitOverrideBytes *int64     `json:"traffic_limit_override_bytes"`
	SpeedLimitOverrideMbps    *int       `json:"speed_limit_override_mbps"`
	DeviceLimitOverride       *int       `json:"device_limit_override"`
	TrafficUsedBytes          int64      `json:"traffic_used_bytes"`
	TrafficLimitBytes         int64      `json:"traffic_limit_bytes"`
	TrafficResetAt            *time.Time `json:"traffic_reset_at"`
	ExpiresAt                 *time.Time `json:"expires_at"`
	Notes                     string     `json:"notes,omitempty"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

type UserCreate struct {
	Role                      string     `json:"role"`
	PlanID                    *string    `json:"plan_id"`
	AccessGroupOverrideID     *string    `json:"access_group_override_id"`
	Name                      string     `json:"name"`
	Email                     *string    `json:"email"`
	TrafficLimitOverrideBytes *int64     `json:"traffic_limit_override_bytes"`
	SpeedLimitOverrideMbps    *int       `json:"speed_limit_override_mbps"`
	DeviceLimitOverride       *int       `json:"device_limit_override"`
	ExpiresAt                 *time.Time `json:"expires_at"`
	Notes                     string     `json:"notes"`
}

type UserCreated struct {
	User
	SubscriptionToken string `json:"subscription_token"`
}

type UserSubscription struct {
	URL string `json:"url"`
}

type UserUpdate struct {
	Role                  *string `json:"role"`
	UUID                  *string `json:"uuid"`
	PlanID                *string `json:"plan_id"`
	AccessGroupOverrideID *string `json:"access_group_override_id"`
	Name                  *string `json:"name"`
	Email                 *string `json:"email"`
	ExpiresAt             *string `json:"expires_at"`
	Notes                 *string `json:"notes"`
	Status                *string `json:"status"`
}

type RoutePolicyUpdate struct {
	Name               *string            `json:"name"`
	Notes              *string            `json:"notes"`
	Status             *string            `json:"status"`
	DefaultOutboundTag *string            `json:"default_outbound_tag"`
	Rules              *[]RoutePolicyRule `json:"rules"`
}

type OutboundUpdate struct {
	Name          *string          `json:"name"`
	Tag           *string          `json:"tag"`
	Protocol      *string          `json:"protocol"`
	Settings      *json.RawMessage `json:"settings"`
	ProxyTag      *string          `json:"proxy_tag"`
	KernelSupport *[]string        `json:"kernel_support"`
	Status        *string          `json:"status"`
}

type RoutePolicy struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Status             string            `json:"status"`
	CurrentRevision    int               `json:"current_revision"`
	DefaultOutboundTag string            `json:"default_outbound_tag"`
	Rules              []RoutePolicyRule `json:"rules"`
	Notes              string            `json:"notes,omitempty"`
	NodeCount          int               `json:"node_count"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type RoutePolicyRule struct {
	Name     string            `json:"name,omitempty"`
	Disabled bool              `json:"disabled,omitempty"`
	Match    RoutePolicyMatch  `json:"match"`
	Action   RoutePolicyAction `json:"action"`
}

type RoutePolicyMatch struct {
	Domains        []string `json:"domains,omitempty"`
	DomainSuffixes []string `json:"domain_suffixes,omitempty"`
	DomainRegexes  []string `json:"domain_regexes,omitempty"`
	GeoIPs         []string `json:"geo_ips,omitempty"`
	IPCIDRs        []string `json:"ip_cidrs,omitempty"`
	Ports          []string `json:"ports,omitempty"`
	Networks       []string `json:"networks,omitempty"`
	SourceCIDRs    []string `json:"source_cidrs,omitempty"`
	SourcePorts    []string `json:"source_ports,omitempty"`
}

type RoutePolicyAction struct {
	Type   string `json:"type"`
	Target string `json:"target,omitempty"`
}

type Outbound struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Tag           string          `json:"tag"`
	Protocol      string          `json:"protocol"`
	Settings      json.RawMessage `json:"settings"`
	ProxyTag      string          `json:"proxy_tag"`
	KernelSupport []string        `json:"kernel_support"`
	Status        string          `json:"status"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type Overview struct {
	MachinesTotal        int           `json:"machines_total"`
	MachinesOnline       int           `json:"machines_online"`
	MachinesOffline      int           `json:"machines_offline"`
	NodesTotal           int           `json:"nodes_total"`
	NodesPublished       int           `json:"nodes_published"`
	AdminsActive         int           `json:"admins_active"`
	UsersActive          int           `json:"users_active"`
	FriendsActive        int           `json:"friends_active"`
	TrafficToday         int64         `json:"traffic_today_bytes"`
	TrafficTodayUpload   int64         `json:"traffic_today_upload_bytes"`
	TrafficTodayDownload int64         `json:"traffic_today_download_bytes"`
	NodeTrafficRanking   []TrafficRank `json:"node_traffic_ranking"`
	UserTrafficRanking   []TrafficRank `json:"user_traffic_ranking"`
}

type TrafficRank struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	UploadBytes   int64  `json:"upload_bytes"`
	DownloadBytes int64  `json:"download_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
}

type TrafficSummary struct {
	UploadBytes   int64 `json:"upload_bytes"`
	DownloadBytes int64 `json:"download_bytes"`
	TotalBytes    int64 `json:"total_bytes"`
}

type HistoricalData struct {
	Retention      HistoricalRetention `json:"retention"`
	Traffic        []DailyTraffic      `json:"traffic"`
	MachineMetrics []MetricSample      `json:"machine_metrics"`
	NodeMetrics    []MetricSample      `json:"node_metrics"`
	Devices        []DeviceHistory     `json:"devices"`
}

type HistoricalRetention struct {
	DevicesDays int `json:"devices_days"`
	TrafficDays int `json:"traffic_days"`
}

type DailyTraffic struct {
	Day           time.Time `json:"day"`
	UploadBytes   int64     `json:"upload_bytes"`
	DownloadBytes int64     `json:"download_bytes"`
}

type MetricSample struct {
	ResourceID   string          `json:"resource_id"`
	ResourceName string          `json:"resource_name"`
	SampledAt    time.Time       `json:"sampled_at"`
	Metrics      json.RawMessage `json:"metrics"`
}

type DeviceHistory struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	NodeID    string    `json:"node_id"`
	NodeName  string    `json:"node_name"`
	IPAddress string    `json:"ip_address"`
	FirstSeen time.Time `json:"first_seen_at"`
	LastSeen  time.Time `json:"last_seen_at"`
	Online    bool      `json:"online"`
}

type AgentMachine struct {
	ID           string
	Name         string
	Status       string
	KernelType   string
	TokenID      string
	Capabilities json.RawMessage
}

type AgentNode struct {
	AgentID int64  `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
}

type AgentUser struct {
	ID          int64  `json:"id"`
	UUID        string `json:"uuid"`
	SpeedLimit  int    `json:"speed_limit"`
	DeviceLimit int    `json:"device_limit"`
}

type AgentNodeSpec struct {
	NodeID     int64           `json:"node_id"`
	Revision   int             `json:"revision"`
	Protocol   string          `json:"protocol"`
	ListenIP   string          `json:"listen_ip"`
	ServerPort int             `json:"server_port"`
	KernelType string          `json:"kernel_type"`
	Settings   json.RawMessage `json:"settings"`
}

type ControlChange struct {
	Cursor    int64           `json:"cursor"`
	NodeID    *int64          `json:"node_id,omitempty"`
	EventType string          `json:"type"`
	Revision  int             `json:"revision"`
	Payload   json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"occurred_at"`
}

type Setting struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	Sensitive bool            `json:"sensitive"`
	Version   int             `json:"version"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type SubscriptionAccessEvent struct {
	ID         string    `json:"id"`
	UserID     *string   `json:"user_id"`
	UserName   string    `json:"user_name"`
	IPAddress  string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
	Outcome    string    `json:"outcome"`
	StatusCode int       `json:"status_code"`
	CreatedAt  time.Time `json:"created_at"`
}

type AuditEvent struct {
	ID           string          `json:"id"`
	AdminID      *string         `json:"admin_id"`
	AdminName    *string         `json:"admin_name"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	Changes      json.RawMessage `json:"changes"`
	IPAddress    *string         `json:"ip_address"`
	RequestID    string          `json:"request_id"`
	CreatedAt    time.Time       `json:"created_at"`
}

type SubscriptionNode struct {
	Name       string
	Host       string
	Port       int
	Protocol   string
	UserUUID   string
	NodeConfig json.RawMessage
}

type Subscription struct {
	UserID    string
	UserName  string
	Role      string
	ExpiresAt *time.Time
	Nodes     []SubscriptionNode
}
