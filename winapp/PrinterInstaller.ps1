#Requires -Version 5.1

$Script:DIR = Split-Path -Parent $MyInvocation.MyCommand.Path
$Script:BINARY = Join-Path $DIR "printer-installer.exe"
$Script:LOG = "$env:TEMP\printer-installer-result.log"
$Script:STATUS_FILE = "$env:TEMP\printer-installer-status.txt"

# --- Load UI strings from binary ---
$envStrings = & $BINARY --ui-env 2>$null
$envStrings -split "`n" | ForEach-Object {
    if ($_ -match "^([A-Z_]+)='(.*)'$") {
        Set-Variable -Name $matches[1] -Value $matches[2] -Scope Script
    }
}

# --- Load Windows.Forms for dialogs ---
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName Microsoft.VisualBasic

function Show-MsgBox {
    param([string]$Text, [string]$Title = "", [string]$Buttons = "OK", [string]$Icon = "None")
    $btn = switch ($Buttons) {
        "OK"       { [System.Windows.Forms.MessageBoxButtons]::OK }
        "YesNo"    { [System.Windows.Forms.MessageBoxButtons]::YesNo }
        "OKCancel" { [System.Windows.Forms.MessageBoxButtons]::OKCancel }
        default    { [System.Windows.Forms.MessageBoxButtons]::OK }
    }
    $ico = switch ($Icon) {
        "Info"    { [System.Windows.Forms.MessageBoxIcon]::Information }
        "Warning" { [System.Windows.Forms.MessageBoxIcon]::Warning }
        "Error"   { [System.Windows.Forms.MessageBoxIcon]::Error }
        "Question"{ [System.Windows.Forms.MessageBoxIcon]::Question }
        default   { [System.Windows.Forms.MessageBoxIcon]::None }
    }
    return [System.Windows.Forms.MessageBox]::Show($Text, $Title, $btn, $ico)
}

function Show-Popup {
    param([string]$Text, [string]$Title = "", [int]$Timeout = 0)
    $wshell = New-Object -ComObject WScript.Shell
    return $wshell.Popup($Text, $Timeout, $Title, 64)  # 64 = Information
}

function Show-DeleteDialog {
    param([string]$OtherNames, [string]$InstalledName)

    $names = $OtherNames -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ -and $_ -ne $InstalledName }
    if (-not $names) { return }

    $items = @()
    foreach ($n in $names) {
        $loc = & $BINARY --printer-location $n 2>$null
        if ($loc) { $items += "$loc`: $n" } else { $items += $n }
    }
    if (-not $items) { return }

    $result = Show-MsgBox -Text $Script:DEL_MSG -Title "Printer Installer" -Buttons "OKCancel" -Icon "Question"
    if ($result -ne "OK") { return }

    $selected = $items | Out-GridView -Title $Script:CHOOSE_PROMPT -OutputMode Multiple
    if (-not $selected) { return }

    # Strip location prefix to get actual printer names
    $cleanNames = $selected | ForEach-Object {
        if ($_ -match ':\s(.+)$') { $matches[1] } else { $_ }
    }
    $cleanNames | Out-File -FilePath "$env:TEMP\printer-installer-delete.txt" -Encoding ascii

    $delProc = Start-Process -FilePath $BINARY -ArgumentList "--delete-printers-file `"$env:TEMP\printer-installer-delete.txt`"" -Verb RunAs -Wait -PassThru -WindowStyle Hidden 2>$null
    if (-not $delProc -or $delProc.ExitCode -ne 0) {
        Remove-Item "$env:TEMP\printer-installer-delete.txt" -Force -ErrorAction SilentlyContinue
        return
    }
    Remove-Item "$env:TEMP\printer-installer-delete.txt" -Force -ErrorAction SilentlyContinue

    $deleted = $selected -join ', '
    Show-MsgBox -Text "$Script:DELETED_PREFIX`n$deleted" -Title "Printer Installer" -Icon "Info"
}

# --- Step 1: Discover printer ---
$discovered = & $BINARY --discover 2>$null
$detectedIP = ($discovered | Select-String "^IP=") -replace '^IP='
$detectedModel = ($discovered | Select-String "^Model=") -replace '^Model='
$detectedLocation = ($discovered | Select-String "^Location=") -replace '^Location='

