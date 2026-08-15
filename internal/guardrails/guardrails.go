// Package guardrails implements the System Protection Shield:
// it refuses to touch critical system processes and protected ports.
package guardrails

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/SystemEndgame/port-hero/internal/config"
	"github.com/SystemEndgame/port-hero/internal/inspector"
)

// cfgMu guards the runtime-configurable protection tables.
var cfgMu sync.RWMutex

// whitelistPorts and whitelistProcesses suppress warning-level violations.
// Critical protections can never be bypassed by the whitelist.
var (
	whitelistPorts     = map[int]bool{}
	whitelistProcesses = map[string]bool{}
)

// Severity levels for violations.
const (
	SeverityCritical = "CRITICAL"
	SeverityWarning  = "WARNING"
)

// Violation describes why a kill action was blocked or flagged.
type Violation struct {
	Severity string // SeverityCritical or SeverityWarning
	Code     string // machine-readable code
	Message  string // human-readable message
}

func (v Violation) String() string {
	return fmt.Sprintf("[%s] %s", v.Severity, v.Message)
}

// Error implements the error interface so violations can surface directly
// as error values in the UI.
func (v Violation) Error() string { return v.String() }

// protectedPorts are well-known system/service ports that must never be
// touched without an explicit --force override.
var protectedPorts = map[int]string{
	7:    "Echo",
	20:   "FTP data",
	21:   "FTP control",
	22:   "SSH (Secure Shell)",
	23:   "Telnet",
	25:   "SMTP (Mail)",
	53:   "DNS (Domain Name System)",
	67:   "DHCP Server",
	68:   "DHCP Client",
	69:   "TFTP",
	80:   "HTTP (World Wide Web)",
	110:  "POP3 (Mail)",
	111:  "RPC portmapper",
	123:  "NTP (Network Time Protocol)",
	135:  "Windows RPC",
	137:  "NetBIOS",
	138:  "NetBIOS",
	139:  "NetBIOS",
	143:  "IMAP (Mail)",
	161:  "SNMP",
	179:  "BGP",
	389:  "LDAP",
	443:  "HTTPS (TLS)",
	445:  "SMB (Windows File Sharing)",
	514:  "Syslog",
	515:  "LPD (Printing)",
	587:  "SMTP Submission",
	631:  "IPP (Printing)",
	873:  "rsync",
	993:  "IMAPS (Mail)",
	995:  "POP3S (Mail)",
	3306: "MySQL/MariaDB",
	3389: "RDP (Remote Desktop)",
	5432: "PostgreSQL",
	6379: "Redis",
	8080: "HTTP-alt (Common dev proxy)",
}

