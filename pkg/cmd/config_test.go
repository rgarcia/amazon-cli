package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func TestResolveKernelAPIKeyUsesConfigOnly(t *testing.T) {
	withTempHome(t)
	t.Setenv("KERNEL_API_KEY", "env-key")
	t.Setenv("AMZN_KERNEL_API_KEY", "amzn-env-key")

	require.NoError(t, writeConfig(&CLIConfig{
		DefaultProfile: "personal",
		Profiles: map[string]ProfileConfig{
			"personal": {KernelAPIKey: "config-key"},
		},
	}))

	got, err := resolveKernelAPIKey(testCommand())
	require.NoError(t, err)
	assert.Equal(t, "config-key", got)
}

func TestResolveKernelAPIKeyIgnoresEnvFallback(t *testing.T) {
	withTempHome(t)
	t.Setenv("KERNEL_API_KEY", "env-key")
	t.Setenv("AMZN_KERNEL_API_KEY", "amzn-env-key")

	require.NoError(t, writeConfig(&CLIConfig{
		DefaultProfile: "personal",
		Profiles: map[string]ProfileConfig{
			"personal": {KernelProfileName: "amazon"},
		},
	}))

	_, err := resolveKernelAPIKey(testCommand())
	require.EqualError(t, err, "no Kernel API key found in the active config profile. Run 'amzn config init' to save one")
}

func TestResolveConfigIgnoresEnvOverrides(t *testing.T) {
	withTempHome(t)
	t.Setenv("AMZN_PROFILE", "work")
	t.Setenv("KERNEL_PROFILE", "work")
	t.Setenv("AMZN_KERNEL_BASE_URL", "https://env.example")
	t.Setenv("KERNEL_BASE_URL", "https://kernel-env.example")
	t.Setenv("AMZN_KERNEL_PROFILE_ID", "env-profile-id")
	t.Setenv("AMZN_KERNEL_PROFILE_NAME", "env-profile-name")

	require.NoError(t, writeConfig(&CLIConfig{
		DefaultProfile: "personal",
		Profiles: map[string]ProfileConfig{
			"personal": {
				KernelAPIKey:      "config-key",
				KernelBaseURL:     "https://config.example",
				KernelProfileID:   "config-profile-id",
				KernelProfileName: "config-profile-name",
			},
			"work": {
				KernelAPIKey: "work-key",
			},
		},
	}))

	cmd := testCommand()
	assert.Equal(t, "personal", resolveProfile(cmd))

	cfg := resolveProfileConfig(cmd)
	assert.Equal(t, "config-profile-id", cfg.KernelProfileID)
	assert.Equal(t, "config-profile-name", cfg.KernelProfileName)
	assert.Equal(t, "https://config.example", resolveKernelBaseURL(cmd))
}

func testCommand() *cli.Command {
	return &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "profile"},
		},
	}
}

func withTempHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}
