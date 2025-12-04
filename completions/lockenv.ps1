Register-ArgumentCompleter -Native -CommandName lockenv -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $commands = @('init', 'lock', 'unlock', 'rm', 'ls', 'status', 'passwd', 'diff', 'compact', 'keyring', 'help', 'completion')
    $keyringCmds = @('save', 'delete', 'status')
    $shells = @('bash', 'zsh', 'fish', 'powershell')

    $tokens = $commandAst.ToString() -split '\s+'

    if ($tokens.Count -eq 1) {
        $commands | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
        return
    }

    $cmd = $tokens[1]
    switch ($cmd) {
        'lock' {
            if ($wordToComplete -like '-*') {
                @('-r', '--remove', '--force') | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterName', $_)
                }
            }
        }
        'unlock' {
            if ($wordToComplete -like '-*') {
                @('--force', '--keep-local', '--keep-both') | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterName', $_)
                }
            }
        }
        'keyring' {
            $keyringCmds | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
            }
        }
        'help' {
            $commands | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
            }
        }
        'completion' {
            $shells | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
            }
        }
    }
}
