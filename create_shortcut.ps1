$ws = New-Object -ComObject WScript.Shell
$startup = [Environment]::GetFolderPath('Startup')
$path = "$startup\ScoreBot.lnk"
$sc = $ws.CreateShortcut($path)
$sc.TargetPath = 'e:\AI_Claude_Projects\ScoreBot-Go\start.bat'
$sc.WorkingDirectory = 'e:\AI_Claude_Projects\ScoreBot-Go'
$sc.WindowStyle = 7
$sc.Save()
Write-Output "Shortcut created: $path"
