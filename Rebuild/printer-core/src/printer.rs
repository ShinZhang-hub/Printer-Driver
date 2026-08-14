//! Printer operations. Platform-specific; macOS (CUPS) implemented, Windows
//! to follow. All functions are plain subprocess orchestration so the same
//! business logic can be reused by both the standalone app and the onboarding
//! app (via Tauri commands).

use crate::config::Config;
use crate::i18n;

/// A single printer to install.
#[derive(Debug, Clone)]
pub struct InstallTarget {
    pub name: String,
    pub ip: String,
    pub model: String,
    pub port: u16,
    pub protocol: String,
    pub is_default: bool,
}

/// Plan produced from the confirmed dialog.
#[derive(Debug, Clone)]
pub struct InstallRequest {
    /// Chosen location name (already resolved by the UI).
    pub location: String,
    pub targets: Vec<InstallTarget>,
    /// Overwrite = remove existing queues at the target IPs before installing.
    pub overwrite: bool,
    /// Additional printer names to delete (user checked them).
    pub delete: Vec<String>,
}

/// Result messages to show (localized, one per action).
#[derive(Debug, Clone, serde::Serialize)]
pub struct InstallOutcome {
    /// Ordered, grouped messages. `kind` lets the UI visually group install
    /// vs remove blocks (e.g. a divider between them).
    pub messages: Vec<ResultMessage>,
    /// True if nothing was executed because everything was skipped.
    pub skipped_all: bool,
}

/// A single result line for the dialog result page.
#[derive(Debug, Clone, serde::Serialize)]
pub struct ResultMessage {
    /// "installed" | "skipped" | "install-failed" | "removed" | "remove-failed"
    pub kind: String,
    pub text: String,
}

/// Resolve a location into install targets, ordered with first as default.
pub fn targets_for_location(cfg: &Config, location: &str) -> Vec<InstallTarget> {
    let mut out = Vec::new();
    if let Some(loc) = cfg.location_by_name(location) {
        let port = if loc.port_number > 0 {
            loc.port_number
        } else {
            cfg.port_number
        };
        let protocol = if loc.protocol.is_empty() {
            &cfg.protocol
        } else {
            &loc.protocol
        };
        for (i, p) in loc.all_printers().iter().enumerate() {
            out.push(InstallTarget {
                name: p.name.clone(),
                ip: p.ip.clone(),
                model: p.model.clone(),
                port,
                protocol: protocol.clone(),
                is_default: i == 0,
            });
        }
    }
    out
}

/// A printer that failed, with the step that failed (reason code).
#[derive(Debug, Default, Clone)]
pub struct FailedPrinter {
    pub name: String,
    pub reason: String,
}

/// Result of a two-round install/delete plan, keyed by printer name.
#[derive(Debug, Default)]
pub struct BatchResult {
    pub installed: Vec<String>,
    pub failed: Vec<FailedPrinter>,
    pub deleted: Vec<String>,
    pub delete_failed: Vec<FailedPrinter>,
}

