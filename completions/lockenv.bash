_lockenv() {
    local cur prev words cword
    _init_completion || return

    local commands="init lock unlock rm ls status passwd diff compact help completion"

    if [[ $cword -eq 1 ]]; then
        COMPREPLY=($(compgen -W "$commands" -- "$cur"))
        return
    fi

    local cmd="${words[1]}"
    case "$cmd" in
        lock)
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "-r --remove --force" -- "$cur"))
            else
                _filedir
            fi
            ;;
        unlock)
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "--force --keep-local --keep-both" -- "$cur"))
            else
                # Complete with files from vault
                local files
                files=$(lockenv ls 2>/dev/null | grep -E '^\s+[.*]' | sed 's/^.*[.*] //' | sed 's/ (.*//')
                COMPREPLY=($(compgen -W "$files" -- "$cur"))
            fi
            ;;
        rm)
            # Complete with files from vault
            local files
            files=$(lockenv ls 2>/dev/null | grep -E '^\s+[.*]' | sed 's/^.*[.*] //' | sed 's/ (.*//')
            COMPREPLY=($(compgen -W "$files" -- "$cur"))
            ;;
        help)
            COMPREPLY=($(compgen -W "$commands" -- "$cur"))
            ;;
        completion)
            COMPREPLY=($(compgen -W "bash zsh fish" -- "$cur"))
            ;;
    esac
}

complete -F _lockenv lockenv
