package config

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// InfraConfig represents the complete infra.yaml structure
type InfraConfig struct {
	Cluster    ClusterConfig    `yaml:"cluster" mapstructure:"cluster"`
	Nodes      []NodeConfig     `yaml:"nodes" mapstructure:"nodes"`
	Cloudflare CloudflareConfig `yaml:"cloudflare" mapstructure:"cloudflare"`
}

type ClusterConfig struct {
	VIP   string   `yaml:"vip" mapstructure:"vip"`
	SANs  []string `yaml:"sans" mapstructure:"sans"`
	Token string   `yaml:"token" mapstructure:"token"`
}

// CloudflareConfig holds credentials for the Cloudflare provider (DNS,
// Tunnel, Access) and the domain the tunnel routes traffic for.
type CloudflareConfig struct {
	AccountID string       `yaml:"accountId" mapstructure:"accountId"`
	APIToken  string       `yaml:"apiToken" mapstructure:"apiToken"`
	Tunnel    TunnelConfig `yaml:"tunnel" mapstructure:"tunnel"`
	Access    AccessConfig `yaml:"access" mapstructure:"access"`
}

type TunnelConfig struct {
	Domain string `yaml:"domain" mapstructure:"domain"`
}

// AccessConfig configures the Cloudflare Access application that gates
// everything routed through the tunnel.
type AccessConfig struct {
	AllowedEmails []string `yaml:"allowedEmails" mapstructure:"allowedEmails"`
	// TeamDomain is the Cloudflare Zero Trust team domain (the <team-name>
	// in https://<team-name>.cloudflareaccess.com), used as the JWT
	// issuer/JWKS source for validating Access-issued tokens at the shared
	// Gateway.
	TeamDomain string `yaml:"teamDomain" mapstructure:"teamDomain"`
}

type SSHConfig struct {
	User     string `yaml:"user" mapstructure:"user"`
	Port     int    `yaml:"port" mapstructure:"port"`
	Password string `yaml:"password" mapstructure:"password"`
}

type NodeConfig struct {
	Name    string            `yaml:"name" mapstructure:"name"`
	Address string            `yaml:"address" mapstructure:"address"`
	Labels  map[string]string `yaml:"labels,omitempty" mapstructure:"labels"`
	SSH     SSHConfig         `yaml:"ssh" mapstructure:"ssh"`
}

type Config struct {
	Node             string
	SSHUser          string
	SSHPassword      string
	K3SSANS          []string
	ClusterInit      bool
	ServerURL        string
	Token            string
	OutputKubeconfig string

	// New fields for config file support
	ConfigFile  string
	InfraConfig *InfraConfig

	// Skip K3s-specific validation (for commands like kubeconfig)
	SkipK3sValidation bool
}

// Load loads configuration with precedence: CLI flags > infra.yaml > env vars > defaults
func Load(cmd *cobra.Command) (*Config, error) {
	return LoadWithOptions(cmd, false)
}

