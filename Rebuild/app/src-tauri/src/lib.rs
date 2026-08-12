use serde::{Deserialize, Serialize};

#[derive(Debug, Deserialize)]
struct ConfirmRequest {
    location: String,
    overwrite: bool,
    delete: Vec<String>,
    lang: String,
}

#[derive(Debug, Serialize)]
struct ConfirmResult {
    messages: Vec<String>,
    skipped_all: bool,
    cancelled: bool,
}

/// Initial state for the confirm dialog. All business logic lives in
/// printer-core so the standalone app and the onboarding app share it.
#[tauri::command]
fn get_initial_state() -> Result<printer_core::InitialState, String> {
    Ok(printer_core::initial_state())
}

/// Execute the confirmed plan (install / overwrite / delete) with one admin
/// prompt.
#[tauri::command]
fn confirm(req: ConfirmRequest) -> Result<ConfirmResult, String> {
    let cfg = printer_core::load_config();
    let targets = printer_core::printer::targets_for_location(&cfg, &req.location);
    if targets.is_empty() {
        return Err("location not found in config".into());
    }
    let plan = printer_core::printer::InstallRequest {
        location: req.location,
        targets,
        overwrite: req.overwrite,
        delete: req.delete,
    };
    let lang = if printer_core::i18n::LANGS.contains(&req.lang.as_str()) {
        req.lang
    } else {
        printer_core::i18n::detect()
    };
    match printer_core::printer::run_install(&cfg, &lang, &plan) {
        Ok(outcome) => Ok(ConfirmResult {
            messages: outcome.messages,
            skipped_all: outcome.skipped_all,
            cancelled: false,
        }),
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
            quit
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