/// Execute the plan. Returns localized result messages.
pub fn run_install(
    cfg: &Config,
    lang: &str,
    req: &InstallRequest,
) -> Result<InstallOutcome, String> {
    let mut messages: Vec<ResultMessage> = Vec::new();
    let mut skipped = Vec::new();
    let mut to_install = Vec::new();

    // ONE `lpstat -v` pass for the whole plan; reuse it for every existence
    // check instead of spawning a subprocess per printer (CUPS cold start).
    let by_ip = printers_by_ip();

    for t in &req.targets {
        let exists = by_ip.get(&t.ip);
        if exists.is_some() {
            if !req.overwrite {
                skipped.push(t.name.clone());
            } else {
                to_install.push(t.clone());
            }
        } else {
            to_install.push(t.clone());
        }
    }

    // Filter delete list: never delete the first printer of the chosen
    // location (the one we are installing as default).
    let first_name = req.targets.first().map(|t| t.name.clone()).unwrap_or_default();
    let mut to_delete: Vec<String> = req
        .delete
        .iter()
        .filter(|d| d.trim() != first_name)
        .cloned()
        .collect();
    // Overwrite: also delete the existing printer at each target IP so the
    // reinstall is clean. Folded into the same two-round delete pass.
    if req.overwrite {
        for t in &req.targets {
            if let Some(existing) = by_ip.get(&t.ip) {
                if !to_delete.contains(existing) {
                    to_delete.push(existing.clone());
                }
            }
        }
    }

    let skipped_all = to_install.is_empty() && to_delete.is_empty();

    if skipped_all {
        if !skipped.is_empty() {
            messages.push(ResultMessage {
                kind: "skipped".into(),
                text: i18n::t(lang, "SKIP_INSTALL_MSG", &[&skipped.join(", ")]),
            });
        }
        return Ok(InstallOutcome {
            messages,
            skipped_all: true,
        });
    }

    let r = install_batch(cfg, lang, &to_install, &to_delete)?;

    if req.overwrite {
        if !r.installed.is_empty() {
            messages.push(ResultMessage {
                kind: "installed".into(),
                text: i18n::t(lang, "OVERWRITTEN_MSG", &[&r.installed.join(", ")]),
            });
        }
    } else if !r.installed.is_empty() {
        messages.push(ResultMessage {
            kind: "installed".into(),
            text: i18n::t(lang, "INSTALLED_LABEL", &[&r.installed.join(", ")]),
        });
    }
    for s in skipped {
        messages.push(ResultMessage {
            kind: "skipped".into(),
            text: i18n::t(lang, "SKIP_INSTALL_MSG", &[&s]),
        });
    }
    for f in &r.failed {
        messages.push(ResultMessage {
            kind: "install-failed".into(),
            text: failed_text(lang, "INSTALL_FAILED_MSG", f),
        });
    }
    if !r.deleted.is_empty() {
        messages.push(ResultMessage {
            kind: "removed".into(),
            text: i18n::t(lang, "REMOVED_MSG", &[&r.deleted.join(", ")]),
        });
    }
    for f in &r.delete_failed {
        messages.push(ResultMessage {
            kind: "remove-failed".into(),
            text: failed_text(lang, "REMOVE_FAILED_MSG", f),
        });
    }
    Ok(InstallOutcome {
        messages,
        skipped_all: false,
    })
}

/// Localized failure line: `❌ <name> 两次尝试后仍安装失败：<reason>`.
/// Falls back to the generic key when the reason code is unknown.
fn failed_text(lang: &str, key: &str, f: &FailedPrinter) -> String {
    let cause_key = match f.reason.as_str() {
        "lpadmin" => "FAIL_CAUSE_LPADMIN",
        "verify" => "FAIL_CAUSE_VERIFY",
        "enable" => "FAIL_CAUSE_ENABLE",
        "accept" => "FAIL_CAUSE_ACCEPT",
        "default" => "FAIL_CAUSE_DEFAULT",
        "delete" => "FAIL_CAUSE_DELETE",
        _ => "FAIL_CAUSE_UNKNOWN",
    };
    let cause = i18n::t(lang, cause_key, &[]);
    let base = i18n::t(lang, key, &[&f.name]);
    format!("{base}：{cause}")
}

/// List installed printers as (name, ip). IP is "" when unknown.
pub fn list_printers_with_ips() -> Vec<(String, String)> {
    imp::printers()
}

/// ONE platform printer-enumeration pass, returned as ip -> name for O(1)
/// conflict lookups. macOS pays a CUPS cold-start cost per `lpstat` spawn,
/// so the initial-state builder calls this exactly once and reuses it.
pub fn printers_by_ip() -> std::collections::HashMap<String, String> {
    let mut m = std::collections::HashMap::new();
    for (name, ip) in list_printers_with_ips() {
        m.insert(ip, name);
    }
    m
}

