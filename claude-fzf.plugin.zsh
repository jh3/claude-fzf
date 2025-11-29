# claude-fzf.plugin.zsh - Zsh integration for claude-fzf
#
# Add this to your .zshrc:
#   source /path/to/claude-fzf/claude-fzf.plugin.zsh
#
# Or if you use a plugin manager, add the directory to your plugins.
#
# Default keybinding: Ctrl-G Ctrl-C (mnemonic: "Go Claude")
# Override with: CLAUDE_FZF_KEY="^x^c" before sourcing

CLAUDE_FZF_DIR="${0:A:h}"
CLAUDE_FZF_KEY="${CLAUDE_FZF_KEY:-^g^c}"

# Widget function for zle
claude-fzf-widget() {
    local selected project_path session_id
    selected=$("${CLAUDE_FZF_DIR}/claude-fzf" --preview 2>/dev/null)

    if [[ -n "$selected" ]]; then
        # Parse output: project_path|session_id
        project_path="${selected%%|*}"
        session_id="${selected##*|}"

        # Build command that cd's into project dir and resumes
        BUFFER="cd ${(q)project_path} && claude --resume ${session_id}"
        zle accept-line
    fi
    zle reset-prompt
}

zle -N claude-fzf-widget
bindkey "${CLAUDE_FZF_KEY}" claude-fzf-widget

echo "claude-fzf: Press ${CLAUDE_FZF_KEY} to search Claude sessions"
