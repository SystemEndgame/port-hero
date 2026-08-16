package ancestry

import "strings"

// supervisorTable maps process names to their canonical supervisor label.
// Detection is name-based (and command-based for a few special cases such
// as pm2, which runs as "node").
var supervisorTable = map[string]string{
	"systemd":          "systemd",
	"systemd-journald": "systemd",
	"systemd-udevd":    "systemd",
	"launchd":          "launchd",
	"init":             "init",
	"docker":           "docker",
	"dockerd":          "docker",
	"containerd":       "containerd",
	"containerd-shim":  "containerd",
	"runc":             "containerd",
	"cron":             "cron",
	"crond":            "cron",
	"pm2":              "pm2",
	"nodemon":          "nodemon",
	"supervisord":      "supervisor",
	"forever":          "forever",
	"npm":              "npm",
	"yarn":             "yarn",
	"pnpm":             "pnpm",
	"nginx":            "nginx",
	"apache2":          "apache2",
	"httpd":            "apache2",
	"uwsgi":            "uwsgi",
	"gunicorn":         "gunicorn",
	"unicorn":          "unicorn",
	"puma":             "puma",
	"passenger":        "passenger",
	"tmux":             "tmux",
	"screen":           "screen",
	"sshd":             "ssh",
	"ssh-agent":        "ssh",
}

// classifySupervisor returns a canonical supervisor label when the process
// name or command line identifies a process manager, or "" otherwise.
func classifySupervisor(name, command string) string {
	if label, ok := supervisorTable[name]; ok {
		return label
	}
	// pm2 and friends are "node" processes with a marker in argv.
	lowCmd := strings.ToLower(command)
	switch {
	case strings.Contains(lowCmd, "pm2"):
		return "pm2"
	case strings.Contains(lowCmd, "nodemon"):
		return "nodemon"
	case strings.Contains(lowCmd, "forever"):
		return "forever"
	case strings.Contains(lowCmd, "supervisord"):
		return "supervisor"
	case strings.HasPrefix(lowCmd, "systemd --user"):
		return "systemd (user)"
	case strings.Contains(lowCmd, "tmux: server"):
		return "tmux"
	}
	// macOS window server / loginwindow are system-level sources.
	switch name {
	case "WindowServer", "loginwindow":
		return "macOS session"
	}
	return ""
}
