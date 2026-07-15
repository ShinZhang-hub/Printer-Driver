#Requires -Version 5.1

$Script:DIR = Split-Path -Parent $MyInvocation.MyCommand.Path
$Script:BINARY = Join-Path $DIR "printer-installer.exe"
$Script:LOG = "$env:TEMP\printer-installer-result.log"
$Script:STATUS_FILE = "$env:TEMP\printer-installer-status.txt"

# --- Self-extract: if EXE not present, decode from embedded base64 ---
if (-not (Test-Path $Script:BINARY)) {
    $selfScript = Get-Content -Path $MyInvocation.MyCommand.Path -Raw
    if ($selfScript -match "`n#__EMBED__`n([A-Za-z0-9+/=]+)") {
        $b64 = $matches[1]
        $exeBytes = [Convert]::FromBase64String($b64)
        $Script:BINARY = Join-Path $env:TEMP "printer-installer-extracted.exe"
        [IO.File]::WriteAllBytes($Script:BINARY, $exeBytes)
    } else {
        [System.Windows.Forms.MessageBox]::Show("EXE not found and no embedded data.", "Error", "OK", "Error")
        exit 1
    }
}

# --- Load UI strings from binary ---
$envStrings = & $BINARY --ui-env 2>$null
$envStrings -split "`n" | ForEach-Object {
    if ($_ -match "^([A-Z_]+)='(.*)'$") {
        Set-Variable -Name $matches[1] -Value $matches[2] -Scope Script
    }
}

# --- Step 1: Discover printer ---
$discovered = & $BINARY --no-snmp --discover 2>$null
$detectedIP = ($discovered | Select-String "^IP=") -replace '^IP='
$detectedLocation = ($discovered | Select-String "^Location=") -replace '^Location='

# --- Resolve detected location ---
$detectedName = ""
$allPrinterNames = ""
$allPrinterIPs = ""
if ($detectedLocation) {
    $resolved = & $BINARY --no-snmp --resolve-location $detectedLocation 2>$null
    $allPrinterNames = ($resolved | Select-String "^Name=" | ForEach-Object { $_ -replace '^Name=' }) -join ','
    $allPrinterIPs = ($resolved | Select-String "^IP=" | ForEach-Object { $_ -replace '^IP=' }) -join ','
    if ($allPrinterNames) { $detectedName = ($allPrinterNames -split ',')[0] }
}

# --- All locations for dropdown ---
$allLocations = & $BINARY --no-snmp --list-locations 2>$null
$locItems = @()
$locItemsNoDetect = @()
if ($allLocations) {
    $locItems = $allLocations -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ }
    $locItemsNoDetect = $locItems | Where-Object { $_ -ne $detectedLocation }
}

# --- All printers + build IP map + conflict map ---
$allPrinters = & $BINARY --debug-printers 2>$null
$printerIPMap = @{}
$locIPMap = @{}
$conflictMap = @{}
$deleteItems = @()

if ($allPrinters) {
    $printerList = $allPrinters -split ',' | ForEach-Object { $_.Trim() } | Where-Object { $_ }
    foreach ($pn in $printerList) {
        # Get printer IP
        $printerInfo = Get-Printer -Name $pn -ErrorAction SilentlyContinue
        $pip = "?"
        if ($printerInfo.PortName) {
            $port = Get-PrinterPort -Name $printerInfo.PortName -ErrorAction SilentlyContinue
            if ($port.PrinterHostAddress) { $pip = $port.PrinterHostAddress }
        }
        $printerIPMap[$pn] = $pip
        $deleteItems += "$pn ($pip)"
    }
}

# Build location IP map and conflict map
foreach ($loc in $locItems) {
    $locResolved = & $BINARY --no-snmp --resolve-location $loc 2>$null
    $locIPs = ($locResolved | Select-String "^IP=" | ForEach-Object { $_ -replace '^IP=' }) -join ','
    $locIPMap[$loc] = $locIPs

    # Check conflict: does any printer exist at these IPs?
    $hits = $false
    foreach ($ip in ($locIPs -split ',')) {
        if (-not $ip) { continue }
        $exist = & $BINARY --printer-at-ip $ip 2>$null
        if ($exist) { $hits = $true; break }
    }
    $conflictMap[$loc] = $hits
}

