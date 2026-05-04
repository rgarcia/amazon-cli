package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/urfave/cli/v3"
)

const (
	defaultConfigProfile  = "personal"
	defaultAmazonBaseURL  = "https://www.amazon.com"
	defaultConfigFilePerm = 0600
)

type ProfileConfig struct {
	KernelAPIKey      string `koanf:"kernel_api_key"`
	KernelBaseURL     string `koanf:"kernel_base_url"`
	KernelProfileID   string `koanf:"kernel_profile_id"`
	KernelProfileName string `koanf:"kernel_profile_name"`
	AmazonBaseURL     string `koanf:"amazon_base_url"`
}

type CLIConfig struct {
	DefaultProfile string                   `koanf:"default_profile"`
	Profiles       map[string]ProfileConfig `koanf:"profiles"`
}

func getConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "amzn")
}

func getConfigPath() string {
	dir := getConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.yaml")
}

func loadConfig() *CLIConfig {
	cfg := &CLIConfig{Profiles: make(map[string]ProfileConfig)}
	k := koanf.New(".")
	if p := getConfigPath(); p != "" {
		_ = k.Load(file.Provider(p), yaml.Parser())
	}
	_ = k.Unmarshal("", cfg)
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]ProfileConfig)
	}
	return cfg
}

func resolveProfile(cmd *cli.Command) string {
	if p := cmd.Root().String("profile"); p != "" {
		return p
	}
	if p := os.Getenv("AMZN_PROFILE"); p != "" {
		return p
	}
	if p := os.Getenv("KERNEL_PROFILE"); p != "" {
		return p
	}
	cfg := loadConfig()
	if cfg.DefaultProfile != "" {
		return cfg.DefaultProfile
	}
	return defaultConfigProfile
}

func resolveProfileConfig(cmd *cli.Command) ProfileConfig {
	cfg := loadConfig()
	p := cfg.Profiles[resolveProfile(cmd)]
	if v := os.Getenv("AMZN_KERNEL_PROFILE_ID"); v != "" {
		p.KernelProfileID = v
	}
	if v := os.Getenv("AMZN_KERNEL_PROFILE_NAME"); v != "" {
		p.KernelProfileName = v
	}
	if p.AmazonBaseURL == "" {
		p.AmazonBaseURL = defaultAmazonBaseURL
	}
	return p
}

func resolveKernelAPIKey(cmd *cli.Command) (string, error) {
	if k := os.Getenv("AMZN_KERNEL_API_KEY"); k != "" {
		return k, nil
	}
	if k := os.Getenv("KERNEL_API_KEY"); k != "" {
		return k, nil
	}
	cfg := resolveProfileConfig(cmd)
	if cfg.KernelAPIKey != "" {
		return cfg.KernelAPIKey, nil
	}
	return "", fmt.Errorf("no Kernel API key found. Set KERNEL_API_KEY, AMZN_KERNEL_API_KEY, or run 'amzn config init'")
}

func resolveKernelBaseURL(cmd *cli.Command) string {
	if u := os.Getenv("AMZN_KERNEL_BASE_URL"); u != "" {
		return u
	}
	if u := os.Getenv("KERNEL_BASE_URL"); u != "" {
		return strings.Trim(u, `"`)
	}
	cfg := resolveProfileConfig(cmd)
	if cfg.KernelBaseURL != "" {
		return cfg.KernelBaseURL
	}
	return ""
}

func writeConfig(cfg *CLIConfig) error {
	dir := getConfigDir()
	if dir == "" {
		return fmt.Errorf("could not determine home directory")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	var b strings.Builder
	if cfg.DefaultProfile != "" {
		fmt.Fprintf(&b, "default_profile: %s\n", cfg.DefaultProfile)
	}
	if len(cfg.Profiles) > 0 {
		b.WriteString("profiles:\n")
		for name, p := range cfg.Profiles {
			fmt.Fprintf(&b, "  %s:\n", name)
			if p.KernelAPIKey != "" {
				fmt.Fprintf(&b, "    kernel_api_key: %q\n", p.KernelAPIKey)
			}
			if p.KernelBaseURL != "" {
				fmt.Fprintf(&b, "    kernel_base_url: %q\n", p.KernelBaseURL)
			}
			if p.KernelProfileID != "" {
				fmt.Fprintf(&b, "    kernel_profile_id: %q\n", p.KernelProfileID)
			}
			if p.KernelProfileName != "" {
				fmt.Fprintf(&b, "    kernel_profile_name: %q\n", p.KernelProfileName)
			}
			if p.AmazonBaseURL != "" {
				fmt.Fprintf(&b, "    amazon_base_url: %q\n", p.AmazonBaseURL)
			}
		}
	}

	return os.WriteFile(getConfigPath(), []byte(b.String()), defaultConfigFilePerm)
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

func profileNames(cfg *CLIConfig) []string {
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	return names
}
