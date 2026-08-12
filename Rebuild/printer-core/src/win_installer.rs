//! Windows printer operations. Enumerates installed printers and drives the
//! install/delete batch through a single UAC-elevated PowerShell process,
//! mirroring the macOS `osascript` one-shot admin flow. Output follows the
//! same `I-OK/I-FAIL/D-OK/D-FAIL\t<name>[\t<reason>]` tag protocol so the
//! shared `parse_batch_output` parses it unchanged.

use std::path::{Path, PathBuf};
use std::process::Command;

use crate::driver;
use crate::i18n;
use crate::printer::{BatchResult, InstallTarget};

/// Base powershell invocation with hidden console friendly flags.
fn powershell() -> Command {
    let mut c = Command::new("powershell");
    c.args(["-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden"]);
    hide_console(&mut c);
    c
}

/// Suppress the console window of a spawned child process. The host app is a
/// GUI (tauri) process; without CREATE_NO_WINDOW the interpreter flashes a
/// black console even for simple queries.
#[cfg(target_os = "windows")]
fn hide_console(c: &mut Command) {
    use std::os::windows::process::CommandExt;
    const CREATE_NO_WINDOW: u32 = 0x0800_0000;
    c.creation_flags(CREATE_NO_WINDOW);
}

#[cfg(not(target_os = "windows"))]
fn hide_console(_c: &mut Command) {}

/// Enumerate installed printers as (name, ip) via Get-Printer +
/// Get-PrinterPort. IP_### ports map straight back to their address.
pub fn printers() -> Vec<(String, String)> {
    let script = r#"
$ErrorActionPreference = 'SilentlyContinue'
Get-Printer | ForEach-Object {
  $name = $_.Name
  $port = Get-PrinterPort -Name $_.PortName -ErrorAction SilentlyContinue
  if ($port) {
    $ip = if ($port.Name -match '^IP_(\d+\.\d+\.\d+\.\d+)$') { $matches[1] }
          elseif ($port.HostAddress) { $port.HostAddress } else { $null }
    if ($ip) { $name + "=" + $ip }
  }
}
"#;
    let out = match powershell().arg("-Command").arg(script).output() {
        Ok(o) if o.status.success() => String::from_utf8_lossy(&o.stdout).to_string(),
        _ => String::new(),
    };
    let mut result = Vec::new();
    for line in out.lines() {
        let line = line.trim();
        if let Some(eq) = line.find('=') {
            let name = line[..eq].trim();
            let ip = line[eq + 1..].trim();
            if !name.is_empty() {
                result.push((name.to_string(), ip.to_string()));
            }
        }
    }
    result
}

/// Whether the current process already runs with admin privileges.
pub fn is_elevated() -> bool {
    match powershell()
        .arg("-Command")
        .arg(concat!(
            "([Security.Principal.WindowsPrincipal]",
            "[Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(",
            "[Security.Principal.WindowsBuiltInRole]::Administrator)"
        ))
        .output()
    {
        Ok(o) => String::from_utf8_lossy(&o.stdout).trim() == "True",
        Err(_) => false,
    }
}

/// Execute a batch install/delete with a single UAC prompt.
pub fn install_batch(
    lang: &str,
    targets: &[InstallTarget],
    delete: &[String],
) -> Result<BatchResult, String> {
    let work = WorkDir::new()?;

    // 1. Unpack embedded drivers and index their INF model entries.
    let drv_dir = driver::unpack_embedded_drivers()?;
    let drv_entries = driver::parse_inf_dir(&drv_dir);

    // 2. Resolve every target to its INF + model name.
    let mut rows: Vec<String> = Vec::new();
    for t in targets {
        let entry = driver::find_model(&drv_entries, &t.model)
            .or_else(|| drv_entries.first())
            .ok_or_else(|| format!("no driver for model '{}'", t.model))?;
        rows.push(format!(
            "{}\t{}\t{}\t{}\t{}\t{}\t{}",
            t.name,
            t.ip,
            t.port,
            t.protocol,
            if t.is_default { "1" } else { "0" },
            inf_name(&entry.inf_file),
            entry.model_name,
        ));
    }

    // 3. Persist plan + delete list for the elevated process.
    let plan_file = work.file("plan.tsv");
    let delete_file = work.file("delete.txt");
    let result_file = work.file("result.out");
    let retry_i = work.file("retry-i.tsv");
    let retry_d = work.file("retry-d.tsv");
    std::fs::write(&plan_file, rows.join("\n")).map_err(|e| e.to_string())?;
    std::fs::write(&delete_file, delete.join("\n")).map_err(|e| e.to_string())?;

    // 4. Generate the admin PowerShell script.
    let prompt = i18n::t(lang, "ADMIN_PROMPT", &[]);
    let script_file = work.file("install.ps1");
    std::fs::write(
        &script_file,
        admin_script(
            &drv_dir,
            &plan_file,
            &delete_file,
            &result_file,
            &retry_i,
            &retry_d,
            &prompt,
        ),
    )
    .map_err(|e| e.to_string())?;

    // 5. One elevated run — direct if already admin.
    match run_elevated(&script_file) {
        Ok(true) => {}
        Ok(false) => {
            let _ = work.cleanup();
            return Err("cancelled".into());
        }
        Err(e) => {
            let _ = work.cleanup();
            return Err(e);
        }
    }

    // 6. Parse tagged output through the shared parser.
    let out = std::fs::read_to_string(&result_file).unwrap_or_default();
    let r = crate::printer::parse_batch_output(&out);
    let _ = work.cleanup();
    Ok(r)
}

