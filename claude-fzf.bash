# claude-fzf.bash - Bash integration for claude-fzf
#
# Add this to your .bashrc:
#   source /path/to/claude-fzf/claude-fzf.bash
#
# Default keybinding: Ctrl-G Ctrl-C (mnemonic: "Go Claude")

CLAUDE_FZF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

__claude_fzf_widget() {
    local selected project_path session_id
    selected=$("${CLAUDE_FZF_DIR}/claude-fzf" --preview 2>/dev/null)

    if [[ -n "$selected" ]]; then
        # Parse output: project_path|session_id
        project_path="${selected%%|*}"
        session_id="${selected##*|}"

        # Build command that cd's into project dir and resumes
        READLINE_LINE="cd ${project_path@Q} && claude --resume ${session_id}"
        READLINE_POINT=${#READLINE_LINE}
    fi
}

# Bind Ctrl-G Ctrl-C
bind -x '"\C-g\C-c": __claude_fzf_widget'

echo "claude-fzf: Press Ctrl-G Ctrl-C to search Claude sessions"