// protectedProcessNames are system daemons that must never be terminated,
// regardless of which port they hold. Kernel threads (comm inside "[...]")
// are handled separately.
var protectedProcessNames = map[string]string{
	"launchd":            "macOS system launcher",
	"systemd":            "Linux system & service manager",
	"init":               "system init",
	"sshd":               "SSH daemon",
	"ssh-agent":          "SSH authentication agent",
	"containerd":         "container runtime",
	"dockerd":            "Docker daemon",
	"dockerd-ent":        "Docker daemon (enterprise)",
	"runc":               "container runtime",
	"cron":               "cron scheduler",
	"crond":              "cron daemon",
	"syslogd":            "system logger",
	"rsyslogd":           "system logger",
	"journald":           "systemd journal daemon",
	"systemd-logind":     "systemd login manager",
	"systemd-journald":   "systemd journal daemon",
	"systemd-udevd":      "systemd device manager",
	"udevd":              "Linux device manager",
	"udisksd":            "disk management daemon",
	"polkitd":            "authorization daemon",
	"dbus-daemon":        "D-Bus message bus",
	"mdnsresponder":      "mDNSResponder (network discovery)",
	"WindowServer":       "macOS window server",
	"loginwindow":        "macOS login window",
	"kernel_task":        "macOS kernel task",
	"hidd":               "HID (input) daemon",
	"mds":                "Spotlight metadata server",
	"mds_stores":         "Spotlight metadata server",
	"bird":               "iCloud sync daemon",
	"nsurlsessiond":      "URL session daemon",
	"securityd":          "macOS security daemon",
	"opendirectoryd":     "directory services daemon",
	"configd":            "configuration daemon",
	"notifyd":            "notification daemon",
	"distnoted":          "distributed notification daemon",
	"cfprefsd":           "preferences daemon",
	"CoreAudio":          "Core Audio daemon",
	"audiomxd":           "audio daemon",
	"bluetoothd":         "Bluetooth daemon",
	"wifi":               "WiFi manager (AirPort)",
	"airportd":           "WiFi daemon",
	"cupsd":              "print daemon",
	"usbd":               "USB daemon",
	"fseventsd":          "file system events daemon",
	"filecoordinationd":  "file coordination daemon",
	"sharedfilelistd":    "shared file list daemon",
	"logd":               "unified logging daemon",
	"sysmond":            "system monitoring daemon",
	"tccd":               "TCC permission daemon",
	"kcm":                "Kerberos daemon",
	"ntpd":               "NTP daemon",
	"chronyd":            "NTP daemon",
	"named":              "BIND DNS server",
	"unbound":            "DNS resolver daemon",
	"dnsmasq":            "DNS/DHCP daemon",
	"avahi-daemon":       "mDNS (Avahi) daemon",
	"NetworkManager":     "Linux network manager",
	"firewalld":          "firewall daemon",
	"iptables":           "firewall",
	"nft":                "firewall",
	"auditd":             "audit daemon",
	"getty":              "login terminal",
	"agetty":             "login terminal",
	"login":              "login",
	"su":                 "substitute user",
	"sudo":               "superuser do",
	"vpnkit":             "Docker VPN kit",
	"com.docker.backend": "Docker Desktop backend",
	"com.docker.vmnetd":  "Docker Desktop vmnet daemon",
	"qemu-system-x86_64": "QEMU VM (often Docker Desktop)",
	"xhyve":              "Docker Desktop VM",
}

// minPID is a heuristic floor: PIDs below this are almost certainly
// system-level and should not be killed by a dev tool.
const minPID = 10

// Self returns the current process PID so the shield can refuse self-kill.
func Self() int { return os.Getpid() }

// IsProtectedProcessName reports whether a process name (comm) is on the
// system-critical list.
func IsProtectedProcessName(name string) (string, bool) {
	reason, ok := protectedProcessNames[name]
	return reason, ok
}

// IsProtectedPort reports whether a port is on the well-known system list.
func IsProtectedPort(port int) (string, bool) {
	reason, ok := protectedPorts[port]
	return reason, ok
}

// IsKernelThread reports whether a process is a kernel thread.
// Kernel threads are listed with their comm wrapped in "[" "]".
func IsKernelThread(name string) bool {
	return strings.HasPrefix(name, "[") && strings.HasSuffix(name, "]")
}

// Configure extends the Safety Shield tables from user configuration.
// Extra protected ports/daemons are merged into the critical tables (they can
// never be bypassed, matching the spirit of the built-in protections).
// Whitelist entries only suppress warning-level violations (protected port,
// low PID) — critical rules always stay active.
//
// It is safe to call multiple times; each call replaces the previous
// whitelist and merges protection entries.
func Configure(cfg *config.Config) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	if cfg == nil {
		whitelistPorts = map[int]bool{}
		whitelistProcesses = map[string]bool{}
		return
	}
	for port, reason := range cfg.Protection.Ports {
		protectedPorts[port] = reason
	}
	for _, name := range cfg.Protection.Daemons {
		if name != "" {
			protectedProcessNames[name] = "user-configured protected process"
		}
	}
	whitelistPorts = make(map[int]bool, len(cfg.Whitelist.Ports))
	for _, p := range cfg.Whitelist.Ports {
		whitelistPorts[p] = true
	}
	whitelistProcesses = make(map[string]bool, len(cfg.Whitelist.Processes))
	for _, name := range cfg.Whitelist.Processes {
		whitelistProcesses[name] = true
	}
}

// IsWhitelistedPort reports whether the port was whitelisted by the user.
func IsWhitelistedPort(port int) bool {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return whitelistPorts[port]
}

