// Package v1alpha1 contains the Go types for Leloir's Kubernetes CRDs.
//
// All types here mirror the YAML CRD definitions in deploy/crds/.
// Controller-runtime uses these to read and reconcile resources.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GroupName is the API group for Leloir CRDs
const GroupName = "leloir.dev"

// SchemeGroupVersion is the group-version-kind pair
const SchemeGroupVersion = GroupName + "/v1alpha1"

// ─────────────────────────────────────────────────────────────────────────────
// Tenant
// ─────────────────────────────────────────────────────────────────────────────

// Tenant represents a Leloir tenant.
// +kubebuilder:resource:scope=Cluster
type Tenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TenantSpec   `json:"spec"`
	Status            TenantStatus `json:"status,omitempty"`
}

// TenantSpec describes a tenant.
type TenantSpec struct {
	DisplayName  string   `json:"displayName"`
	Namespaces   []string `json:"namespaces"`
	OIDCGroups   []string `json:"oidcGroups,omitempty"`
	DefaultBudget *string  `json:"defaultBudget,omitempty"` // ref to TenantBudget
}

// TenantStatus describes the observed state.
type TenantStatus struct {
	Ready      bool               `json:"ready"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// AgentRegistration
// ─────────────────────────────────────────────────────────────────────────────

// AgentRegistration registers an AI agent adapter with Leloir.
type AgentRegistration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AgentRegistrationSpec   `json:"spec"`
	Status            AgentRegistrationStatus `json:"status,omitempty"`
}

// AgentRegistrationSpec is the agent definition.
type AgentRegistrationSpec struct {
	Adapter      AgentAdapterRef `json:"adapter"`
	ModelConfig  string          `json:"modelConfig,omitempty"` // LLMGateway provider key
	CustomConfig map[string]any  `json:"customConfig,omitempty"`

	// A2A permissions (defense in depth, layer 1)
	CanInvoke    []string `json:"canInvoke,omitempty"`    // allowlist
	CannotInvoke []string `json:"cannotInvoke,omitempty"` // explicit deny, wins over allow

	Capabilities []string `json:"capabilities,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// AgentAdapterRef references the container image for the adapter.
type AgentAdapterRef struct {
	Image    string            `json:"image"`
	Name     string            `json:"name,omitempty"`     // populated from Identity()
	Version  string            `json:"version,omitempty"`  // populated from Identity()
	Replicas *int32            `json:"replicas,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
}

// AgentRegistrationStatus reports agent health.
type AgentRegistrationStatus struct {
	Health             string             `json:"health"` // "healthy" | "degraded" | "unhealthy"
	LastHealthCheck    *metav1.Time       `json:"lastHealthCheck,omitempty"`
	ActiveInvestigations int32            `json:"activeInvestigations"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// AlertRoute
// ─────────────────────────────────────────────────────────────────────────────

// AlertRoute routes incoming alerts to an agent with specific context.
type AlertRoute struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              AlertRouteSpec   `json:"spec"`
	Status            AlertRouteStatus `json:"status,omitempty"`
}

// AlertRouteSpec is the routing rule.
type AlertRouteSpec struct {
	Enabled  bool  `json:"enabled"`
	Priority int32 `json:"priority,omitempty"` // higher = evaluated first

	Match AlertMatch `json:"match"`

	Agent          string   `json:"agent"`                    // AgentRegistration name
	Team           []string `json:"team,omitempty"`           // A2A team: sub-agents allowed for this route
	AllowedSources []string `json:"allowedSources,omitempty"` // MCPServer refs
	Skills         []string `json:"skills,omitempty"`         // SkillSource refs
	Notify         []string `json:"notify,omitempty"`         // NotificationChannel refs

	Budget   *RouteBudget    `json:"budget,omitempty"`
	Timeouts RouteTimeouts   `json:"timeouts,omitempty"`
}

// AlertMatch describes label-based alert matching.
type AlertMatch struct {
	Labels  map[string]string `json:"labels,omitempty"`
	Sources []string          `json:"sources,omitempty"` // e.g. ["alertmanager"]
}

// RouteBudget sets per-investigation cost caps.
type RouteBudget struct {
	MaxUSD       float64 `json:"maxUSD,omitempty"`
	MaxTokens    int64   `json:"maxTokens,omitempty"`
	MaxToolCalls int32   `json:"maxToolCalls,omitempty"`
	MaxSubInvocations int32 `json:"maxSubInvocations,omitempty"`
}

// RouteTimeouts configures deadlines.
type RouteTimeouts struct {
	InvestigationMinutes int32 `json:"investigationMinutes,omitempty"`
}

// AlertRouteStatus reports usage stats.
type AlertRouteStatus struct {
	InvestigationsStarted   int64 `json:"investigationsStarted"`
	InvestigationsSucceeded int64 `json:"investigationsSucceeded"`
	InvestigationsFailed    int64 `json:"investigationsFailed"`
	LastInvestigation       *metav1.Time `json:"lastInvestigation,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// MCPServer
// ─────────────────────────────────────────────────────────────────────────────

// MCPServer registers an MCP tool source.
type MCPServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MCPServerSpec   `json:"spec"`
	Status            MCPServerStatus `json:"status,omitempty"`
}

// MCPServerSpec defines the MCP source.
type MCPServerSpec struct {
	TrustTier  string `json:"trustTier"`  // "internal-hosted" | "vendor-vetted" | "external"
	Visibility string `json:"visibility"` // "global" | "namespace"

	Transport MCPTransport `json:"transport"`

	AllowedTools   []string          `json:"allowedTools,omitempty"`
	CredentialRef  *SecretRef        `json:"credentialRef,omitempty"`
	Policies       MCPPolicies       `json:"policies,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}

// MCPTransport describes how to talk to the MCP server.
type MCPTransport struct {
	Type    string   `json:"type"`    // "streamable-http" | "grpc" | "sse" | "stdio"
	URL     string   `json:"url,omitempty"`
	Command string   `json:"command,omitempty"` // for stdio
	Args    []string `json:"args,omitempty"`    // for stdio

	TLS *TransportTLS `json:"tls,omitempty"`

	ProtoDescriptorCMRef *ConfigMapRef `json:"protoDescriptorConfigMapRef,omitempty"` // for grpc
}

// TransportTLS configures TLS for the transport.
type TransportTLS struct {
	InsecureSkipVerify bool       `json:"insecureSkipVerify,omitempty"`
	CASecretRef        *SecretRef `json:"caSecretRef,omitempty"`
	ClientCertSecretRef *SecretRef `json:"clientCertSecretRef,omitempty"`
}

// MCPPolicies configures behavior of the MCP client.
type MCPPolicies struct {
	RateLimit       string   `json:"rateLimit,omitempty"` // e.g. "100/min"
	RequireApproval []string `json:"requireApproval,omitempty"` // tool names needing HITL
	TimeoutSeconds  int32    `json:"timeoutSeconds,omitempty"`
}

// SecretRef references a Kubernetes Secret.
type SecretRef struct {
	Name string `json:"name"`
	Key  string `json:"key,omitempty"`
}

// ConfigMapRef references a Kubernetes ConfigMap.
type ConfigMapRef struct {
	Name string `json:"name"`
	Key  string `json:"key,omitempty"`
}

// MCPServerStatus reports health of the MCP source.
type MCPServerStatus struct {
	Healthy          bool         `json:"healthy"`
	LastProbe        *metav1.Time `json:"lastProbe,omitempty"`
	ToolCallsTotal   int64        `json:"toolCallsTotal"`
	ToolCallsFailed  int64        `json:"toolCallsFailed"`
	DiscoveredTools  []string     `json:"discoveredTools,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// NotificationChannel, ApprovalPolicy, TenantBudget, SkillSource
// (abbreviated — full definitions in separate files when implementing M1+)
// ─────────────────────────────────────────────────────────────────────────────

// NotificationChannel — Teams/Slack/Telegram/Webhook endpoint config.
type NotificationChannel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              NotificationChannelSpec `json:"spec"`
}

// NotificationChannelSpec — abbreviated, expand in M1.
type NotificationChannelSpec struct {
	Type   string            `json:"type"` // "teams" | "slack" | "telegram" | "webhook"
	Config map[string]any    `json:"config"`
	Secret *SecretRef        `json:"secretRef,omitempty"`
}

// TenantBudget — hard cost cap per tenant.
type TenantBudget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              TenantBudgetSpec `json:"spec"`
}

// TenantBudgetSpec — abbreviated, expand in M3.
type TenantBudgetSpec struct {
	MonthlyLimitUSD   float64 `json:"monthlyLimitUSD"`
	PerInvestigationMaxUSD float64 `json:"perInvestigationMaxUSD,omitempty"`
	Enforcement       string  `json:"enforcement,omitempty"` // "hard" | "soft" | "off"
}

// ApprovalPolicy — HITL rules.
type ApprovalPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ApprovalPolicySpec `json:"spec"`
}

// ApprovalPolicySpec — abbreviated, expand in M4.
type ApprovalPolicySpec struct {
	Scope ApprovalScope `json:"scope"`
	Tiers []ApprovalTier `json:"tiers"`
}

// ApprovalScope covers what the policy applies to.
type ApprovalScope struct {
	Tools []string `json:"tools,omitempty"`
	A2A   bool     `json:"a2a,omitempty"`
}

// ApprovalTier defines an approval level.
type ApprovalTier struct {
	Name      string   `json:"name"`
	Callers   []string `json:"callers,omitempty"`
	Targets   []string `json:"targets,omitempty"`
	Approvers []string `json:"approvers,omitempty"`
	Channel   string   `json:"channel,omitempty"`
}

// SkillSource — runbook/skill repository.
type SkillSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SkillSourceSpec `json:"spec"`
}

// SkillSourceSpec — abbreviated, expand in M2.
type SkillSourceSpec struct {
	Type       string      `json:"type"` // "inline" | "git" | "configmap"
	Visibility string      `json:"visibility"` // "global" | "namespace"
	Inline     []InlineSkill `json:"inline,omitempty"`
	Git        *GitSkillRef `json:"git,omitempty"`
}

// InlineSkill is a skill defined directly in the CRD.
type InlineSkill struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Content     string   `json:"content"`
	Tags        []string `json:"tags,omitempty"`
}

// GitSkillRef references a git repo with skills.
type GitSkillRef struct {
	URL    string `json:"url"`
	Ref    string `json:"ref,omitempty"`
	Path   string `json:"path,omitempty"`
	Secret *SecretRef `json:"secretRef,omitempty"`
}
