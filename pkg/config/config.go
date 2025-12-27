package config

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

// InfraConfig represents the complete infra.yaml structure
type InfraConfig struct {
	Cluster ClusterConfig `yaml:"cluster" mapstructure:"cluster"`
	SSH     SSHConfig     `yaml:"ssh" mapstructure:"ssh"`
	Nodes   []NodeConfig  `yaml:"nodes" mapstructure:"nodes"`
}

type ClusterConfig struct {
	VIP   string   `yaml:"vip" mapstructure:"vip"`
	SANs  []string `yaml:"sans" mapstructure:"sans"`
	Token string   `yaml:"token" mapstructure:"token"`
}

type SSHConfig struct {
	User string `yaml:"user" mapstructure:"user"`
	Port int    `yaml:"port" mapstructure:"port"`
}

type NodeConfig struct {
	Name    string            `yaml:"name" mapstructure:"name"`
	Address string            `yaml:"address" mapstructure:"address"`
	Labels  map[string]string `yaml:"labels,omitempty" mapstructure:"labels"`
	SSH     *SSHConfig        `yaml:"ssh,omitempty" mapstructure:"ssh"`
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

	// Skip K3s-specific validation (for commands like clustertoken, kubeconfig)
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
	if cfg.SSH.Port == 0 {
		cfg.SSH.Port = 22
	}

	// SANs default to empty - K3s includes localhost, 127.0.0.1, hostname, and node IPs by default
	// Users should only add the VIP or custom DNS names they actually use
}

// validateInfraConfig validates the infra.yaml structure
func validateInfraConfig(cfg *InfraConfig) error {
	if len(cfg.Nodes) == 0 {
		return fmt.Errorf("at least one node must be defined in infra.yaml")
	}

	// Check for unique node names
	names := make(map[string]bool)
	for _, node := range cfg.Nodes {
		if names[node.Name] {
			return fmt.Errorf("duplicate node name: %s", node.Name)
		}
		names[node.Name] = true
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
		selectedNode = findNodeByName(infraCfg, nodeName)
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

	// SSH User
	if flag, _ := cmd.Flags().GetString("ssh-user"); flag != "" {
		cfg.SSHUser = flag
	} else if selectedNode != nil && selectedNode.SSH != nil && selectedNode.SSH.User != "" {
		cfg.SSHUser = selectedNode.SSH.User
	} else if infraCfg != nil && infraCfg.SSH.User != "" {
		cfg.SSHUser = infraCfg.SSH.User
	} else if env := viper.GetString("ssh_user"); env != "" {
		cfg.SSHUser = env
	}

	// SSH Password (environment variable only)
	cfg.SSHPassword = viper.GetString("ssh_password")

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

// Helper functions
func findNodeByName(cfg *InfraConfig, name string) *NodeConfig {
	for i := range cfg.Nodes {
		if cfg.Nodes[i].Name == name {
			return &cfg.Nodes[i]
		}
	}
	return nil
}

func validateConfig(cfg *Config) error {
	if cfg.Node == "" {
		return fmt.Errorf("node is required (use --node with node name from infra.yaml)")
	}

	if cfg.SSHUser == "" {
		return fmt.Errorf("SSH user is required (use --ssh-user or define in infra.yaml)")
	}

	// Prompt for password if not provided
	if cfg.SSHPassword == "" {
		fmt.Fprintf(os.Stderr, "SSH password for %s@%s: ", cfg.SSHUser, cfg.Node)

		// Open /dev/tty to read password from terminal
		tty, err := os.Open("/dev/tty")
		if err != nil {
			return fmt.Errorf("failed to open terminal: %w", err)
		}
		defer tty.Close()

		password, err := term.ReadPassword(int(tty.Fd()))
		fmt.Fprintln(os.Stderr) // New line after password input
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		cfg.SSHPassword = string(password)
	}

	// K3s-specific validation (skip for commands like clustertoken, kubeconfig)
	if !cfg.SkipK3sValidation {
		if !cfg.ClusterInit && cfg.ServerURL == "" {
			return fmt.Errorf("server URL required for joining nodes (use --server)")
		}

		if !cfg.ClusterInit && cfg.Token == "" {
			return fmt.Errorf("cluster token required for joining nodes (use --token)")
		}

		if cfg.ClusterInit && (cfg.ServerURL != "" || cfg.Token != "") {
			return fmt.Errorf("--cluster-init cannot be used with --server or --token")
		}
	}

	return nil
}
