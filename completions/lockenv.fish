# lockenv fish completions

set -l commands init lock unlock rm ls status passwd diff compact help completion

complete -c lockenv -f

# Commands
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a init -d 'Create a .lockenv vault'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a lock -d 'Encrypt and store files'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a unlock -d 'Decrypt and restore files'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a rm -d 'Remove files from vault'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a ls -d 'Show vault status'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a status -d 'Show vault status'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a passwd -d 'Change vault password'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a diff -d 'Compare vault with local'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a compact -d 'Compact vault'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a help -d 'Show help'
complete -c lockenv -n "not __fish_seen_subcommand_from $commands" -a completion -d 'Generate completions'

# lock flags and files
complete -c lockenv -n "__fish_seen_subcommand_from lock" -s r -d 'Remove original files'
complete -c lockenv -n "__fish_seen_subcommand_from lock" -l remove -d 'Remove original files'
complete -c lockenv -n "__fish_seen_subcommand_from lock" -l force -d 'Lock without confirmation'
complete -c lockenv -n "__fish_seen_subcommand_from lock" -F

# unlock flags
complete -c lockenv -n "__fish_seen_subcommand_from unlock" -l force -d 'Overwrite local files'
complete -c lockenv -n "__fish_seen_subcommand_from unlock" -l keep-local -d 'Keep local versions'
complete -c lockenv -n "__fish_seen_subcommand_from unlock" -l keep-both -d 'Keep both versions'

# help completions
complete -c lockenv -n "__fish_seen_subcommand_from help" -a "$commands"

# completion completions
complete -c lockenv -n "__fish_seen_subcommand_from completion" -a "bash zsh fish"