# --- Load WinForms ---
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

# --- Build single dialog ---
$form = New-Object System.Windows.Forms.Form
$form.Text = $Script:TITLE
$form.Size = New-Object System.Drawing.Size(540, 420)
$form.StartPosition = "CenterScreen"
$form.FormBorderStyle = "FixedDialog"
$form.MaximizeBox = $false
$form.MinimizeBox = $false

# Summary label
$y = 10
$line1 = "$detectedLocation  |  $detectedName  |  IP: $detectedIP"
$lblInfo = New-Object System.Windows.Forms.Label
$lblInfo.Text = $line1
$lblInfo.Location = New-Object System.Drawing.Point(20, $y)
$lblInfo.Size = New-Object System.Drawing.Size(490, 16)
$lblInfo.Font = New-Object System.Drawing.Font("Segoe UI", 9)
$form.Controls.Add($lblInfo)
$y += 22

# Separator 1
$sep1 = New-Object System.Windows.Forms.Label
$sep1.BorderStyle = "Fixed3D"
$sep1.Size = New-Object System.Drawing.Size(490, 2)
$sep1.Location = New-Object System.Drawing.Point(20, $y)
$form.Controls.Add($sep1)
$y += 8

# 1. Location confirm checkbox
$confirmText = $Script:CONFIRM_FMT -f $detectedLocation
$chkConfirm = New-Object System.Windows.Forms.CheckBox
$chkConfirm.Text = $confirmText
$chkConfirm.Location = New-Object System.Drawing.Point(24, $y)
$chkConfirm.Size = New-Object System.Drawing.Size(490, 22)
$chkConfirm.Checked = $true
$chkConfirm.Font = New-Object System.Drawing.Font("Segoe UI", 9)
$form.Controls.Add($chkConfirm)
$y += 26

# 2. Location picker (hidden when checked)
$pickerY = $y
$lblPicker = New-Object System.Windows.Forms.Label
$lblPicker.Text = $Script:PICKER_PROMPT
$lblPicker.Location = New-Object System.Drawing.Point(24, $y)
$lblPicker.Size = New-Object System.Drawing.Size(120, 22)
$lblPicker.Font = New-Object System.Drawing.Font("Segoe UI", 9)
$lblPicker.Visible = $false
$form.Controls.Add($lblPicker)

$cmbPicker = New-Object System.Windows.Forms.ComboBox
$cmbPicker.Location = New-Object System.Drawing.Point(148, $y)
$cmbPicker.Size = New-Object System.Drawing.Size(360, 22)
$cmbPicker.Font = New-Object System.Drawing.Font("Segoe UI", 9)
$cmbPicker.DropDownStyle = "DropDownList"
$cmbPicker.Visible = $false
foreach ($item in $locItemsNoDetect) { [void]$cmbPicker.Items.Add($item) }
if ($cmbPicker.Items.Count -gt 0) { $cmbPicker.SelectedIndex = 0 }
$form.Controls.Add($cmbPicker)
$y += 28

# Separator 2
$sep2 = New-Object System.Windows.Forms.Label
$sep2.BorderStyle = "Fixed3D"
$sep2.Size = New-Object System.Drawing.Size(490, 2)
$sep2.Location = New-Object System.Drawing.Point(20, $y)
$form.Controls.Add($sep2)
$y += 8

# 3. Conflict popup
$conflictY = $y
$lblConflict = New-Object System.Windows.Forms.Label
$lblConflict.Text = $Script:CONFLICT_LABEL
$lblConflict.Location = New-Object System.Drawing.Point(24, $y)
$lblConflict.Size = New-Object System.Drawing.Size(160, 22)
$lblConflict.Font = New-Object System.Drawing.Font("Segoe UI", 9)
$form.Controls.Add($lblConflict)

$cmbConflict = New-Object System.Windows.Forms.ComboBox
$cmbConflict.Location = New-Object System.Drawing.Point(188, $y)
$cmbConflict.Size = New-Object System.Drawing.Size(320, 22)
$cmbConflict.Font = New-Object System.Drawing.Font("Segoe UI", 9)
$cmbConflict.DropDownStyle = "DropDownList"
[void]$cmbConflict.Items.Add($Script:SKIP_BTN)
[void]$cmbConflict.Items.Add($Script:OVERWRITE_LABEL)
$cmbConflict.SelectedIndex = 0
$form.Controls.Add($cmbConflict)
$y += 28

