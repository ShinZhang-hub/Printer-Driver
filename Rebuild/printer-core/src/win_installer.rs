//! Windows printer operations. Enumerates installed printers and drives the
//! install/delete batch through a single UAC-elevated PowerShell process,
//! mirroring the macOS `osascript` one-shot admin flow. Output follows the
//! same `I-OK/I-FAIL/D-OK/D-FAIL\t<name>[\t<reason>]` tag protocol so the
//! shared `parse_batch_output` parses it unchanged.

use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::{Mutex, OnceLock};

use crate::driver;
use crate::i18n;
use crate::printer::{BatchResult, InstallTarget};

// ── Cached Windows init data ──────────────────────────────────────────────
// A single PowerShell process gathers language + IPs + printers + default
// at startup. Subsequent calls read from cache, avoiding repeated cold-start
// overhead and preventing multiple console windows from flashing.

pub struct WinInit {
    pub lang: String,
    pub local_ips: Vec<String>,
    pub printers: Vec<(String, String)>,
    pub default_printer: String,
}

static WIN_INIT: OnceLock<Mutex<Option<WinInit>>> = OnceLock::new();

fn cache() -> std::sync::MutexGuard<'static, Option<WinInit>> {
    WIN_INIT.get_or_init(|| Mutex::new(None)).lock().unwrap()
}

/// Run the single combined PowerShell query and cache the result.
/// Called once by `initial_state()`.
pub fn run_init() -> WinInit {
    let mut c = Command::new("powershell");
    c.args(["-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden"]);
    hide_console(&mut c);
    let script = r#"
$lang = (Get-WinUserLanguageList)[0].LanguageTag
Write-Output "LANG=$lang"
$ips = @()
try { $ips = (Get-NetIPAddress -AddressFamily IPv4).IPAddress } catch {}
foreach ($ip in $ips) { Write-Output "IP=$ip" }
Write-Output "---"
Get-Printer -ErrorAction SilentlyContinue | ForEach-Object {
  $name = $_.Name
  $port = Get-PrinterPort -Name $_.PortName -ErrorAction SilentlyContinue
  if ($port) {
    $ip = if ($port.Name -match '^IP_(\d+\.\d+\.\d+\.\d+)$') { $matches[1] }
          elseif ($port.HostAddress) { $port.HostAddress } else { $null }
    if ($ip) { Write-Output "PRINTER=$name=$ip" }
  }
}
Write-Output "---"
$def = Get-CimInstance -ClassName Win32_Printer -ErrorAction SilentlyContinue |
       Where-Object { $_.Default } | Select-Object -First 1
if ($def) { Write-Output "DEFAULT=$($def.Name)" }
"#;
    let out = match c.arg("-Command").arg(script).output() {
        Ok(o) if o.status.success() => String::from_utf8_lossy(&o.stdout).to_string(),
        _ => String::new(),
    };

    let mut lang = String::new();
    let mut local_ips = Vec::new();
    let mut printers = Vec::new();
    let mut default_printer = String::new();
    let mut section = 0u8;

    for line in out.lines() {
        let line = line.trim();
        if line == "---" {
            section += 1;
            continue;
        }
        match section {
            0 => {
                if let Some(v) = line.strip_prefix("LANG=") {
                    lang = v.to_string();
                } else if let Some(v) = line.strip_prefix("IP=") {
                    local_ips.push(v.to_string());
                }
            }
            1 => {
                if let Some(v) = line.strip_prefix("PRINTER=") {
                    if let Some((name, ip)) = v.split_once('=') {
                        printers.push((name.to_string(), ip.to_string()));
                    }
                }
            }
            2 => {
                if let Some(v) = line.strip_prefix("DEFAULT=") {
                    default_printer = v.to_string();
                }
            }
            _ => {}
        }
    }

    WinInit { lang, local_ips, printers, default_printer }
}

