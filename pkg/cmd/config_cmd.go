package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
)

var configCmd = cli.Command{
	Name:  "config",
	Usage: "Manage CLI configuration",
	Commands: []*cli.Command{
		&configInitCmd,
		&configShowCmd,
	},
	HideHelpCommand: true,
}

var configInitCmd = cli.Command{
	Name:            "init",
	Usage:           "Initialize or update CLI configuration",
	Action:          handleConfigInit,
	HideHelpCommand: true,
}

var configShowCmd = cli.Command{
	Name:            "show",
	Usage:           "Show current configuration",
	Action:          handleConfigShow,
	HideHelpCommand: true,
}

func handleConfigInit(_ context.Context, _ *cli.Command) error {
	reader := bufio.NewReader(os.Stdin)
	cfg := loadConfig()

	if len(cfg.Profiles) > 0 {
		fmt.Printf("Existing profiles: %s\n\n", strings.Join(profileNames(cfg), ", "))
	}

	name := prompt(reader, "Profile name", defaultConfigProfile)
	if name == "" {
		return fmt.Errorf("profile name is required")
	}

	current := cfg.Profiles[name]
	apiKey := prompt(reader, "Kernel API key (blank to use KERNEL_API_KEY)", "")
	profileID := prompt(reader, "Kernel browser profile ID (optional)", current.KernelProfileID)
	profileName := prompt(reader, "Kernel browser profile name", current.KernelProfileName)
	if profileID == "" && profileName == "" {
		return fmt.Errorf("Kernel browser profile ID or name is required")
	}
	amazonBaseURL := prompt(reader, "Amazon base URL", firstNonEmpty(current.AmazonBaseURL, defaultAmazonBaseURL))

	current.KernelAPIKey = apiKey
	current.KernelProfileID = profileID
	current.KernelProfileName = profileName
	current.AmazonBaseURL = amazonBaseURL
	cfg.Profiles[name] = current

	defaultDefault := "n"
	if cfg.DefaultProfile == "" || len(cfg.Profiles) == 1 {
		defaultDefault = "y"
	}
	if strings.ToLower(prompt(reader, fmt.Sprintf("Set %q as the default profile? (y/n)", name), defaultDefault)) == "y" {
		cfg.DefaultProfile = name
	}

	if err := writeConfig(cfg); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Printf("Profile %q saved to %s\n", name, getConfigPath())
	return nil
}

func handleConfigShow(_ context.Context, cmd *cli.Command) error {
	cfg := loadConfig()
	active := resolveProfile(cmd)

	fmt.Printf("Config file: %s\n", getConfigPath())
	fmt.Printf("Default profile: %s\n", cfg.DefaultProfile)
	fmt.Printf("Active profile: %s\n\n", active)

	if len(cfg.Profiles) == 0 {
		fmt.Println("No profiles configured. Run 'amzn config init' to set up.")
	}

	table := NewTableWriter(os.Stdout, "PROFILE", "API KEY", "KERNEL PROFILE", "AMAZON URL", "DEFAULT")
	for name, p := range cfg.Profiles {
		key := ""
		if p.KernelAPIKey != "" {
			key = maskKey(p.KernelAPIKey)
		}
		def := ""
		if name == cfg.DefaultProfile {
			def = "*"
		}
		kp := firstNonEmpty(p.KernelProfileName, p.KernelProfileID)
		table.AddRow(name, key, kp, p.AmazonBaseURL, def)
	}
	table.Render()
	return nil
}

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	return input
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