# --- Step 2: Location confirmation ---
$locationArg = ""
$installName = ""
$installIP = ""

if ($detectedLocation) {
    $confirmMsg = $Script:CONFIRM_FMT -f $detectedLocation
    $confirmResult = Show-MsgBox -Text $confirmMsg -Title "Printer Installer" -Buttons "YesNo" -Icon "Question"
    if ($confirmResult -eq "Yes") {
        $locationArg = "--location '$detectedLocation'"
        $resolved = & $BINARY --resolve-location $detectedLocation 2>$null
        $installName = ($resolved | Select-String "^Name=") -replace '^Name='
        $installIP = ($resolved | Select-String "^IP=") -replace '^IP='
    }
}

# --- Step 3: Location picker (if not confirmed) ---
if (-not $locationArg) {
    $locations = & $BINARY --list-locations 2>$null
    if ($locations) {
        $locList = $locations -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ }
        if ($locList) {
            $picked = $locList | Out-GridView -Title $Script:PICKER_PROMPT -OutputMode Single
            if ($picked) {
                $locationArg = "--location '$picked'"
                $resolved = & $BINARY --resolve-location $picked 2>$null
                $installName = ($resolved | Select-String "^Name=") -replace '^Name='
                $installIP = ($resolved | Select-String "^IP=") -replace '^IP='
            }
        }
    } else {
        $apt = [Microsoft.VisualBasic.Interaction]::InputBox($Script:NAME_PROMPT, "Printer Installer", $detectedModel)
        if ($apt) {
            $locationArg = "--name '$apt'"
            $installName = $apt
            $installIP = $detectedIP
            if ($detectedIP) { $locationArg += " --ip '$detectedIP'" }
        }
    }
}

# User cancelled → exit
if (-not $locationArg) { exit 0 }

# --- Step 4: Conflict detection ---
if ($installIP) {
    $existingName = & $BINARY --printer-at-ip $installIP 2>$null
    if ($existingName) {
        $conflictMsg = $Script:CONFLICT_FMT -f $installIP, $existingName
        $conflictResult = Show-MsgBox -Text $conflictMsg -Title "Printer Installer" -Buttons "YesNo" -Icon "Warning"
        if ($conflictResult -ne "Yes") { exit 0 }  # No = Skip
    }
}

# --- Step 4: Install (elevated) ---
try {
    $installProc = Start-Process -FilePath $BINARY -ArgumentList $locationArg -Verb RunAs -Wait -PassThru -WindowStyle Hidden -ErrorAction Stop 2>$null
    $exitCode = $installProc.ExitCode
} catch {
    # User cancelled UAC → exit silently
    exit 0
}

if ($exitCode -ne 0) {
    # Actual installation failure
    $errMsg = "Installation failed with exit code $exitCode"
    Show-MsgBox -Text "$Script:FAIL_PREFIX`n$errMsg" -Title "Printer Installer" -Icon "Error"
    exit 0
}

# --- Step 5: Post-install ---
$rawMsg = ""
if (Test-Path $STATUS_FILE) {
    $rawMsg = (Get-Content $STATUS_FILE -Raw) -replace '"', ''
} else {
    $rawMsg = "Printer installed successfully"
}

# Translate Go output
$dialogMsg = $rawMsg -replace " installed$", $Script:INSTALLED_LABEL
$dialogMsg = $dialogMsg -replace "^Other printers: ", $Script:OTHER_PRINTERS_LABEL
$dialogMsg = "✅ $dialogMsg$($Script:AUTO_CLOSE)"

Show-Popup -Text $dialogMsg -Title "Printer Installer" -Timeout 5

# Check for other printers → delete dialog
$otherLine = $rawMsg | Select-String "Other printers: "
if ($otherLine) {
    $otherVal = ($otherLine -replace '.*Other printers: ').Trim()
    $firstLine = ($rawMsg -split "`n")[0].Trim()
    $installedName = $firstLine -replace ' installed$'
    Show-DeleteDialog -OtherNames $otherVal -InstalledName $installedName
}