/// Run a PowerShell script elevated via UAC. `Ok(true)` = ran to completion,
/// `Ok(false)` = user cancelled the prompt, `Err` = failure.
fn run_elevated(script_file: &Path) -> Result<bool, String> {
    if is_elevated() {
        let st = powershell()
            .arg("-File")
            .arg(script_file)
            .status()
            .map_err(|e| e.to_string())?;
        return Ok(st.success());
    }

    // Launch a tiny wrapper that elevates the real script via
    // Start-Process -Verb RunAs -Wait -PassThru. The wrapper's own exit code
    // encodes the outcome so we can separate "cancelled" from "failed".
    // `-WindowStyle Hidden` (on both Start-Process and the powershell args)
    // keeps the elevated console window from flashing; child console apps
    // (pnputil, printui) then attach to the hidden console instead of
    // spawning their own.
    let wrapper = script_file.with_extension("ps1.launch.ps1");
    let wrapper_src = format!(
        r#"$ErrorActionPreference = 'Stop'
try {{
  $p = Start-Process -FilePath 'powershell.exe' -Verb RunAs -Wait -PassThru `
    -WindowStyle Hidden `
    -ArgumentList @('-NoLogo','-NoProfile','-ExecutionPolicy','Bypass','-WindowStyle','Hidden','-File','"{}"','"{}"')
  if ($p -and $p.ExitCode -eq 0) {{ Write-Output 'UAC_OK'; exit 0 }}
  else {{ Write-Output 'UAC_FAIL'; exit 1 }}
}} catch {{
  Write-Output 'UAC_CANCELLED'
  exit 2
}}
"#,
        script_file.display(),
        script_file.display(),
    );
    std::fs::write(&wrapper, wrapper_src).map_err(|e| e.to_string())?;

    let out = powershell()
        .arg("-File")
        .arg(&wrapper)
        .output()
        .map_err(|e| e.to_string())?;
    let code = out.status.code().unwrap_or(-1);
    match code {
        0 => Ok(true),
        2 => Ok(false),
        _ => {
            let msg = String::from_utf8_lossy(&out.stdout).trim().to_string();
            Err(format!("elevation failed (exit {}): {}", code, msg))
        }
    }
}