/// Initialize the cache. Returns the detected language.
pub fn init_cache() -> String {
    let wi = run_init();
    let lang = wi.lang.clone();
    *cache() = Some(wi);
    lang
}

pub fn cached_printers() -> Vec<(String, String)> {
    cache()
        .as_ref()
        .map(|w| w.printers.clone())
        .unwrap_or_default()
}

pub fn cached_lang() -> String {
    cache()
        .as_ref()
        .map(|w| w.lang.clone())
        .unwrap_or_default()
}

pub fn cached_default_printer() -> String {
    cache()
        .as_ref()
        .map(|w| w.default_printer.clone())
        .unwrap_or_default()
}

pub fn cached_local_ips() -> Vec<String> {
    cache()
        .as_ref()
        .map(|w| w.local_ips.clone())
        .unwrap_or_default()
}

/// Append a timestamped line to a persistent debug log in %TEMP%.
/// The per-run work dir is cleaned up, so without this there is no trace of
/// what the elevated script actually did when an install fails.
pub fn debug_log(msg: &str) {
    let path = std::env::temp_dir().join("printer-installer-debug.log");
    let ts = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|d| d.as_secs())
        .unwrap_or(0);
    let mut line = format!("[{}] {}", ts, msg);
    if !line.ends_with('\n') {
        line.push('\n');
    }
    // Best-effort; never fail the install because logging failed.
    // Cap the log at ~512KB by rotating.
    if let Ok(md) = std::fs::metadata(&path) {
        if md.len() > 512 * 1024 {
            let _ = std::fs::remove_file(&path);
        }
    }
    use std::io::Write as _;
    if let Ok(mut f) = std::fs::OpenOptions::new().create(true).append(true).open(&path) {
        let _ = f.write_all(line.as_bytes());
    }
}

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

    // 2. Resolve every target to its INF + model name. No fuzzy fallback:
    // installing with the WRONG driver is worse than a clear error.
    let mut rows: Vec<String> = Vec::new();
    for t in targets {
        let entry = driver::find_model(&drv_entries, &t.model)
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

    // Debug: confirm all work files exist and show plan content.
    for (label, p) in [
        ("plan", &plan_file),
        ("delete", &delete_file),
        ("script", &script_file),
    ] {
        match std::fs::metadata(p) {
            Ok(md) => debug_log(&format!("work {} exists bytes={}", label, md.len())),
            Err(e) => debug_log(&format!("work {} MISSING: {}", label, e)),
        }
    }
    if let Ok(plan_text) = std::fs::read_to_string(&plan_file) {
        for line in plan_text.lines() {
            debug_log(&format!("  plan-row: {}", line.trim()));
        }
    }
    // Debug: dump full script (so a silent-exit can be matched to exact content).
    if let Ok(script_text) = std::fs::read_to_string(&script_file) {
        debug_log(&format!("--- script begin ({}) ---", script_file.display()));
        for line in script_text.lines() {
            debug_log(&format!("  PS: {}", line));
        }
        debug_log("--- script end ---");
    }
    debug_log(&format!("workdir: {}", work.dir.display()));

    // 5. One elevated run — direct if already admin.
    debug_log(&format!(
        "install_batch start: {} targets, {} deletes, elevated={}",
        targets.len(),
        delete.len(),
        is_elevated()
    ));
    for t in targets {
        debug_log(&format!(
            "  target name={} ip={} model={}",
            t.name, t.ip, t.model
        ));
    }
    match run_elevated(&script_file) {
        Ok(true) => {
            debug_log("run_elevated: OK");
        }
        Ok(false) => {
            debug_log("run_elevated: CANCELLED by user");
            let _ = work.cleanup();
            return Err("cancelled".into());
        }
        Err(e) => {
            debug_log(&format!("run_elevated: ERR {}", e));
            let _ = work.cleanup();
            return Err(e);
        }
    }

    // 6. Parse tagged output through the shared parser.
    let out = std::fs::read_to_string(&result_file).unwrap_or_default();
    debug_log(&format!("result.out bytes={}", out.len()));
    for line in out.lines() {
        debug_log(&format!("  result: {}", line.trim()));
    }
    let r = crate::printer::parse_batch_output(&out);
    // On empty result (silent script exit), KEEP the work dir for manual
    // inspection instead of cleaning up. Log its location.
    if out.trim().is_empty() {
        debug_log(&format!(
            "EMPTY RESULT — workdir KEPT for inspection: {}",
            work.dir.display()
        ));
    } else {
        let _ = work.cleanup();
    }

    // Refresh the cached printer/default snapshot so any post-install
    // verification (default-printer self-check in printer::run_install, the
    // confirm command) sees the ACTUAL new state instead of the stale
    // pre-install snapshot.
    let _ = init_cache();

    Ok(r)
}

