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
#[derive(Debug, Clone)]
pub struct InstallOutcome {
    pub messages: Vec<String>,
    /// True if nothing was executed because everything was skipped.
    pub skipped_all: bool,
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

/// Execute the plan. Returns localized result messages.
pub fn run_install(
    _cfg: &Config,
    lang: &str,
    req: &InstallRequest,
) -> Result<InstallOutcome, String> {
    let mut messages = Vec::new();
    let mut skipped = Vec::new();
    let mut to_install = Vec::new();

    for t in &req.targets {
        let exists = find_printer_by_ip(&t.ip);
        if !req.overwrite && exists.is_some() {
            skipped.push(t.name.clone());
        } else if req.overwrite && exists.is_some() {
            to_install.push(t.clone());
        } else {
            to_install.push(t.clone());
        }
    }

    // Filter delete list: never delete the first printer of the chosen
    // location (the one we are installing as default).
    let first_name = req.targets.first().map(|t| t.name.clone()).unwrap_or_default();
    let to_delete: Vec<String> = req
        .delete
        .iter()
        .filter(|d| d.trim() != first_name)
        .cloned()
        .collect();

    let skipped_all = to_install.is_empty() && to_delete.is_empty();

    if skipped_all {
        if !skipped.is_empty() {
            messages.push(i18n::t(
                lang,
                "SKIP_INSTALL_MSG",
                &[&skipped.join(", ")],
            ));
        }
        return Ok(InstallOutcome {
            messages,
            skipped_all: true,
        });
    }

    install_batch(&to_install, req.overwrite, &to_delete)?;

    if req.overwrite {
        messages.push(i18n::t(
            lang,
            "OVERWRITTEN_MSG",
            &[&to_install.iter().map(|t| t.name.clone()).collect::<Vec<_>>().join(", ")],
        ));
    } else if !to_install.is_empty() {
        let names: Vec<String> = to_install.iter().map(|t| t.name.clone()).collect();
        messages.push(i18n::t(lang, "INSTALLED_LABEL", &[&names.join(", ")]));
    }
    for s in skipped {
        messages.push(i18n::t(lang, "SKIP_INSTALL_MSG", &[&s]));
    }
    if !to_delete.is_empty() {
        messages.push(i18n::t(
            lang,
            "REMOVED_MSG",
            &[&to_delete.join(", ")],
        ));
    }
    Ok(InstallOutcome {
        messages,
        skipped_all: false,
    })
}

/// List installed printers as (name, ip). IP is "" when unknown.
pub fn list_printers_with_ips() -> Vec<(String, String)> {
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

pub fn find_printer_by_ip(ip: &str) -> Option<String> {
    let out = run("lpstat", &["-v"]);
    let needle = format!("://{}:", ip);
    for line in out.lines() {
        if line.contains(&needle) {
            let name = extract_printer_name(line);
            if !name.is_empty() {
                return Some(name.to_string());
            }
        }
    }
    None
}

fn socket_host(line: &str) -> Option<String> {
    let idx = line.find("socket://")?;
    let rest = &line[idx + "socket://".len()..];
    let end = rest
        .find(|c: char| c == ':' || c == ' ' || c == '\n' || c == '\t')
        .unwrap_or(rest.len());
    Some(rest[..end].to_string())
}

/// Extract the printer name from an `lpstat -v` line. Handles localized
/// output (e.g. zh: `用于Printer-BG的设备：socket://...`) by finding the last
/// `:` / full-width `：` separator before the URI, then taking the last ASCII
/// word — mirroring the original Go `extractPrinterNameBeforeURI`.
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
        } else {
            if !cur.is_empty() {
                last_word = std::mem::take(&mut cur);
            }
        }
    }
    if !cur.is_empty() {
        last_word = cur;
    }
    last_word
}

#[cfg(target_os = "macos")]
mod imp {
    use super::*;

    pub const PPD: &[u8] = include_bytes!("../assets/ff-mac-driver.ppd");

    /// Install the batch + deletes with a single admin prompt (osascript).
    pub fn install_batch(
        targets: &[InstallTarget],
        overwrite: bool,
        delete: &[String],
    ) -> Result<(), String> {
        // Write PPD + batch script to temp files (readable by root).
        let ppd_path = format!("/tmp/printer-installer-{}.ppd", std::process::id());
        std::fs::write(&ppd_path, PPD).map_err(|e| e.to_string())?;

        let script_path = format!("/tmp/printer-installer-{}.sh", std::process::id());
        let mut lines: Vec<String> = vec![
            "#!/bin/bash".into(),
            format!("PPD='{}'", shell_escape(&ppd_path)),
        ];
        for name in delete {
            lines.push(format!("lpadmin -x {} || true", shell_escape(name)));
        }
        for t in targets {
            if overwrite {
                if let Some(existing) = find_printer_by_ip(&t.ip) {
                    lines.push(format!("lpadmin -x {} || true", shell_escape(&existing)));
                }
            }
            lines.push(format!(
                "lpadmin -E -p {} -v 'socket://{}:{}/{}' -P \"$PPD\"",
                shell_escape(&t.name),
                shell_escape(&t.ip),
                t.port,
                t.protocol
            ));
            lines.push(format!("cupsenable {}", shell_escape(&t.name)));
            lines.push(format!("cupsaccept {}", shell_escape(&t.name)));
            if t.is_default {
                lines.push(format!("lpadmin -d {}", shell_escape(&t.name)));
            }
        }
        lines.push("exit 0".into());
        let script = lines.join("\n");
        std::fs::write(&script_path, script).map_err(|e| e.to_string())?;

        let result = run_admin_script(
            &script_path,
            &crate::i18n::t(&crate::i18n::detect(), "ADMIN_PROMPT", &[]),
        );
        let _ = std::fs::remove_file(&script_path);
        let _ = std::fs::remove_file(&ppd_path);
        result
    }

    pub fn run_admin_script(script_path: &str, prompt: &str) -> Result<(), String> {
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
            return Ok(());
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
}

#[cfg(not(target_os = "macos"))]
mod imp {
    use super::*;

    pub fn install_batch(
        _targets: &[InstallTarget],
        _overwrite: bool,
        _delete: &[String],
    ) -> Result<(), String> {
        Err("not implemented on this platform".into())
    }
}

pub use imp::{install_batch, run_admin_script};

fn run(cmd: &str, args: &[&str]) -> String {
    match std::process::Command::new(cmd).args(args).output() {
        Ok(o) => String::from_utf8_lossy(&o.stdout).to_string(),
        Err(_) => String::new(),
    }
}