/// Build the elevated PowerShell install script body. References only files
/// under the work dir + unpacked drivers, runs the two-round retry and
/// writes the tagged protocol lines to `result`.
fn admin_script(
    drv_dir: &Path,
    plan_file: &Path,
    delete_file: &Path,
    result_file: &Path,
    retry_i: &Path,
    retry_d: &Path,
    _prompt: &str,
) -> String {
    format!(
        r#"param()
$ErrorActionPreference = 'Continue'
$DrvDir = '{drv}'
$PlanFile = '{plan}'
$DeleteFile = '{delete}'
$Result = '{result}'
$RetryI = '{retry_i}'
$RetryD = '{retry_d}'
$script:LAST = 'unknown'
Set-Content -Path $Result -Value ''

function InstallOne([string]$d) {{
  $f = $d -split "`t"
  if ($f.Count -lt 7) {{ $script:LAST = 'lpadmin'; return 1 }}
  $name = $f[0]; $ip = $f[1]; $port = $f[2]; $proto = $f[3]; $isdef = $f[4]
  $infRel = $f[5]; $model = $f[6]
  $inf = Join-Path $DrvDir $infRel
  $portName = "IP_$ip"
  $script:LAST = 'ok'

  # 1. drop anything bound to this port for a clean reinstall
  Get-Printer -ErrorAction SilentlyContinue |
    Where-Object {{ $_.PortName -eq $portName }} |
    ForEach-Object {{ Remove-Printer -Name $_.Name -Confirm:$false -ErrorAction SilentlyContinue }}

  # 2. driver package
  if (-not (Test-Path $inf)) {{ $script:LAST = 'lpadmin'; return 1 }}
  $null = (& pnputil /add-driver $inf 2>&1 | Out-String)
  if ($LASTEXITCODE -ne 0 -and $LASTEXITCODE -ne 5) {{ $script:LAST = 'lpadmin'; return 1 }}

  # 3. TCP/IP port
  Remove-PrinterPort -Name $portName -ErrorAction SilentlyContinue
  if (-not (Get-PrinterPort -Name $portName -ErrorAction SilentlyContinue)) {{
    try {{ $null = Add-PrinterPort -Name $portName -PrinterHostAddress $ip -PortNumber $port -ErrorAction Stop }}
    catch {{ $script:LAST = 'lpadmin'; return 1 }}
  }}

  # 4. printer queue
  $null = (& rundll32 printui.dll,PrintUIEntry /if /b $name /f $inf /r $portName /m $model 2>&1 | Out-String)
  if (-not (Get-Printer -Name $name -ErrorAction SilentlyContinue)) {{
    $script:LAST = 'verify'; return 1
  }}

  # 5. default
  if ($isdef -eq '1') {{
    $null = (& rundll32 printui.dll,PrintUIEntry /y /n $name 2>&1 | Out-String)
    $script:LAST = 'default'
  }}
  $script:LAST = 'ok'
  return 0
}}

function DeleteOne([string]$n) {{
  Remove-Printer -Name $n -Confirm:$false -ErrorAction SilentlyContinue
  if (Get-Printer -Name $n -ErrorAction SilentlyContinue) {{
    $script:LAST = 'delete'; return 1
  }}
  return 0
}}

# Round 1: delete then install; only failures go to the retry files.
if (Test-Path $DeleteFile) {{
  Get-Content $DeleteFile | Where-Object {{ $_.Trim() }} | ForEach-Object {{
    $n = $_.Trim()
    if (DeleteOne $n) {{ Add-Content $RetryD $n }}
    else {{ Add-Content $Result ("D-OK`t" + $n) }}
  }}
}}
if (Test-Path $PlanFile) {{
  Get-Content $PlanFile | Where-Object {{ $_.Trim() }} | ForEach-Object {{
    $spec = $_.Trim()
    $name = ($spec -split "`t")[0]
    if (InstallOne $spec) {{ Add-Content $RetryI $spec }}
    else {{ Add-Content $Result ("I-OK`t" + $name) }}
  }}
}}
# Round 2: verdicts for retries.
Get-Content $RetryD -ErrorAction SilentlyContinue | ForEach-Object {{
  $n = $_.Trim()
  if (DeleteOne $n) {{ Add-Content $Result ("D-FAIL`t" + $n + "`t" + $script:LAST) }}
  else {{ Add-Content $Result ("D-OK`t" + $n) }}
}}
Get-Content $RetryI -ErrorAction SilentlyContinue | ForEach-Object {{
  $spec = $_.Trim()
  $name = ($spec -split "`t")[0]
  if (InstallOne $spec) {{ Add-Content $Result ("I-FAIL`t" + $name + "`t" + $script:LAST) }}
  else {{ Add-Content $Result ("I-OK`t" + $name) }}
}}
Remove-Item $RetryI,$RetryD -ErrorAction SilentlyContinue
exit 0
"#,
        drv = ps_quote(&drv_dir.display().to_string()),
        plan = ps_quote(&plan_file.display().to_string()),
        delete = ps_quote(&delete_file.display().to_string()),
        result = ps_quote(&result_file.display().to_string()),
        retry_i = ps_quote(&retry_i.display().to_string()),
        retry_d = ps_quote(&retry_d.display().to_string()),
    )
}

/// Single-quote a path for embedding into the PowerShell script body.
/// PowerShell single-quoted strings only need `''` for an embedded quote;
/// backslashes are literal, so they must NOT be escaped.
fn ps_quote(s: &str) -> String {
    s.replace('\'', "''")
}

fn inf_name(inf: &str) -> String {
    Path::new(inf)
        .file_name()
        .map(|n| n.to_string_lossy().to_string())
        .unwrap_or_default()
}

/// Scratch dir holding plan/scripts/result; auto-cleaned.
struct WorkDir {
    dir: PathBuf,
}

impl WorkDir {
    fn new() -> Result<WorkDir, String> {
        let dir = std::env::temp_dir().join(format!(
            "printer-installer-run-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&dir);
        std::fs::create_dir_all(&dir).map_err(|e| e.to_string())?;
        Ok(WorkDir { dir })
    }

    fn file(&self, name: &str) -> PathBuf {
        self.dir.join(name)
    }

    fn cleanup(&self) -> std::io::Result<()> {
        std::fs::remove_dir_all(&self.dir)
    }
}