# Separator 3
$sep3 = New-Object System.Windows.Forms.Label
$sep3.BorderStyle = "Fixed3D"
$sep3.Size = New-Object System.Drawing.Size(490, 2)
$sep3.Location = New-Object System.Drawing.Point(20, $y)
$form.Controls.Add($sep3)
$y += 8

# 4. Delete checkboxes
$lblDelete = New-Object System.Windows.Forms.Label
$lblDelete.Text = $Script:CHOOSE_PROMPT
$lblDelete.Location = New-Object System.Drawing.Point(24, $y)
$lblDelete.Size = New-Object System.Drawing.Size(490, 18)
$lblDelete.Font = New-Object System.Drawing.Font("Segoe UI", 9)
$form.Controls.Add($lblDelete)
$y += 22

$delBoxes = @()
foreach ($di in $deleteItems) {
    $parts = $di -split ' \('
    $pn = $parts[0]
    $pip = $printerIPMap[$pn]
    $initDisabled = $false
    if ($allPrinterIPs -split ',' | Where-Object { $_ -eq $pip }) { $initDisabled = $true }

    $cb = New-Object System.Windows.Forms.CheckBox
    $cb.Text = $di
    $cb.Location = New-Object System.Drawing.Point(40, $y)
    $cb.Size = New-Object System.Drawing.Size(470, 20)
    $cb.Font = New-Object System.Drawing.Font("Segoe UI", 9)
    $cb.Enabled = -not $initDisabled
    $cb.Tag = $pn
    $form.Controls.Add($cb)
    $delBoxes += $cb
    $y += 22
}

# OK / Cancel buttons
$y += 8
$btnOK = New-Object System.Windows.Forms.Button
$btnOK.Text = $Script:OK_LABEL
$btnOK.Location = New-Object System.Drawing.Point(340, $y)
$btnOK.Size = New-Object System.Drawing.Size(80, 26)
$btnOK.DialogResult = "OK"
$form.Controls.Add($btnOK)

$btnCancel = New-Object System.Windows.Forms.Button
$btnCancel.Text = $Script:CANCEL_LABEL
$btnCancel.Location = New-Object System.Drawing.Point(430, $y)
$btnCancel.Size = New-Object System.Drawing.Size(80, 26)
$btnCancel.DialogResult = "Cancel"
$form.Controls.Add($btnCancel)

$form.AcceptButton = $btnOK
$form.CancelButton = $btnCancel

# --- Toggle handler: checkbox #1 → show/hide picker ---
$chkConfirm.Add_CheckedChanged({
    $show = -not $chkConfirm.Checked
    $lblPicker.Visible = $show
    $cmbPicker.Visible = $show

    # Update conflict & delete based on chosen location
    $curLoc = if ($chkConfirm.Checked) { $detectedLocation } else { $cmbPicker.SelectedItem }
    if (-not $curLoc) { $curLoc = $detectedLocation }

    # Conflict popup enable
    $hasConflict = $conflictMap[$curLoc]
    $cmbConflict.Enabled = $hasConflict

    # Delete checkboxes: disable if IP matches chosen location
    $curIPs = if ($locIPMap[$curLoc]) { $locIPMap[$curLoc] -split ',' } else { @() }
    foreach ($cb in $delBoxes) {
        $pn = $cb.Tag
        $pip = $printerIPMap[$pn]
        $cb.Enabled = ($curIPs -notcontains $pip)
    }
})

# Also on picker change
$cmbPicker.Add_SelectedIndexChanged({
    if (-not $chkConfirm.Checked) {
        $curLoc = $cmbPicker.SelectedItem
        $hasConflict = $conflictMap[$curLoc]
        $cmbConflict.Enabled = $hasConflict

        $curIPs = if ($locIPMap[$curLoc]) { $locIPMap[$curLoc] -split ',' } else { @() }
        foreach ($cb in $delBoxes) {
            $pn = $cb.Tag
            $pip = $printerIPMap[$pn]
            $cb.Enabled = ($curIPs -notcontains $pip)
        }
    }
})

