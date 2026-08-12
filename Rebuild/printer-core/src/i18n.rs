use std::collections::HashMap;
use std::process::Command;

pub const LANGS: [&str; 5] = ["en", "ja", "ko", "zh", "zh-Hant"];

/// All UI strings, keyed by language. Mirrors the existing copy so both
/// apps (standalone + onboarding) share the same wording.
pub fn strings(lang: &str) -> HashMap<String, String> {
    let lang = if LANGS.contains(&lang) { lang } else { "en" };
    let mut m = HashMap::new();
    for (key, table) in STRINGS {
        let v = table
            .iter()
            .find(|(l, _)| *l == lang)
            .or_else(|| table.iter().find(|(l, _)| *l == "en"))
            .unwrap()
            .1;
        m.insert(key.to_string(), v.to_string());
    }
    m
}

pub fn t<'a>(lang: &str, key: &str, args: &[&str]) -> String {
    let s = strings(lang);
    let fmt = s.get(key).map(|s| s.clone()).unwrap_or_default();
    let mut out = fmt;
    for (i, a) in args.iter().enumerate() {
        // Replace %s sequentially (matches the current behavior of the script).
        out = out.replacen("%s", a, 1);
        let named = format!("%{{{}}}", i + 1);
        out = out.replace(&named, a);
    }
    out
}

/// Map a system locale identifier (e.g. "zh-Hant_TW", "zh_CN", "ja_JP")
/// to one of LANGS. Chinese is split into simplified ("zh") and
/// traditional ("zh-Hant").
fn map_system_locale(s: &str) -> Option<String> {
    let lower = s.to_lowercase();
    if lower.starts_with("zh") {
        let is_hant = lower.contains("hant")
            || lower.contains("_tw")
            || lower.contains("_hk")
            || lower.contains("_mo");
        return Some(if is_hant { "zh-Hant" } else { "zh" }.to_string());
    }
    let lang = s.split(['_', '.', '-']).next().unwrap_or("").to_string();
    if LANGS.contains(&lang.as_str()) {
        Some(lang)
    } else {
        None
    }
}

pub fn detect() -> String {
    if let Ok(l) = std::env::var("PRINTER_INSTALLER_LANG") {
        if LANGS.contains(&l.as_str()) {
            return l;
        }
    }
    // macOS: query the system locale.
    #[cfg(target_os = "macos")]
    {
        if let Ok(out) = Command::new("osascript")
            .args(["-e", "user locale of (system info)"])
            .output()
        {
            let s = String::from_utf8_lossy(&out.stdout).trim().to_string();
            if let Some(lang) = map_system_locale(&s) {
                return lang;
            }
        }
    }
    // Fallback to LANG env.
    if let Ok(l) = std::env::var("LANG") {
        if let Some(lang) = map_system_locale(&l) {
            return lang;
        }
    }
    "en".to_string()
}

type Table = [(&'static str, &'static str); 5];

