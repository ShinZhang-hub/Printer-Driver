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

pub fn t(lang: &str, key: &str, args: &[&str]) -> String {
    let s = strings(lang);
    let fmt = s.get(key).cloned().unwrap_or_default();
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
    // Windows: query the first system UI language tag.
    #[cfg(target_os = "windows")]
    {
        if let Ok(out) = Command::new("powershell")
            .args([
                "-NoProfile",
                "-Command",
                "(Get-WinUserLanguageList | Select-Object -First 1).LanguageTag",
            ])
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
            ("en", "Detected at %s — uncheck to choose another office"),
            ("ja", "%s を検出 — チェックを外すと他オフィスを選択"),
            ("ko", "%s 감지 — 해제하면 다른 오피스 선택"),
            ("zh", "检测到您在 %s — 取消勾选可选其他办公室"),
            ("zh-Hant", "偵測到您位於 %s — 取消勾選可選其他辦公室"),
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
            ("en", "Default printer already exists — **overwrite** or **skip** ?"),
            ("ja", "選択オフィスの既定プリンターが既存。**上書き** か **スキップ** ？"),
            ("ko", "선택 사무실의 기본 프린터가 이미 있음. **덮어쓰기** / **건너뛰기** ？"),
            ("zh", "所选办公室的默认打印机已存在，**覆盖** 或 **跳过** ？"),
            ("zh-Hant", "所選辦公室的預設印表機已存在，**覆蓋** 或 **跳過** ？"),
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
            ("en", "**%d** printers found — check to remove (selected office default kept)"),
            ("ja", "既存プリンター **%d** 台 — 削除は選択（選択オフィス既定は不可）："),
            ("ko", "기존 프린터 **%d** 대 — 제거는 선택（선택 사무실 기본은 불가）："),
            ("zh", "本机已存在 **%d** 台打印机，勾选可移除（所选办公室默认打印机除外）："),
            ("zh-Hant", "本機已存在 **%d** 台印表機，勾選可移除（所選辦公室預設印表機除外）："),
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
    (
        "INSTALL_FAILED_MSG",
        [
            ("en", "❌ %s failed to install after 2 attempts"),
            ("ja", "❌ %s が2回試行後もインストールに失敗しました"),
            ("ko", "❌ %s 2회 시도 후에도 설치에 실패했습니다"),
            ("zh", "❌ %s 两次尝试后仍安装失败"),
            ("zh-Hant", "❌ %s 兩次嘗試後仍安裝失敗"),
        ],
    ),
    (
        "REMOVE_FAILED_MSG",
        [
            ("en", "❌ %s failed to remove after 2 attempts"),
            ("ja", "❌ %s が2回試行後も削除に失敗しました"),
            ("ko", "❌ %s 2회 시도 후에도 제거에 실패했습니다"),
            ("zh", "❌ %s 两次尝试后仍移除失败"),
            ("zh-Hant", "❌ %s 兩次嘗試後仍移除失敗"),
        ],
    ),
    (
        "FAIL_CAUSE_LPADMIN",
        [
            ("en", "could not create the print queue (lpadmin error)"),
            ("ja", "キューの作成に失敗しました（lpadmin エラー）"),
            ("ko", "인쇄 큐를 만들지 못했습니다 (lpadmin 오류)"),
            ("zh", "无法创建打印机队列（lpadmin 返回错误）"),
            ("zh-Hant", "無法建立印表機佇列（lpadmin 回傳錯誤）"),
        ],
    ),
    (
        "FAIL_CAUSE_VERIFY",
        [
            ("en", "queue registration could not be verified"),
            ("ja", "キューの登録を確認できませんでした"),
            ("ko", "큐 등록을 확인할 수 없습니다"),
            ("zh", "队列注册校验未通过（查无此打印机）"),
            ("zh-Hant", "佇列註冊驗證未通過（查無此印表機）"),
        ],
    ),
    (
        "FAIL_CAUSE_ENABLE",
        [
            ("en", "could not enable the print queue"),
            ("ja", "キューを有効化できませんでした"),
            ("ko", "인쇄 큐를 활성화할 수 없습니다"),
            ("zh", "无法启用打印机队列"),
            ("zh-Hant", "無法啟用印表機佇列"),
        ],
    ),
    (
        "FAIL_CAUSE_ACCEPT",
        [
            ("en", "could not set the queue to accept new jobs"),
            ("ja", "新規ジョブを受け付ける設定にできませんでした"),
            ("ko", "새 작업을 받도록 큐를 설정할 수 없습니다"),
            ("zh", "无法设置为接受新作业"),
            ("zh-Hant", "無法設定為接受新作業"),
        ],
    ),
    (
        "FAIL_CAUSE_DEFAULT",
        [
            ("en", "could not set it as the default printer"),
            ("ja", "既定プリンターに設定できませんでした"),
            ("ko", "기본 프린터로 설정할 수 없습니다"),
            ("zh", "无法设为默认打印机"),
            ("zh-Hant", "無法設為預設印表機"),
        ],
    ),
    (
        "FAIL_CAUSE_DELETE",
        [
            ("en", "could not remove the printer"),
            ("ja", "プリンターを削除できませんでした"),
            ("ko", "프린터를 제거할 수 없습니다"),
            ("zh", "两轮尝试后仍无法删除打印机"),
            ("zh-Hant", "兩次嘗試後仍無法刪除印表機"),
        ],
    ),
    (
        "FAIL_CAUSE_UNKNOWN",
        [
            ("en", "an unknown error occurred"),
            ("ja", "不明なエラーが発生しました"),
            ("ko", "알 수 없는 오류가 발생했습니다"),
            ("zh", "发生未知错误"),
            ("zh-Hant", "發生未知錯誤"),
        ],
    ),
];