// IsWhitelistedProcess reports whether the process name was whitelisted.
func IsWhitelistedProcess(name string) bool {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return whitelistProcesses[name]
}

// Check evaluates every safety rule for killing the given process.
//
//   - force  bypasses "warning" level violations (protected ports, low PIDs).
//   - Critical violations (PID 1, kernel threads, system daemons, self-kill,
//     foreign processes) can never be bypassed.
//
// It returns the list of violations that still apply after applying force,
// plus the list of violations that were bypassed by force.
func Check(p *inspector.Process, force bool) (active, bypassed []Violation) {
	if p == nil {
		return []Violation{{
			Severity: SeverityCritical,
			Code:     "NO_PROCESS",
			Message:  "No process was found for this port.",
		}}, nil
	}

	own := Self()

	// ---- CRITICAL (never bypassed) ----

	if p.PID <= 0 {
		active = append(active, Violation{
			Severity: SeverityCritical,
			Code:     "INVALID_PID",
			Message:  fmt.Sprintf("Refusing: invalid PID %d.", p.PID),
		})
	}

	if p.PID == own {
		active = append(active, Violation{
			Severity: SeverityCritical,
			Code:     "SELF_KILL",
			Message:  "Refusing: this is Port Hero's own process. I will not help you commit suicide.",
		})
	}

	if p.PID == 1 {
		active = append(active, Violation{
			Severity: SeverityCritical,
			Code:     "PID_ONE",
			Message:  "Refusing: PID 1 is the system init process (launchd/systemd). Terminating it would crash the machine.",
		})
	}

	if IsKernelThread(p.Name) {
		active = append(active, Violation{
			Severity: SeverityCritical,
			Code:     "KERNEL_THREAD",
			Message:  fmt.Sprintf("Refusing: %s is a kernel thread and cannot be terminated.", p.Name),
		})
	}

	if reason, ok := IsProtectedProcessName(p.Name); ok {
		active = append(active, Violation{
			Severity: SeverityCritical,
			Code:     "PROTECTED_PROCESS",
			Message:  fmt.Sprintf("Refusing: %s (%s) is a critical system process.", p.Name, reason),
		})
	}

	// Do not kill processes we do not own unless we are root.
	if p.User != "" && p.User != currentUser() && !isRoot() {
		active = append(active, Violation{
			Severity: SeverityCritical,
			Code:     "FOREIGN_PROCESS",
			Message:  fmt.Sprintf("Refusing: %s belongs to user %q. Only its owner or root may terminate it.", p.Name, p.User),
		})
	}

	// ---- WARNING (bypassable with --force) ----
	warn := warningViolations(p)

	if force {
		bypassed = append(bypassed, warn...)
	} else {
		active = append(active, warn...)
	}

	return active, bypassed
}

// warningViolations collects the bypassable warning-level violations for a
// process, honouring user whitelists.
func warningViolations(p *inspector.Process) []Violation {
	var warn []Violation

	if p.PID > 0 && p.PID < minPID && !IsWhitelistedProcess(p.Name) {
		warn = append(warn, Violation{
			Severity: SeverityWarning,
			Code:     "LOW_PID",
			Message:  fmt.Sprintf("PID %d is very low — this may be a system process.", p.PID),
		})
	}

	if reason, ok := IsProtectedPort(p.Port); ok && !IsWhitelistedPort(p.Port) {
		warn = append(warn, Violation{
			Severity: SeverityWarning,
			Code:     "PROTECTED_PORT",
			Message:  fmt.Sprintf("Port %d (%s) is a well-known system service port.", p.Port, reason),
		})
	}

	return warn
}

// HasCritical reports whether any active violation is critical.
func HasCritical(violations []Violation) bool {
	for _, v := range violations {
		if v.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

// CurrentUser returns the username of the current process owner.
func CurrentUser() string {
	u := os.Getenv("USER")
	if u == "" {
		u = os.Getenv("LOGNAME")
	}
	return u
}

// currentUser returns the username of the current process owner.
func currentUser() string { return CurrentUser() }

// isRoot reports whether we are running with elevated privileges.
func isRoot() bool { return os.Geteuid() == 0 }
