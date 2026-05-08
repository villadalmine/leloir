// Package config loads Leloir configuration from YAML files.
//
// Each of the three binaries (control plane, MCP gateway, webhook receiver)
// has its own config schema; this package provides one loader per binary
// plus shared sub-structs.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ControlPlaneConfig is the config for leloir-controlplane.
type ControlPlaneConfig struct {
	Profile string `yaml:"profile"` // "local" | "corporate"

	API struct {
		HTTPAddr string `yaml:"httpAddr"` // e.g. ":8080"
		GRPCAddr string `yaml:"grpcAddr"` // e.g. ":9090"
	} `yaml:"api"`

	Database struct {
		Driver string `yaml:"driver"` // "postgres" | "memory"
		DSN    string `yaml:"dsn"`    // e.g. "postgres://user:pass@host:5432/leloir"
	} `yaml:"database"`

	Auth AuthConfig `yaml:"auth"`

	LLMGateway struct {
		Enabled  bool   `yaml:"enabled"`
		Endpoint string `yaml:"endpoint"` // e.g. "http://llmgateway.leloir.svc:8080"
	} `yaml:"llmGateway"`

	MCPGateway struct {
		Endpoint string `yaml:"endpoint"` // e.g. "http://mcpgateway.leloir.svc:8080"
	} `yaml:"mcpGateway"`

	Audit AuditConfig `yaml:"audit"`

	Observability ObservabilityConfig `yaml:"observability"`

	Kubernetes struct {
		Enabled    bool   `yaml:"enabled"`
		Kubeconfig string `yaml:"kubeconfig"` // empty = in-cluster
	} `yaml:"kubernetes"`
}

// AuthConfig covers OIDC setup.
type AuthConfig struct {
	Mode string `yaml:"mode"` // "oidc" | "single-user" (local only)

	OIDC struct {
		Issuer       string   `yaml:"issuer"`
		ClientID     string   `yaml:"clientId"`
		ClientSecret string   `yaml:"clientSecret"`
		Scopes       []string `yaml:"scopes"`
		GroupsClaim  string   `yaml:"groupsClaim"`
	} `yaml:"oidc"`

	SingleUser struct {
		Username string `yaml:"username"`
	} `yaml:"singleUser"`
}

// AuditConfig covers audit log behavior.
type AuditConfig struct {
	HotRetentionDays int `yaml:"hotRetentionDays"` // default 90

	WarmStorage struct {
		Enabled       bool   `yaml:"enabled"`
		Bucket        string `yaml:"bucket"`
		RetentionDays int    `yaml:"retentionDays"`
	} `yaml:"warmStorage"`

	HashChain struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"hashChain"`

	WORM struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"worm"`
}

// ObservabilityConfig covers OTel, metrics.
type ObservabilityConfig struct {
	OTLP struct {
		Enabled  bool   `yaml:"enabled"`
		Endpoint string `yaml:"endpoint"`
	} `yaml:"otlp"`

	Metrics struct {
		Enabled bool   `yaml:"enabled"`
		Addr    string `yaml:"addr"` // ":9091"
	} `yaml:"metrics"`

	ServiceName string `yaml:"serviceName"`
	Environment string `yaml:"environment"`
}

// MCPGatewayConfig is the config for leloir-mcp-gateway.
type MCPGatewayConfig struct {
	ListenAddr string `yaml:"listenAddr"` // ":8080"

	// EgressPolicy: "allowlist" (production) | "allow-all" (dev)
	EgressPolicy string `yaml:"egressPolicy"`

	// ControlPlaneURL lets the gateway look up MCPServer CRDs
	ControlPlaneURL string `yaml:"controlPlaneUrl"`

	RateLimit struct {
		Enabled    bool `yaml:"enabled"`
		DefaultRPM int  `yaml:"defaultRpm"`
	} `yaml:"rateLimit"`

	TLS struct {
		Enabled  bool   `yaml:"enabled"`
		CertFile string `yaml:"certFile"`
		KeyFile  string `yaml:"keyFile"`
	} `yaml:"tls"`

	Observability ObservabilityConfig `yaml:"observability"`
}

// WebhookConfig is the config for leloir-webhook-receiver.
type WebhookConfig struct {
	ListenAddr string `yaml:"listenAddr"` // ":8081"

	// Forwarder points at the control plane's internal alert ingestion
	ForwardTo string `yaml:"forwardTo"`

	// MaxRequestSize in bytes; default 1MB
	MaxRequestSize int64 `yaml:"maxRequestSize"`

	// Timeout for upstream forward
	ForwardTimeout time.Duration `yaml:"forwardTimeout"`

	Observability ObservabilityConfig `yaml:"observability"`
}

// Load reads a ControlPlaneConfig from a YAML file.
func Load(path string) (*ControlPlaneConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg ControlPlaneConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	applyControlPlaneDefaults(&cfg)
	if err := validateControlPlane(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// LoadMCPGateway reads MCPGatewayConfig.
func LoadMCPGateway(path string) (*MCPGatewayConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg MCPGatewayConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.EgressPolicy == "" {
		cfg.EgressPolicy = "allowlist"
	}
	return &cfg, nil
}

// LoadWebhook reads WebhookConfig.
func LoadWebhook(path string) (*WebhookConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg WebhookConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8081"
	}
	if cfg.MaxRequestSize == 0 {
		cfg.MaxRequestSize = 1 << 20 // 1MB
	}
	if cfg.ForwardTimeout == 0 {
		cfg.ForwardTimeout = 10 * time.Second
	}
	return &cfg, nil
}

func applyControlPlaneDefaults(cfg *ControlPlaneConfig) {
	if cfg.Profile == "" {
		cfg.Profile = "local"
	}
	if cfg.API.HTTPAddr == "" {
		cfg.API.HTTPAddr = ":8080"
	}
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "postgres"
	}
	if cfg.Audit.HotRetentionDays == 0 {
		if cfg.Profile == "corporate" {
			cfg.Audit.HotRetentionDays = 90
		} else {
			cfg.Audit.HotRetentionDays = 7
		}
	}
	if cfg.Observability.ServiceName == "" {
		cfg.Observability.ServiceName = "leloir-controlplane"
	}
	if cfg.Observability.Environment == "" {
		cfg.Observability.Environment = cfg.Profile
	}
}

func validateControlPlane(cfg *ControlPlaneConfig) error {
	if cfg.Profile != "local" && cfg.Profile != "corporate" {
		return fmt.Errorf("invalid profile %q: must be 'local' or 'corporate'", cfg.Profile)
	}
	if cfg.Auth.Mode == "oidc" && cfg.Auth.OIDC.Issuer == "" {
		return fmt.Errorf("auth.oidc.issuer required when auth.mode is 'oidc'")
	}
	// Corporate profile: hard-require audit retention of at least 90 days
	if cfg.Profile == "corporate" && cfg.Audit.HotRetentionDays < 90 {
		return fmt.Errorf("profile 'corporate' requires audit.hotRetentionDays >= 90 (got %d)",
			cfg.Audit.HotRetentionDays)
	}
	return nil
}
