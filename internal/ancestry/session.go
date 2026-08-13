package ancestry

// detectSession describes the controlling session of the chain, best effort.
func detectSession(nodes []Node) string {
	for _, n := range nodes {
		if n.Supervisor == "ssh" {
			return "SSH session"
		}
		if n.Supervisor == "tmux" {
			return "tmux session"
		}
		if n.Supervisor == "screen" {
			return "screen session"
		}
	}
	// Interactive shell detection: the parent is a shell with a tty.
	if len(nodes) >= 2 {
		switch nodes[1].Name {
		case "bash", "zsh", "fish", "sh", "dash", "ksh":
			return "interactive shell (" + nodes[1].Name + ")"
		}
	}
	return ""
}