#[cfg(target_os = "macos")]
mod imp {
    use super::*;

    pub const PPD: &[u8] = include_bytes!("../assets/ff-mac-driver.ppd");

    /// Install the batch + deletes with a single admin prompt (osascript).
    pub fn install_batch(
        _cfg: &Config,
        lang: &str,
        targets: &[InstallTarget],
        delete: &[String],
    ) -> Result<BatchResult, String> {
        // Write PPD + batch script to temp files (readable by root).
        let ppd_path = format!("/tmp/printer-installer-{}.ppd", std::process::id());
        std::fs::write(&ppd_path, PPD).map_err(|e| e.to_string())?;

        // Unpack the embedded FF driver (filter + PDEs) to a temp dir; the
        // admin script copies it into /Library/Printers/FUJIFILM as root.
        let drv_tmp = format!("/tmp/printer-installer-drv-{}", std::process::id());
        let drv_src = crate::mac_driver::unpack_to(std::path::Path::new(&drv_tmp))?;

        let pid = std::process::id();
        let retry_i = format!("/tmp/printer-installer-retry-i-{pid}");
        let retry_d = format!("/tmp/printer-installer-retry-d-{pid}");

        let script_path = format!("/tmp/printer-installer-{pid}.sh");
        let mut lines: Vec<String> = vec![
            "#!/bin/bash".into(),
            "set -u".into(),
            format!("PPD={}", shell_escape(&ppd_path)),
            format!("DRV_SRC={}", shell_escape(drv_src.to_str().unwrap_or(""))),
            format!("RETRY_I='{retry_i}'"),
            format!("RETRY_D='{retry_d}'"),
            ": > \"$RETRY_I\"".into(),
            ": > \"$RETRY_D\"".into(),
            // Install the embedded FF driver (filter + PDEs) so the PPD's
            // cupsFilter / APDialogExtension paths resolve on fresh machines.
            "mkdir -p /Library/Printers/FUJIFILM".into(),
            "ditto \"$DRV_SRC\" /Library/Printers/FUJIFILM".into(),
            // CUPS (runs as _lp) refuses filters not owned by root:wheel.
            "chown -R root:wheel /Library/Printers/FUJIFILM".into(),
            "chmod -R go-w /Library/Printers/FUJIFILM".into(),
            "chmod 555 /Library/Printers/FUJIFILM/Filter/FFACMMCFilter 2>/dev/null".into(),
            // Records which step failed; the final attempt's reason wins.
            "LAST_REASON=".into(),
            "install_one() {".into(),
            "  local d=\"$1\" name ip port proto isdef".into(),
            "  IFS=$'\\t' read -r name ip port proto isdef <<< \"$d\"".into(),
            "  if ! lpadmin -E -p \"$name\" -v \"socket://$ip:$port/$proto\" -P \"$PPD\" 2>/dev/null; then LAST_REASON=lpadmin; return 1; fi".into(),
            "  if ! lpstat -p \"$name\" >/dev/null 2>&1; then LAST_REASON=verify; return 1; fi".into(),
            "  if ! cupsenable \"$name\" 2>/dev/null; then LAST_REASON=enable; return 1; fi".into(),
            "  if ! cupsaccept \"$name\" 2>/dev/null; then LAST_REASON=accept; return 1; fi".into(),
            "  if [ \"$isdef\" = \"1\" ] && ! lpadmin -d \"$name\" 2>/dev/null; then LAST_REASON=default; return 1; fi".into(),
            "  LAST_REASON=ok".into(),
            "  return 0".into(),
            "}".into(),
            "delete_one() {".into(),
            "  if ! lpadmin -x \"$1\" 2>/dev/null; then LAST_REASON=delete; return 1; fi".into(),
            "  LAST_REASON=ok".into(),
            "  return 0".into(),
            "}".into(),
        ];

        // Round 1: delete, then install — failures only recorded into retry
        // files (no final verdict yet); the next printer keeps going.
        // Retry files store the RAW tab-separated spec (no shell escaping),
        // since round 2 reads them back with `read` rather than evaluating.
        for name in delete {
            let e = shell_escape(name);
            lines.push(format!(
                "delete_one '{e}' && printf 'D-OK\\t{name}\\n' || printf '%s\\n' '{}' >> \"$RETRY_D\"",
                name
            ));
        }
        for t in targets {
            let n = &t.name;
            let spec = format!(
                "{}\t{}\t{}\t{}\t{}",
                t.name,
                t.ip,
                t.port,
                t.protocol,
                if t.is_default { "1" } else { "0" }
            );
            let se = shell_escape(&spec);
            lines.push(format!(
                "install_one '{se}' && printf 'I-OK\\t{n}\\n' || printf '%s\\n' '{spec}' >> \"$RETRY_I\""
            ));
        }

        // Round 2: retry only what failed in round 1, in order. These lines
        // carry the final verdict + the failing step for the UI. Fields are
        // tab-separated so printer names containing spaces survive parsing.
        lines.push("while IFS= read -r d; do".into());
        lines.push("  delete_one \"$d\" && printf 'D-OK\\t%s\\n' \"$d\" || printf 'D-FAIL\\t%s\\t%s\\n' \"$d\" \"$LAST_REASON\"".into());
        lines.push("done < \"$RETRY_D\"".into());
        lines.push("while IFS= read -r spec; do".into());
        lines.push("  name=\"${spec%%$'\\t'*}\"".into());
        lines.push("  install_one \"$spec\" && printf 'I-OK\\t%s\\n' \"$name\" || printf 'I-FAIL\\t%s\\t%s\\n' \"$name\" \"$LAST_REASON\"".into());
        lines.push("done < \"$RETRY_I\"".into());
        lines.push("rm -f \"$RETRY_I\" \"$RETRY_D\"".into());
        lines.push("exit 0".into());
        let script = lines.join("\n");
        std::fs::write(&script_path, script).map_err(|e| e.to_string())?;

        let out = run_admin_script(&script_path, &crate::i18n::t(lang, "ADMIN_PROMPT", &[]))?;
        let _ = std::fs::remove_file(&script_path);
        let _ = std::fs::remove_file(&ppd_path);
        let _ = std::fs::remove_dir_all(&drv_tmp);
        Ok(super::parse_batch_output(&out))
    }

