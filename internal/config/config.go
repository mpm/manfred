package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config holds all MANFRED configuration.
type Config struct {
	DataDir     string `mapstructure:"data_dir"`
	ProjectsDir string `mapstructure:"projects_dir"`
	JobsDir     string `mapstructure:"jobs_dir"`
	TicketsDir  string `mapstructure:"tickets_dir"`

	Database    DatabaseConfig    `mapstructure:"database"`
	Credentials CredentialsConfig `mapstructure:"credentials"`
	Claude      ClaudeConfig      `mapstructure:"claude"`
	GitHub      GitHubConfig      `mapstructure:"github"`
	Server      ServerConfig      `mapstructure:"server"`
	Logging     LoggingConfig     `mapstructure:"logging"`
}

// DatabaseConfig holds database settings.
type DatabaseConfig struct {
	Path string `mapstructure:"path"` // Path to SQLite database file
}

// ClaudeConfig holds Claude Code related settings.
type ClaudeConfig struct {
	BundlePath string `mapstructure:"bundle_path"` // Path to claude-bundle tarball or directory
}

// CredentialsConfig holds credential-related settings.
type CredentialsConfig struct {
	AnthropicAPIKey        string `mapstructure:"anthropic_api_key"`
	ClaudeCredentialsFile  string `mapstructure:"claude_credentials_file"`
}

// ServerConfig holds web server settings.
type ServerConfig struct {
	Addr string `mapstructure:"addr"`
	Port int    `mapstructure:"port"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// GitHubConfig holds GitHub integration settings.
type GitHubConfig struct {
	Token           string `mapstructure:"token"`             // Personal Access Token
	WebhookSecret   string `mapstructure:"webhook_secret"`    // Webhook signature secret
	RateLimitBuffer int    `mapstructure:"rate_limit_buffer"` // Stop when this many requests remain
}

// ProjectConfig holds per-project configuration from project.yml.
type ProjectConfig struct {
	Name          string       `yaml:"name"`
	Repo          string       `yaml:"repo"`
	DefaultBranch string       `yaml:"default_branch"`
	Docker        DockerConfig `yaml:"docker"`
}

// DockerConfig holds Docker-related project settings.
type DockerConfig struct {
	ComposeFile string `yaml:"compose_file"`
	MainService string `yaml:"main_service"`
	Workdir     string `yaml:"workdir"`
}

// Load reads configuration from file, environment, and defaults.
func Load() (*Config, error) {
	cfg := &Config{}

	// Set defaults
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	defaultDataDir := filepath.Join(home, ".manfred")

	viper.SetDefault("data_dir", defaultDataDir)
	viper.SetDefault("server.addr", "127.0.0.1")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")

	// Unmarshal into struct
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply defaults for derived paths
	if cfg.DataDir == "" {
		cfg.DataDir = defaultDataDir
	}
	if cfg.ProjectsDir == "" {
		cfg.ProjectsDir = filepath.Join(cfg.DataDir, "projects")
	}
	if cfg.JobsDir == "" {
		cfg.JobsDir = filepath.Join(cfg.DataDir, "jobs")
	}
	if cfg.TicketsDir == "" {
		cfg.TicketsDir = filepath.Join(cfg.DataDir, "tickets")
	}
	if cfg.Credentials.ClaudeCredentialsFile == "" {
		cfg.Credentials.ClaudeCredentialsFile = filepath.Join(cfg.DataDir, "config", ".credentials.json")
	}
	if cfg.Claude.BundlePath == "" {
		cfg.Claude.BundlePath = filepath.Join(cfg.DataDir, "claude-bundle")
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = filepath.Join(cfg.DataDir, "manfred.db")
	}

	// Override with environment variables
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		cfg.Credentials.AnthropicAPIKey = key
	}
	if dir := os.Getenv("MANFRED_DATA_DIR"); dir != "" {
		cfg.DataDir = dir
	}
	if dir := os.Getenv("MANFRED_PROJECTS_DIR"); dir != "" {
		cfg.ProjectsDir = dir
	}
	if dir := os.Getenv("MANFRED_JOBS_DIR"); dir != "" {
		cfg.JobsDir = dir
	}
	if dir := os.Getenv("MANFRED_TICKETS_DIR"); dir != "" {
		cfg.TicketsDir = dir
	}
	if path := os.Getenv("MANFRED_DATABASE_PATH"); path != "" {
		cfg.Database.Path = path
	}

	// GitHub configuration from environment
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		cfg.GitHub.Token = token
	}
	if secret := os.Getenv("MANFRED_WEBHOOK_SECRET"); secret != "" {
		cfg.GitHub.WebhookSecret = secret
	}
	if cfg.GitHub.RateLimitBuffer == 0 {
		cfg.GitHub.RateLimitBuffer = 100
	}

	return cfg, nil
}

// ProjectConfig loads the configuration for a specific project.
func (c *Config) ProjectConfig(name string) (*ProjectConfig, error) {
	projectYml := filepath.Join(c.ProjectsDir, name, "project.yml")

	data, err := os.ReadFile(projectYml)
	if err != nil {
		return nil, fmt.Errorf("failed to read project config: %w", err)
	}

	var projCfg ProjectConfig
	if err := yaml.Unmarshal(data, &projCfg); err != nil {
		return nil, fmt.Errorf("failed to parse project config: %w", err)
	}

	// Apply defaults
	if projCfg.Docker.ComposeFile == "" {
		projCfg.Docker.ComposeFile = "docker-compose.yml"
	}
	if projCfg.Docker.MainService == "" {
		projCfg.Docker.MainService = "app"
	}
	if projCfg.Docker.Workdir == "" {
		projCfg.Docker.Workdir = "/app"
	}
	if projCfg.DefaultBranch == "" {
		projCfg.DefaultBranch = "main"
	}

	return &projCfg, nil
}

// ProjectRepositoryPath returns the path to the project's repository.
func (c *Config) ProjectRepositoryPath(name string) string {
	return filepath.Join(c.ProjectsDir, name, "repository")
}

// EnsureDirectories creates all required directories.
func (c *Config) EnsureDirectories() error {
	dirs := []string{
		c.DataDir,
		c.ProjectsDir,
		c.JobsDir,
		c.TicketsDir,
		filepath.Dir(c.Credentials.ClaudeCredentialsFile),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// ClaudeCredentialsExist checks if Claude credentials file exists.
func (c *Config) ClaudeCredentialsExist() bool {
	_, err := os.Stat(c.Credentials.ClaudeCredentialsFile)
	return err == nil
}