/// Run a PowerShell script elevated via UAC. `Ok(true)` = ran to completion,
/// `Ok(false)` = user cancelled the prompt, `Err` = failure.
fn run_elevated(script_file: &Path) -> Result<bool, String> {
    if is_elevated() {
        // Capture stdout/stderr (not just status) so script errors are visible
        // in debug_log instead of vanishing — a script can exit 0 while
        // failing every Add-Content under 'Continue'.
        let out = powershell()
            .arg("-File")
            .arg(script_file)
            .output()
            .map_err(|e| e.to_string())?;
        let code = out.status.code().unwrap_or(-1);
        let stdout = String::from_utf8_lossy(&out.stdout).trim().to_string();
        let stderr = String::from_utf8_lossy(&out.stderr).trim().to_string();
        // Cap to keep the log readable.
        let cut = |s: &str| {
            if s.len() > 2000 {
                format!("{}...[truncated]", &s[..2000])
            } else {
                s.to_string()
            }
        };
        debug_log(&format!("direct-run exit={}", code));
        if !stdout.is_empty() {
            debug_log(&format!("direct-run stdout: {}", cut(&stdout)));
        }
        if !stderr.is_empty() {
            debug_log(&format!("direct-run stderr: {}", cut(&stderr)));
        }
        return Ok(out.status.success());
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
  if (-not (Test-Path $inf)) {{ $script:LAST = 'lpadmin'; Add-Content $Result ("DBG`t" + $name + "`tmissing-inf`t" + $inf); return 1 }}
  $pnputilOut = (& pnputil /add-driver $inf 2>&1 | Out-String)
  if ($LASTEXITCODE -ne 0 -and $LASTEXITCODE -ne 5) {{ $script:LAST = 'lpadmin'; Add-Content $Result ("DBG`t" + $name + "`tpnputil-exit=$LASTEXITCODE`t" + $pnputilOut.Trim().Substring(0, [Math]::Min(500, $pnputilOut.Trim().Length))); return 1 }}

  # 3. TCP/IP port
  Remove-PrinterPort -Name $portName -ErrorAction SilentlyContinue
  if (-not (Get-PrinterPort -Name $portName -ErrorAction SilentlyContinue)) {{
    try {{ $null = Add-PrinterPort -Name $portName -PrinterHostAddress $ip -PortNumber $port -ErrorAction Stop }}
    catch {{ $script:LAST = 'lpadmin'; Add-Content $Result ("DBG`t" + $name + "`tadd-port-fail`t" + $_.Exception.Message); return 1 }}
  }}

  # 4. printer queue
  $printuiOut = (& rundll32 printui.dll,PrintUIEntry /if /b $name /f $inf /r $portName /m $model 2>&1 | Out-String)
  if (-not (Get-Printer -Name $name -ErrorAction SilentlyContinue)) {{
    $script:LAST = 'verify'; Add-Content $Result ("DBG`t" + $name + "`tprintui-verify-fail`t" + $printuiOut.Trim().Substring(0, [Math]::Min(500, $printuiOut.Trim().Length))); return 1
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