    pub fn run_admin_script(script_path: &str, prompt: &str) -> Result<String, String> {
        let escaped_prompt = prompt.replace('\\', "\\\\").replace('"', "\\\"");
        let applet = format!(
            "do shell script \"bash '{}'\" with administrator privileges with prompt \"{}\"",
            shell_escape(script_path),
            escaped_prompt
        );
        let out = std::process::Command::new("osascript")
            .arg("-e")
            .arg(applet)
            .output()
            .map_err(|e| e.to_string())?;
        if out.status.success() {
            return Ok(String::from_utf8_lossy(&out.stdout).trim().to_string());
        }
        let msg = String::from_utf8_lossy(&out.stderr).trim().to_string();
        // User cancelling the password dialog must not surface as an error.
        if msg.to_lowercase().contains("cancel") || msg.contains("-128") {
            return Err("cancelled".into());
        }
        Err(msg)
    }

    pub fn shell_escape(s: &str) -> String {
        format!("'{}'", s.replace('\'', "'\\''"))
    }

    /// Enumerate installed printers via `lpstat -v`.
    pub fn printers() -> Vec<(String, String)> {
        let out = run("lpstat", &["-v"]);
        let mut result = Vec::new();
        for line in out.lines() {
            let name = extract_printer_name(line);
            if name.is_empty() {
                continue;
            }
            result.push((name.to_string(), socket_host(line).unwrap_or_default()));
        }
        result
    }

    fn run(cmd: &str, args: &[&str]) -> String {
        match std::process::Command::new(cmd).args(args).output() {
            Ok(o) => String::from_utf8_lossy(&o.stdout).to_string(),
            Err(_) => String::new(),
        }
    }