const STRINGS: &[(&str, Table)] = &[
    (
        "TITLE",
        [
            ("en", "Printer Driver Installer"),
            ("ja", "プリンタードライバーインストーラー"),
            ("ko", "프린터 드라이버 설치"),
            ("zh", "打印机驱动安装"),
            ("zh-Hant", "印表機驅動程式安裝程式"),
        ],
    ),
    (
        "DETECTING",
        [
            ("en", "Detecting..."),
            ("ja", "検出中..."),
            ("ko", "감지 중..."),
            ("zh", "检测中..."),
            ("zh-Hant", "偵測中..."),
        ],
    ),
    (
        "INSTALLING",
        [
            ("en", "Installing/removing printers, please wait..."),
            ("ja", "プリンターをインストール／削除中です。しばらくお待ちください..."),
            ("ko", "프린터 설치/제거 중입니다. 잠시만 기다려 주세요..."),
            ("zh", "正在安装/删除打印机，请稍后..."),
            ("zh-Hant", "正在安裝/移除印表機，請稍候..."),
        ],
    ),
    (
        "CONFIRM_FMT",
        [
            ("en", "Detected at %s, uncheck to pick another location"),
            ("ja", "%s を検出、チェックを外すと別の場所を選択できます"),
            ("ko", "%s 감지됨, 체크 해제 시 다른 위치 선택 가능"),
            ("zh", "检测到您在%s，取消勾选也可选择其他位置"),
            ("zh-Hant", "偵測到您位於%s，取消勾選也可選擇其他位置"),
        ],
    ),
    (
        "PICKER_PROMPT",
        [
            ("en", "Select the correct location:"),
            ("ja", "正しい場所を選択してください："),
            ("ko", "올바른 위치를 선택하세요："),
            ("zh", "请选择正确的位置："),
            ("zh-Hant", "請選擇正確的位置："),
        ],
    ),
    (
        "CONFLICT_LABEL",
        [
            ("en", "A printer exists at this IP, choose:"),
            ("ja", "このIPにプリンターが既存、選択："),
            ("ko", "이 IP에 프린터 존재, 선택："),
            ("zh", "同IP打印机已存在，请选择："),
            ("zh-Hant", "此 IP 已有印表機，請選擇："),
        ],
    ),
    (
        "SKIP_BTN",
        [
            ("en", "Skip"),
            ("ja", "スキップ"),
            ("ko", "건너뛰기"),
            ("zh", "跳过"),
            ("zh-Hant", "跳過"),
        ],
    ),
    (
        "OVERWRITE_LABEL",
        [
            ("en", "Overwrite"),
            ("ja", "上書きインストール"),
            ("ko", "덮어쓰기"),
            ("zh", "覆盖安装"),
            ("zh-Hant", "覆蓋安裝"),
        ],
    ),
    (
        "EXISTING_PRINTERS",
        [
            ("en", "Existing printers (%d), check to remove:"),
            ("ja", "既存プリンター (%d)、削除する場合はチェック："),
            ("ko", "기존 프린터 (%d), 제거하려면 선택："),
            ("zh", "现有打印机 (%d)，如需移除请勾选："),
            ("zh-Hant", "現有印表機 (%d)，如需移除請勾選："),
        ],
    ),
    (
        "OK_LABEL",
        [
            ("en", "OK"),
            ("ja", "OK"),
            ("ko", "확인"),
            ("zh", "好"),
            ("zh-Hant", "好"),
        ],
    ),
    (
        "CANCEL_LABEL",
        [
            ("en", "Cancel"),
            ("ja", "キャンセル"),
            ("ko", "취소"),
            ("zh", "取消"),
            ("zh-Hant", "取消"),
        ],
    ),
    (
        "INSTALLED_LABEL",
        [
            ("en", "✅ %s installed successfully"),
            ("ja", "✅ %s をインストールしました"),
            ("ko", "✅ %s 설치 완료"),
            ("zh", "✅ %s 已成功安装"),
            ("zh-Hant", "✅ %s 已成功安裝"),
        ],
    ),
    (
        "SKIP_INSTALL_MSG",
        [
            ("en", "ℹ️ %s already exists, no action needed"),
            ("ja", "ℹ️ %s は既に存在します。操作不要"),
            ("ko", "ℹ️ %s 이(가) 이미 존재합니다. 작업 불필요"),
            ("zh", "ℹ️ %s 已存在，无需操作"),
            ("zh-Hant", "ℹ️ %s 已存在，無需操作"),
        ],
    ),
    (
        "OVERWRITTEN_MSG",
        [
            ("en", "✅ %s updated successfully"),
            ("ja", "✅ %s を上書きインストールしました"),
            ("ko", "✅ %s 덮어쓰기 설치 완료"),
            ("zh", "✅ %s 已成功覆盖安装"),
            ("zh-Hant", "✅ %s 已成功覆蓋安裝"),
        ],
    ),
    (
        "REMOVED_MSG",
        [
            ("en", "🗑️ %s removed successfully"),
            ("ja", "🗑️ %s を削除しました"),
            ("ko", "🗑️ %s 제거 완료"),
            ("zh", "🗑️ %s 已成功移除"),
            ("zh-Hant", "🗑️ %s 已成功移除"),
        ],
    ),
    (
        "FAIL_PREFIX",
        [
            ("en", "❌ Installation failed:"),
            ("ja", "❌ インストール失敗："),
            ("ko", "❌ 설치 실패："),
            ("zh", "❌ 安装失败："),
            ("zh-Hant", "❌ 安裝失敗："),
        ],
    ),
    (
        "NO_LOCATION",
        [
            ("en", "No location detected"),
            ("ja", "場所が検出されませんでした"),
            ("ko", "위치를 감지할 수 없음"),
            ("zh", "未检测到位置"),
            ("zh-Hant", "未偵測到位置"),
        ],
    ),
    (
        "ADMIN_PROMPT",
        [
            ("en", "Printer driver installation requires admin privileges"),
            ("ja", "プリンタードライバーのインストールには管理者権限が必要です"),
            ("ko", "프린터 드라이버 설치를 위해 관리자 권한이 필요합니다"),
            ("zh", "打印机驱动安装需要管理员权限"),
            ("zh-Hant", "印表機驅動程式安裝需要管理員權限"),
        ],
    ),
];