# --- Show ---
$form.Topmost = $true
$result = $form.ShowDialog()

if ($result -ne "OK") { exit 0 }

# --- Collect results ---
$confirmed = $chkConfirm.Checked
$pickedLoc = if ($cmbPicker.SelectedItem) { $cmbPicker.SelectedItem } else { $detectedLocation }
$overwrite = ($cmbConflict.SelectedIndex -eq 1)
$toDelete = ($delBoxes | Where-Object { $_.Checked } | ForEach-Object { $_.Tag })

$chosenLoc = if ($confirmed) { $detectedLocation } else { $pickedLoc }

# --- Resolve chosen location printers ---
$chosenResolved = & $BINARY --resolve-location $chosenLoc 2>$null
$chosenIPs = ($chosenResolved | Select-String "^IP=" | ForEach-Object { $_ -replace '^IP=' }) -join ','
$chosenNames = ($chosenResolved | Select-String "^Name=" | ForEach-Object { $_ -replace '^Name=' }) -join ','

# --- Build combined install+delete script ---
$skipMsg = ""
$combinedArgs = @()

if ($chosenLoc) {
    if (-not $overwrite) {
        # Check if all printers at chosen location exist
        $skipAll = $true
        foreach ($cip in ($chosenIPs -split ',')) {
            if (-not $cip) { continue }
            $exist = & $BINARY --printer-at-ip $cip 2>$null
            if (-not $exist) { $skipAll = $false; break }
        }
        if ($skipAll) {
            $skipMsg = $Script:SKIP_INSTALL_MSG -f $chosenNames
        } else {
            $combinedArgs += "--location '$chosenLoc'"
        }
    } else {
        $combinedArgs += "--location '$chosenLoc'"
    }
}

# Add delete
if ($toDelete -and $toDelete.Count -gt 0) {
    $chosenFirst = ($chosenNames -split ',')[0]
    $deleteList = $toDelete | Where-Object { $_ -ne $chosenFirst }
    if ($deleteList) {
        $deleteList | Out-File -FilePath "$env:TEMP\printer-installer-delete.txt" -Encoding ascii
        $combinedArgs += "--delete-printers-file `"$env:TEMP\printer-installer-delete.txt`""
    }
}

# --- Execute ---
$scriptRan = $false
if ($combinedArgs.Count -gt 0) {
    $argsStr = $combinedArgs -join " "
    try {
        $proc = Start-Process -FilePath $BINARY -ArgumentList $argsStr -Verb RunAs -Wait -PassThru -WindowStyle Hidden -ErrorAction Stop
        $exitCode = $proc.ExitCode
        $scriptRan = $true
    } catch {
        exit 0
    }

    if ($exitCode -ne 0) {
        $errMsg = "Installation failed with exit code $exitCode"
        [System.Windows.Forms.MessageBox]::Show("$Script:FAIL_PREFIX`n$errMsg", "Printer Installer", "OK", "Error")
        exit 0
    }
}

Remove-Item "$env:TEMP\printer-installer-delete.txt" -Force -ErrorAction SilentlyContinue

# --- Success ---
$successMsg = ""

if ($skipMsg) {
    $successMsg = $skipMsg
} elseif ($scriptRan) {
    if ($overwrite) {
        $successMsg = "$($Script:OVERWRITTEN_MSG -f $chosenNames)"
    } else {
        $successMsg = "$($Script:INSTALLED_LABEL -f $chosenNames)"
    }
}

# Append delete results
if ($toDelete -and $toDelete.Count -gt 0) {
    $chosenFirst = ($chosenNames -split ',')[0]
    $deleted = ($toDelete | Where-Object { $_ -ne $chosenFirst }) -join ', '
    if ($deleted) {
        if ($successMsg) { $successMsg += "`n`n" }
        $successMsg += "$($Script:REMOVED_MSG -f $deleted)"
    }
}

if ($successMsg) {
    [System.Windows.Forms.MessageBox]::Show($successMsg, "Printer Installer", "OK", "Information")
}
