package agent

import (
	"os"
	"testing"
)

func TestSetupHostFS_SetsEnv(t *testing.T) {
	// Clear env vars first
	envVars := []string{"HOST_PROC", "HOST_SYS", "HOST_ETC", "HOST_VAR", "HOST_RUN", "HOST_DEV", "HOST_ROOT"}
	for _, k := range envVars {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}

	SetupHostFS("/hostfs")

	expected := map[string]string{
		"HOST_PROC": "/hostfs/proc",
		"HOST_SYS":  "/hostfs/sys",
		"HOST_ETC":  "/hostfs/etc",
		"HOST_VAR":  "/hostfs/var",
		"HOST_RUN":  "/hostfs/run",
		"HOST_DEV":  "/hostfs/dev",
		"HOST_ROOT": "/hostfs",
	}

	for k, v := range expected {
		got := os.Getenv(k)
		if got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestSetupHostFS_DefaultEmpty(t *testing.T) {
	t.Setenv("HOST_PROC", "")
	os.Unsetenv("HOST_PROC")

	SetupHostFS("")

	if got := os.Getenv("HOST_PROC"); got != "" {
		t.Errorf("HOST_PROC should not be set when hostFS is empty, got %q", got)
	}
}

func TestSetupHostFS_AlreadySet(t *testing.T) {
	t.Setenv("HOST_PROC", "/custom/proc")

	SetupHostFS("/hostfs")

	if got := os.Getenv("HOST_PROC"); got != "/custom/proc" {
		t.Errorf("HOST_PROC should not be overridden, got %q, want /custom/proc", got)
	}
}
