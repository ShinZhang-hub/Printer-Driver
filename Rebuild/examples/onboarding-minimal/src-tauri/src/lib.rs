use serde::Deserialize;
use tauri::Emitter;

/// 入职指引前端把「选择的位置 + 覆盖/删除选项」作为请求传来，
/// 转成 printer-core 的 InstallRequest 再执行。
#[derive(Deserialize)]
struct InstallRequestDto {
    location: String,
    overwrite: bool,
    delete: Vec<String>,
}

/// 拿 printer-core 计算好的初始状态（位置识别 / 冲突 / 删除列表 / 文案）。
/// 直接复用 flow::initial_state()，前端渲染它即可。
#[tauri::command]
fn get_printer_state() -> printer_core::InitialState {
    printer_core::initial_state()
}

/// 执行安装 / 覆盖 / 删除（内部处理两平台授权：osascript / UAC）。
#[tauri::command]
fn run_printer_install(
    req: InstallRequestDto,
) -> Result<printer_core::printer::InstallOutcome, String> {
    let cfg = printer_core::load_config();
    let targets = printer_core::printer::targets_for_location(&cfg, &req.location);
    if targets.is_empty() {
        return Err(format!("location '{}' not found in config", req.location));
    }
    let plan = printer_core::printer::InstallRequest {
        location: req.location,
        targets,
        overwrite: req.overwrite,
        delete: req.delete,
    };
    let lang = printer_core::i18n::detect();
    printer_core::printer::run_install(&cfg, &lang, &plan)
}

/// 当前语言的界面文案（供入职指引渲染按钮/提示）。
#[tauri::command]
fn get_printer_strings(
    lang: Option<String>,
) -> std::collections::HashMap<String, String> {
    let lang = lang.unwrap_or_else(printer_core::i18n::detect);
    printer_core::i18n::strings(&lang)
}

/// 后台刷新远端配置（不影响当前显示；有更新时发事件给前端）。
#[tauri::command]
fn refresh_printer_config(app: tauri::AppHandle) {
    std::thread::spawn(move || {
        if printer_core::config::refresh_config() {
            let _ = app.emit_to("main", "printer-config-updated", ());
        }
    });
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            get_printer_state,
            run_printer_install,
            get_printer_strings,
            refresh_printer_config
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
