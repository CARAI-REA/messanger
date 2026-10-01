package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// DockerAvailable reports whether a Docker daemon is reachable for testcontainers.
// On Colima it also sets TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE so Ryuk can mount
// the in-VM socket (/var/run/docker.sock) instead of the macOS host path.
func DockerAvailable(t *testing.T) bool {
	t.Helper()
	ensureDockerHost()
	ensureColimaTestcontainersEnv()
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func ensureDockerHost() {
	if os.Getenv("DOCKER_HOST") != "" {
		return
	}
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), ".colima/default/docker.sock"),
		filepath.Join(os.Getenv("HOME"), ".colima/docker.sock"),
		"/var/run/docker.sock",
	}
	for _, sock := range candidates {
		if st, err := os.Stat(sock); err == nil && st.Mode()&os.ModeSocket != 0 {
			_ = os.Setenv("DOCKER_HOST", "unix://"+sock)
			return
		}
	}
}

func ensureColimaTestcontainersEnv() {
	host := os.Getenv("DOCKER_HOST")
	colimaSock := strings.Contains(host, ".colima")
	if !colimaSock {
		// Also detect active colima context without DOCKER_HOST.
		if out, err := exec.Command("docker", "context", "show").Output(); err == nil && strings.TrimSpace(string(out)) == "colima" {
			colimaSock = true
		}
	}
	if !colimaSock {
		return
	}
	if os.Getenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE") == "" {
		_ = os.Setenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", "/var/run/docker.sock")
	}
	if os.Getenv("TESTCONTAINERS_RYUK_DISABLED") == "" {
		_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	}
	if os.Getenv("TESTCONTAINERS_HOST_OVERRIDE") == "" {
		if out, err := exec.Command("colima", "list", "-j").Output(); err == nil {
			// {"address":"192.168.64.2", ...}
			s := string(out)
			if i := strings.Index(s, `"address"`); i >= 0 {
				rest := s[i:]
				if j := strings.Index(rest, `"`); j >= 0 {
					rest = rest[j+1:]
					if k := strings.Index(rest, `"`); k >= 0 {
						// wrong parse - use simpler
					}
				}
			}
			// naive extract
			const key = `"address":"`
			if idx := strings.Index(s, key); idx >= 0 {
				rest := s[idx+len(key):]
				if end := strings.Index(rest, `"`); end > 0 {
					addr := rest[:end]
					if addr != "" {
						_ = os.Setenv("TESTCONTAINERS_HOST_OVERRIDE", addr)
					}
				}
			}
		}
	}
}
