package agent

import (
	"os"
	"path/filepath"
)

// SetupHostFS configures gopsutil environment variables for containerized collection.
// When running inside a container with the host filesystem mounted at hostFS,
// this sets HOST_PROC, HOST_SYS, etc. so gopsutil reads the host's metrics.
func SetupHostFS(hostFS string) {
	if hostFS == "" {
		return
	}
	envMap := map[string]string{
		"HOST_PROC": filepath.Join(hostFS, "proc"),
		"HOST_SYS":  filepath.Join(hostFS, "sys"),
		"HOST_ETC":  filepath.Join(hostFS, "etc"),
		"HOST_VAR":  filepath.Join(hostFS, "var"),
		"HOST_RUN":  filepath.Join(hostFS, "run"),
		"HOST_DEV":  filepath.Join(hostFS, "dev"),
		"HOST_ROOT": hostFS,
	}
	for k, v := range envMap {
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}