    fn socket_host(line: &str) -> Option<String> {
        let idx = line.find("socket://")?;
        let rest = &line[idx + "socket://".len()..];
        let end = rest
            .find([':', ' ', '\n', '\t'])
            .unwrap_or(rest.len());
        Some(rest[..end].to_string())
    }

    /// Extract the printer name from an `lpstat -v` line. Handles localized
    /// output (e.g. zh: `用于Printer-BG的设备：socket://...`) by finding the
    /// last `:` / full-width `：` separator before the URI, then taking the
    /// last ASCII word — mirroring the Go `extractPrinterNameBeforeURI`.
    fn extract_printer_name(line: &str) -> String {
        let uri_idx = match line.find("://") {
            Some(i) => i,
            None => return String::new(),
        };
        let prefix = &line[..uri_idx];
        let mut sep_idx = prefix.rfind(':');
        if let Some(ff) = prefix.rfind('：') {
            sep_idx = Some(ff.max(sep_idx.unwrap_or(0)));
        }
        let prefix = match sep_idx {
            Some(i) => &prefix[..i],
            None => prefix,
        };
        let mut last_word = String::new();
        let mut cur = String::new();
        for c in prefix.chars() {
            if c.is_ascii() && !c.is_whitespace() {
                cur.push(c);
            } else if !cur.is_empty() {
                last_word = std::mem::take(&mut cur);
            }
        }
        if !cur.is_empty() {
            last_word = cur;
        }
        last_word
    }
}

#[cfg(target_os = "windows")]
mod imp {
    use super::*;

    pub fn install_batch(
        _cfg: &Config,
        lang: &str,
        targets: &[InstallTarget],
        delete: &[String],
    ) -> Result<BatchResult, String> {
        crate::win_installer::install_batch(lang, targets, delete)
    }

    pub fn printers() -> Vec<(String, String)> {
        crate::win_installer::printers()
    }

    pub fn run_admin_script(_script_path: &str, _prompt: &str) -> Result<String, String> {
        unreachable!("run_admin_script is macOS-only")
    }
}

#[cfg(all(not(target_os = "windows"), not(target_os = "macos")))]
mod imp {
    use super::*;

    pub fn install_batch(
        _cfg: &Config,
        _lang: &str,
        _targets: &[InstallTarget],
        _delete: &[String],
    ) -> Result<BatchResult, String> {
        Err("not implemented on this platform".into())
    }

    pub fn printers() -> Vec<(String, String)> {
        Vec::new()
    }

    pub fn run_admin_script(_script_path: &str, _prompt: &str) -> Result<String, String> {
        unreachable!("run_admin_script is macOS-only")
    }
}

pub use imp::{install_batch, run_admin_script};

/// Parse `I-OK/I-FAIL/D-OK/D-FAIL\t<name>[\t<reason>]` lines emitted by the
/// batch script. Tab-separated so printer names containing spaces survive.
/// osascript converts the script's LF line endings to CR, so normalize both.
pub(crate) fn parse_batch_output(out: &str) -> BatchResult {
    let mut r = BatchResult::default();
    for line in out.replace('\r', "\n").lines() {
        let mut parts = line.split('\t');
        let tag = parts.next().unwrap_or("").trim();
        let rest = parts.next().unwrap_or("");
        let reason = parts.next().unwrap_or("");
        if rest.is_empty() {
            continue;
        }
        match tag {
            "I-OK" => r.installed.push(rest.to_string()),
            "I-FAIL" => r.failed.push(FailedPrinter {
                name: rest.to_string(),
                reason: reason.to_string(),
            }),
            "D-OK" => r.deleted.push(rest.to_string()),
            "D-FAIL" => r.delete_failed.push(FailedPrinter {
                name: rest.to_string(),
                reason: reason.to_string(),
            }),
            _ => {}
        }
    }
    r
}
