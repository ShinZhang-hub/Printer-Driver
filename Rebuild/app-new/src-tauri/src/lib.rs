use serde::{Deserialize, Serialize};
use tauri::Emitter;

#[derive(Debug, Deserialize)]
struct ConfirmRequest {
    location: String,
    overwrite: bool,
    delete: Vec<String>,
    lang: String,
    /// Additional locations to install (from "继续添加").
    #[serde(default)]
    added: Vec<String>,
    /// User-selected printer names for partial install (e.g. Mori 2选1). Empty = all.
    #[serde(default)]
    selected: Vec<String>,
    /// User-selected default printer name. Empty = no default set.
    #[serde(default, rename = "defaultPrinter")]
    default_printer: String,
}

#[derive(Debug, Serialize)]
struct ConfirmResult {
    messages: Vec<printer_core::printer::ResultMessage>,
    skipped_all: bool,
    cancelled: bool,
}

#[derive(Debug, Serialize)]
struct HealthResult {
    healthy: bool,
}

/// Initial state for the confirm dialog. All business logic lives in
/// printer-core so the standalone app and the onboarding app share it.
#[tauri::command]
fn get_initial_state() -> Result<printer_core::InitialState, String> {
    Ok(printer_core::initial_state())
}

/// Execute the confirmed plan (install / overwrite / delete) with one admin
/// prompt. All locations (main + added) are merged into a single install
/// batch so the user only enters the password once.
#[tauri::command]
fn confirm(req: ConfirmRequest) -> Result<ConfirmResult, String> {
    let cfg = printer_core::load_config();
    let lang = if printer_core::i18n::LANGS.contains(&req.lang.as_str()) {
        req.lang
    } else {
        printer_core::i18n::detect()
    };

    // Collect all targets: main location + added locations
    // When location is empty (pure remove mode), only delete list is populated.
    let mut all_targets = if req.location.is_empty() {
        Vec::new()
    } else {
        printer_core::printer::targets_for_location(&cfg, &req.location)
    };
    let all_delete = req.delete.clone();
    let overwrite = req.overwrite;

    for added_loc in &req.added {
        let targets = printer_core::printer::targets_for_location(&cfg, added_loc);
        if targets.is_empty() {
            continue; // skip unknown locations
        }
        // Mark added targets as non-default (only main location's first is default)
        for mut t in targets {
            t.is_default = false;
            all_targets.push(t);
        }
        // Added locations don't participate in delete/overwrite
    }

    // 部分安装：若前端传了 selected，则只保留勾选的打印机
    if !req.selected.is_empty() {
        let sel: std::collections::HashSet<String> = req.selected.iter().cloned().collect();
        all_targets.retain(|t| sel.contains(&t.name));
    }

    if all_targets.is_empty() && all_delete.is_empty() {
        return Err("no operations to perform".into());
    }

    // Apply user's default printer selection
    if !req.default_printer.is_empty() {
        for t in &mut all_targets {
            t.is_default = t.name == req.default_printer;
        }
    } else {
        // No default requested → clear all flags
        for t in &mut all_targets {
            t.is_default = false;
        }
    }

    // Single install call = single admin prompt
    let plan = printer_core::printer::InstallRequest {
        location: req.location.clone(),
        targets: all_targets.clone(),
        overwrite,
        delete: all_delete,
    };

    let requested_default = req.default_printer.clone();

    match printer_core::printer::run_install(&cfg, &lang, &plan) {
        Ok(mut outcome) => {
            // 自检：确保勾选的默认打印机真的成为系统默认（读取 lpstat -d 校验）
            if !requested_default.is_empty() {
                let actual = printer_core::printer::default_printer();
                if actual != requested_default {
                    let _ = printer_core::printer::set_default_printer(&requested_default);
                    let mut actual2 = printer_core::printer::default_printer();
                    if actual2 != requested_default {
                        #[cfg(target_os = "macos")]
                        {
                            let script_path = format!(
                                "/tmp/printer-default-final-{}.sh",
                                std::process::id()
                            );
                            let esc_final = requested_default.replace('\'', "'\\''");
                            let script_content = format!(
                                "#!/bin/bash\nCONSOLE_USER=$(stat -f %Su /dev/console 2>/dev/null || echo \"\")\nif [ -n \"$CONSOLE_USER\" ] && [ \"$CONSOLE_USER\" != \"root\" ]; then sudo -u \"$CONSOLE_USER\" lpoptions -d '{}' 2>/dev/null || true; fi\nlpoptions -d '{}' 2>/dev/null || true\nlpadmin -d '{}' 2>/dev/null || true\n",
                                esc_final, esc_final, esc_final
                            );
                            if std::fs::write(&script_path, &script_content).is_ok() {
                                #[cfg(unix)]
                                {
                                    use std::os::unix::fs::PermissionsExt;
                                    let _ = std::fs::set_permissions(
                                        &script_path,
                                        std::fs::Permissions::from_mode(0o755),
                                    );
                                }
                                let prompt =
                                    printer_core::i18n::t(&lang, "ADMIN_PROMPT", &[]);
                                let _ = printer_core::printer::run_admin_script(
                                    &script_path, &prompt,
                                );
                                let _ = std::fs::remove_file(&script_path);
                                actual2 = printer_core::printer::default_printer();
                            }
                        }
                    }
                    if actual2 != requested_default {
                        // 追加失败信息，避免前端误以为成功
                        outcome.messages.push(printer_core::printer::ResultMessage {
                            kind: "install-failed".into(),
                            text: printer_core::i18n::t(
                                &lang,
                                "INSTALL_FAILED_MSG",
                                &[&requested_default],
                            ) + "："
                                + &printer_core::i18n::t(&lang, "FAIL_CAUSE_DEFAULT", &[]),
                        });
                    }
                }
            }
            Ok(ConfirmResult {
                messages: outcome.messages,
                skipped_all: outcome.skipped_all,
                cancelled: false,
            })
        }
        Err(e) if e == "cancelled" => Ok(ConfirmResult {
            messages: Vec::new(),
            skipped_all: false,
            cancelled: true,
        }),
        Err(e) => Err(e),
    }
}

/// UI strings for a requested language, so the dialog can switch languages
/// live without reloading the window. Copy stays in printer-core.
#[tauri::command]
fn get_strings(lang: String) -> Result<std::collections::HashMap<String, String>, String> {
    Ok(printer_core::i18n::strings(&lang))
}

/// Kick off a background refresh of the shared config. Resolves immediately;
/// the UI renders from the cached (embedded) config and only refreshes when
/// the `config-updated` event arrives.
#[tauri::command]
fn refresh_config(app: tauri::AppHandle) {
    std::thread::spawn(move || {
        let changed = printer_core::config::refresh_config();
        if changed {
            let _ = app.emit_to("main", "config-updated", ());
        }
    });
}

/// Return ALL installed printers with their IPs and whether each is the
/// system default. Used by the Remove tab.
#[tauri::command]
fn get_installed_printers() -> Result<Vec<printer_core::printer::InstalledPrinter>, String> {
    Ok(printer_core::printer::installed_printers())
}

/// Check if the remote config server is reachable.
#[tauri::command]
fn check_server_health() -> Result<HealthResult, String> {
    Ok(HealthResult {
        healthy: printer_core::printer::check_server_health(),
    })
}

/// Terminate the process immediately (same as the window close button).
#[tauri::command]
fn quit(app: tauri::AppHandle) {
    app.exit(0);
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            get_initial_state,
            confirm,
            get_strings,
            refresh_config,
            get_installed_printers,
            check_server_health,
            quit
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