// LoadWithOptions loads configuration with optional K3s validation skip
func LoadWithOptions(cmd *cobra.Command, skipK3sValidation bool) (*Config, error) {
	cfg := &Config{
		SkipK3sValidation: skipK3sValidation,
	}

	// Initialize viper for environment variables
	viper.SetEnvPrefix("HOMELAB")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Load config file if it exists
	configFile, _ := cmd.Flags().GetString("config")
	if configFile == "" {
		configFile = findConfigFile()
	}

	var infraCfg *InfraConfig
	if configFile != "" {
		var err error
		infraCfg, err = loadInfraYAML(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
		cfg.InfraConfig = infraCfg
		cfg.ConfigFile = configFile
	}

	// Apply configuration with precedence
	if err := applyConfigWithPrecedence(cmd, cfg, infraCfg); err != nil {
		return nil, err
	}

	// Validation
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadFromFile loads InfraConfig directly from a file path (for Pulumi)
func LoadFromFile(path string) (*InfraConfig, error) {
	return loadInfraYAML(path)
}

// LoadInfra loads just the infra.yaml configuration, for commands that
// operate across all nodes rather than targeting one selected node.
func LoadInfra(cmd *cobra.Command) (*InfraConfig, error) {
	configFile, _ := cmd.Flags().GetString("config")
	if configFile == "" {
		configFile = findConfigFile()
	}
	if configFile == "" {
		return nil, fmt.Errorf("no infra.yaml config file found")
	}

	return loadInfraYAML(configFile)
}

// findConfigFile searches for infra.yaml in common locations
func findConfigFile() string {
	candidates := []string{
		"./infra.yaml",
		"./infra.yml",
		"../infra.yaml",
		"../infra.yml",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// loadInfraYAML loads and parses the infra.yaml file
func loadInfraYAML(path string) (*InfraConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg InfraConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	applyDefaults(&cfg)

	// Validate structure
	if err := validateInfraConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// applyDefaults sets default values for optional fields
func applyDefaults(cfg *InfraConfig) {
	for i := range cfg.Nodes {
		if cfg.Nodes[i].SSH.Port == 0 {
			cfg.Nodes[i].SSH.Port = 22
		}
	}

	// SANs default to empty - K3s includes localhost, 127.0.0.1, hostname, and node IPs by default
	// Users should only add the VIP or custom DNS names they actually use
}

// validateInfraConfig validates the infra.yaml structure
func validateInfraConfig(cfg *InfraConfig) error {
	if len(cfg.Nodes) == 0 {
		return fmt.Errorf("at least one node must be defined in infra.yaml")
	}

	// Check for unique node names and required SSH credentials
	names := make(map[string]bool)
	for _, node := range cfg.Nodes {
		if names[node.Name] {
			return fmt.Errorf("duplicate node name: %s", node.Name)
		}
		names[node.Name] = true

		if node.SSH.User == "" {
			return fmt.Errorf("node '%s': ssh.user is required", node.Name)
		}
		if node.SSH.Password == "" {
			return fmt.Errorf("node '%s': ssh.password is required", node.Name)
		}
	}

	// Validate VIP if provided
	if cfg.Cluster.VIP != "" {
		if net.ParseIP(cfg.Cluster.VIP) == nil {
			return fmt.Errorf("invalid cluster VIP: %s", cfg.Cluster.VIP)
		}
	}

	return nil
}

// applyConfigWithPrecedence applies configuration with proper precedence
func applyConfigWithPrecedence(cmd *cobra.Command, cfg *Config, infraCfg *InfraConfig) error {
	// Node selection logic - --node is always a name from infra.yaml
	var selectedNode *NodeConfig
	nodeName, _ := cmd.Flags().GetString("node")

	if infraCfg != nil && nodeName != "" {
		selectedNode = FindNodeByName(infraCfg, nodeName)
		if selectedNode == nil {
			return fmt.Errorf("node '%s' not found in config file", nodeName)
		}
	}

	// Node/Address
	if selectedNode != nil {
		cfg.Node = selectedNode.Address
	} else if env := viper.GetString("node"); env != "" {
		cfg.Node = env
	}

	// SSH User/Password come solely from the selected node's infra.yaml entry
	if selectedNode != nil {
		cfg.SSHUser = selectedNode.SSH.User
		cfg.SSHPassword = selectedNode.SSH.Password
	}

	// SANs
	if sansFlag, err := cmd.Flags().GetStringSlice("sans"); err == nil && len(sansFlag) > 0 {
		cfg.K3SSANS = sansFlag
	} else if infraCfg != nil && len(infraCfg.Cluster.SANs) > 0 {
		cfg.K3SSANS = infraCfg.Cluster.SANs
	} else if sansEnv := viper.GetString("k3s_sans"); sansEnv != "" {
		cfg.K3SSANS = strings.Split(sansEnv, ",")
	}
	// No else - K3s includes localhost, 127.0.0.1, hostname, and node IPs by default

	// Cluster Init (flag only)
	if clusterInit, _ := cmd.Flags().GetBool("cluster-init"); cmd.Flags().Changed("cluster-init") {
		cfg.ClusterInit = clusterInit
	}

	// Server URL
	if serverURL, _ := cmd.Flags().GetString("server"); serverURL != "" {
		cfg.ServerURL = serverURL
	}

	// Token
	if token, _ := cmd.Flags().GetString("token"); token != "" {
		cfg.Token = token
	} else if infraCfg != nil && infraCfg.Cluster.Token != "" {
		cfg.Token = infraCfg.Cluster.Token
	} else {
		cfg.Token = viper.GetString("k3s_token")
	}

	// Output Kubeconfig
	if output, _ := cmd.Flags().GetString("output-kubeconfig"); output != "" {
		cfg.OutputKubeconfig = output
	} else {
		cfg.OutputKubeconfig = "./kubeconfig"
	}

	return nil
}

// FindNodeByName returns the node with the given name, or nil if absent.
func FindNodeByName(cfg *InfraConfig, name string) *NodeConfig {
	for i := range cfg.Nodes {
		if cfg.Nodes[i].Name == name {
			return &cfg.Nodes[i]
		}
	}
	return nil
}

// FindNodeByAddress returns the node with the given address, or nil if
// absent.
func FindNodeByAddress(cfg *InfraConfig, address string) *NodeConfig {
	for i := range cfg.Nodes {
		if cfg.Nodes[i].Address == address {
			return &cfg.Nodes[i]
		}
	}
	return nil
}

// SetClusterToken writes token into path's cluster.token field, editing the
// parsed node tree rather than a plain struct so comments are preserved.
// Blank lines and indentation aren't preserved (yaml.v3 reformats to its
// own default indent when re-encoding a node tree).
func SetClusterToken(path, token string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}
	if len(doc.Content) == 0 {
		return fmt.Errorf("empty config file: %s", path)
	}

	cluster := mappingValue(doc.Content[0], "cluster")
	if cluster == nil {
		return fmt.Errorf("no top-level 'cluster' key in %s", path)
	}
	tokenNode := mappingValue(cluster, "token")
	if tokenNode == nil {
		return fmt.Errorf("no 'cluster.token' key in %s", path)
	}

	tokenNode.SetString(token)

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("failed to marshal config file: %w", err)
	}
	if err := os.WriteFile(path, out, 0600); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}

	return nil
}

// mappingValue returns the value node for key within mapping node m, or
// nil if m isn't a mapping or doesn't contain key.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func validateConfig(cfg *Config) error {
	if cfg.Node == "" {
		return fmt.Errorf("node is required (use --node with node name from infra.yaml)")
	}

	if cfg.SSHUser == "" || cfg.SSHPassword == "" {
		return fmt.Errorf("node ssh credentials are required (define ssh.user/ssh.password in infra.yaml)")
	}

	// K3s-specific validation (skip for commands like kubeconfig).
	// Note: an empty Token when joining is NOT an error here - the k3s
	// command fetches it automatically from the join server if unset.
	if !cfg.SkipK3sValidation {
		if !cfg.ClusterInit && cfg.ServerURL == "" {
			return fmt.Errorf("server URL required for joining nodes (use --server)")
		}

		if cfg.ClusterInit && (cfg.ServerURL != "" || cfg.Token != "") {
			return fmt.Errorf("--cluster-init cannot be used with --server or --token")
		}
	}

	return nil
}
