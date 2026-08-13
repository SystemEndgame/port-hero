// Package completion renders shell completion scripts for bash, zsh and fish.
package completion

import "fmt"

// Script returns the completion script for the given shell.
func Script(shell string) (string, error) {
	switch shell {
	case "bash":
		return bash(), nil
	case "zsh":
		return zsh(), nil
	case "fish":
		return fish(), nil
	default:
		return "", fmt.Errorf("unsupported shell %q (supported: bash, zsh, fish)", shell)
	}
}

func bash() string {
	return `# bash completion for port-hero
_port_hero() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local opts="--kill --force --restart --why --json --pid --version --help --file --completion"
    if [[ ${cur} == -* ]]; then
        COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
        return 0
    fi
    # Suggest listening ports.
    local ports
    ports=$(port --json 2>/dev/null | grep -o '"Port": [0-9]*' | awk '{print $2}' | sort -un)
    COMPREPLY=( $(compgen -W "${ports}" -- "${cur}") )
}
complete -F _port_hero port port-hero
`
}

func zsh() string {
	return `#compdef port port-hero
# zsh completion for port-hero
_port_hero() {
    local -a opts
    opts=(
      '--kill[graceful kill]'
      '--force[force kill]'
      '--restart[kill and restart]'
      '--why[show causality chain]'
      '--json[json output]'
      '--pid[process by pid]'
      '--file[file lock holders]'
      '--completion[shell completion]'
      '--version[version]'
      '--help[help]'
    )
    if (( CURRENT == 2 )); then
        _describe 'options' opts
    fi
}
compdef _port_hero port port-hero
`
}

func fish() string {
	return `# fish completion for port-hero
complete -c port -s k -l kill -d 'Graceful kill (SIGTERM)'
complete -c port -s F -l force -d 'Force kill (SIGKILL)'
complete -c port -s r -l restart -d 'Kill and restart'
complete -c port -l why -d 'Show causality chain'
complete -c port -s j -l json -d 'JSON output'
complete -c port -l pid -d 'Target a PID'
complete -c port -l file -d 'Show file lock holders'
complete -c port -s v -l version -d 'Version'
complete -c port -s h -l help -d 'Help'
`
}
