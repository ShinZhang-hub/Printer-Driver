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
/// traditional ("zh-Hant"). Tolerates space-separated multi-tag output and
/// surrounding whitespace.
fn map_system_locale(s: &str) -> Option<String> {
    let lower = s.trim().to_lowercase();
    if lower.starts_with("zh") {
        let is_hant = lower.contains("hant")
            || lower.contains("_tw")
            || lower.contains("_hk")
            || lower.contains("_mo");
        return Some(if is_hant { "zh-Hant" } else { "zh" }.to_string());
    }
    let lang = s.split(['_', '.', '-', ' ']).next().unwrap_or("").to_string();
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
    // Windows: read the cached language captured by the single combined
    // init pass (see win_installer::run_init), so we don't spawn another
    // PowerShell just for locale detection.
    #[cfg(target_os = "windows")]
    {
        let lang = crate::win_installer::cached_lang();
        if let Some(lang) = map_system_locale(&lang) {
            return lang;
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
    // Windows fallback: query the first system UI language tag.
    #[cfg(target_os = "windows")]
    {
        if let Ok(out) = Command::new("powershell")
            .args([
                "-NoProfile",
                "-Command",
                "(Get-WinUserLanguageList)[0].LanguageTag",
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
            ("en", "Detected at %s, click to choose another office"),
            ("ja", "%s を検出、クリックで他オフィスを選択"),
            ("ko", "%s 감지, 클릭하여 다른 오피스 선택"),
            ("zh", "检测到您在 %s，点击可选其他办公室"),
            ("zh-Hant", "偵測到您位於 %s，點擊可選其他辦公室"),
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
        "SET_DEFAULT_LABEL",
        [
            ("en", "Set as default printer"),
            ("ja", "既定のプリンターに設定"),
            ("ko", "기본 프린터로 설정"),
            ("zh", "设为默认打印机"),
            ("zh-Hant", "設為預設印表機"),
        ],
    ),
    (
        "DEFAULT_CHOICE_LABEL",
        [
            ("en", "Default printer:"),
            ("ja", "既定のプリンター："),
            ("ko", "기본 프린터:"),
            ("zh", "选择默认打印机："),
            ("zh-Hant", "選擇預設印表機："),
        ],
    ),
    (
        "EXISTING_PRINTERS",
        [
            ("en", "**%d** printers found, check to remove:"),
            ("ja", "既存プリンター **%d** 台、削除するにはチェック："),
            ("ko", "기존 프린터 **%d** 대, 제거하려면 선택："),
            ("zh", "本机已存在 **%d** 台打印机，勾选可移除："),
            ("zh-Hant", "本機已存在 **%d** 台印表機，勾選可移除："),
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
    (
        "REVIEW_TITLE",
        [
            ("en", "Confirm"),
            ("ja", "確認"),
            ("ko", "확인"),
            ("zh", "确认操作"),
            ("zh-Hant", "確認操作"),
        ],
    ),
    (
        "REVIEW_INSTALL",
        [
            ("en", "Install: "),
            ("ja", "インストール："),
            ("ko", "설치："),
            ("zh", "安装："),
            ("zh-Hant", "安裝："),
        ],
    ),
    (
        "REVIEW_ADD_INSTALL",
        [
            ("en", "Additional install: "),
            ("ja", "追加インストール："),
            ("ko", "추가 설치："),
            ("zh", "追加安装："),
            ("zh-Hant", "追加安裝："),
        ],
    ),
    (
        "REVIEW_CONFLICT",
        [
            ("en", "Conflict: "),
            ("ja", "競合："),
            ("ko", "충돌："),
            ("zh", "冲突处理："),
            ("zh-Hant", "衝突處理："),
        ],
    ),
    (
        "REVIEW_DEFAULT_PRINTER",
        [
            ("en", "Default printer: "),
            ("ja", "既定プリンター："),
            ("ko", "기본 프린터: "),
            ("zh", "默认打印机："),
            ("zh-Hant", "預設印表機："),
        ],
    ),
    (
        "REVIEW_REMOVE",
        [
            ("en", "Remove: "),
            ("ja", "削除："),
            ("ko", "제거："),
            ("zh", "移除："),
            ("zh-Hant", "移除："),
        ],
    ),
    (
        "REVIEW_NONE",
        [
            ("en", "None"),
            ("ja", "なし"),
            ("ko", "없음"),
            ("zh", "无"),
            ("zh-Hant", "無"),
        ],
    ),
    (
        "REVIEW_SKIPPED_ADDED",
        [
            ("en", "Skipped (duplicate): "),
            ("ja", "スキップ（重複）："),
            ("ko", "건너뜀 (중복): "),
            ("zh", "跳过（重复）："),
            ("zh-Hant", "跳過（重複）："),
        ],
    ),
    (
        "REVIEW_FILTERED_REMOVE",
        [
            ("en", "Filtered (to install): "),
            ("ja", "フィルター済（インストール対象）："),
            ("ko", "필터됨 (설치 대상): "),
            ("zh", "过滤（待安装）："),
            ("zh-Hant", "過濾（待安裝）："),
        ],
    ),
    (
        "BTN_ADD_MORE",
        [
            ("en", "＋ Add more"),
            ("ja", "＋ 追加"),
            ("ko", "＋ 추가"),
            ("zh", "＋ 继续添加"),
            ("zh-Hant", "＋ 繼續新增"),
        ],
    ),
    (
        "BTN_ADD",
        [
            ("en", "Add"),
            ("ja", "追加"),
            ("ko", "추가"),
            ("zh", "添加"),
            ("zh-Hant", "新增"),
        ],
    ),
    (
        "BTN_CANCEL",
        [
            ("en", "Cancel"),
            ("ja", "キャンセル"),
            ("ko", "취소"),
            ("zh", "取消"),
            ("zh-Hant", "取消"),
        ],
    ),
    (
        "SELECT_ALL",
        [
            ("en", "Select all"),
            ("ja", "すべて選択"),
            ("ko", "전체 선택"),
            ("zh", "全选"),
            ("zh-Hant", "全選"),
        ],
    ),
    (
        "NO_MORE_TO_ADD",
        [
            ("en", "No more to add"),
            ("ja", "追加なし"),
            ("ko", "추가 없음"),
            ("zh", "无更多可添加"),
            ("zh-Hant", "無更多可新增"),
        ],
    ),
    (
        "TAB_INSTALL",
        [
            ("en", "Install"),
            ("ja", "インストール"),
            ("ko", "설치"),
            ("zh", "安装"),
            ("zh-Hant", "安裝"),
        ],
    ),
    (
        "TAB_REMOVE",
        [
            ("en", "Remove"),
            ("ja", "削除"),
            ("ko", "제거"),
            ("zh", "移除"),
            ("zh-Hant", "移除"),
        ],
    ),
    (
        "TAB_REPAIR",
        [
            ("en", "Repair"),
            ("ja", "修復"),
            ("ko", "복구"),
            ("zh", "修复"),
            ("zh-Hant", "修復"),
        ],
    ),
    (
        "TAB_ANYWHERE",
        [
            ("en", "Print Anywhere"),
            ("ja", "どこでも印刷"),
            ("ko", "어디서나 인쇄"),
            ("zh", "异地打印"),
            ("zh-Hant", "跨點列印"),
        ],
    ),
    (
        "ANYWHERE_BANNER_TITLE",
        [
            ("en", "Remote network detected"),
            ("ja", "異なるネットワークを検出"),
            ("ko", "원격 네트워크 감지됨"),
            ("zh", "检测到异地网络"),
            ("zh-Hant", "偵測到異地網路"),
        ],
    ),
    (
        "ANYWHERE_BANNER_DESC",
        [
            ("en", "Your current IP and default printer are not in the same location. If you need to print, you can apply for remote printing access."),
            ("ja", "現在のIPと既定のプリンターは同じ場所にありません。印刷が必要な場合は、異地印刷の権限を申請できます。"),
            ("ko", "현재 IP와 기본 프린터가 같은 위치에 있지 않습니다. 인쇄가 필요한 경우 원격 인쇄 권한을 신청할 수 있습니다."),
            ("zh", "当前 IP 与您默认打印机不在同一位置，如需打印，可以申请异地打印权限。"),
            ("zh-Hant", "目前 IP 與您的預設印表機不在同一位置，如需列印，可申請異地列印權限。"),
        ],
    ),
    (
        "ANYWHERE_BANNER_DISMISS",
        [
            ("en", "Got it"),
            ("ja", "了解"),
            ("ko", "알겠습니다"),
            ("zh", "知道了"),
            ("zh-Hant", "知道了"),
        ],
    ),
    (
        "ANYWHERE_DESC_TITLE",
        [
            ("en", "Print with ease while traveling"),
            ("ja", "出張先でもかんたん印刷"),
            ("ko", "출장 중에도 간편하게 인쇄"),
            ("zh", "出差也能轻松打印"),
            ("zh-Hant", "出差也能輕鬆列印"),
        ],
    ),
    (
        "ANYWHERE_DESC",
        [
            ("en", "Submit a request. After approval, a temporary password (valid for 24 hours) will be sent via email. Most requests are approved within 30 minutes. Please check your email."),
            ("ja", "申請を送信すると、承認後に一時的な印刷パスワード（24時間有効）がメールで届きます。通常30分以内に承認されます。メールをご確認ください。"),
            ("ko", "신청서를 제출하면 승인 후 임시 인쇄 비밀번호(24시간 유효)가 이메일로 발송됩니다. 대부분 30분 이내에 승인됩니다. 이메일을 확인해 주세요."),
            ("zh", "提交申请，审批通过后将通过邮件发送临时打印密码（有效期24小时）。审批一般在30分钟内完成，请留意邮箱。"),
            ("zh-Hant", "提交申請，審批通過後將透過郵件發送臨時列印密碼（有效期24小時）。審批一般在30分鐘內完成，請留意郵箱。"),
        ],
    ),
    (
        "ANYWHERE_TIP_TITLE",
        [
            ("en", "Tip"),
            ("ja", "ヒント"),
            ("ko", "팁"),
            ("zh", "温馨提示"),
            ("zh-Hant", "溫馨提示"),
        ],
    ),
    (
        "ANYWHERE_TIP_BODY",
        [
            ("en", "If you have your original home office badge, you can also try tapping it directly on the remote printer after approval."),
            ("ja", "元の所属オフィスの社員証をお持ちの場合は、承認後も異地プリンターに直接タッチして試せます。"),
            ("ko", "원래 소속 사무실 배지를 소지한 경우 승인 후에도 원격 프린터에 직접 태그해 볼 수 있습니다."),
            ("zh", "若您携带了原属地办公室工牌，审批通过后亦可直接在异地打印机上尝试刷卡。"),
            ("zh-Hant", "若您攜帶了原屬地辦公室工牌，審批通過後亦可直接在異地印表機上嘗試刷卡。"),
        ],
    ),
    (
        "ANYWHERE_FORM_OFFICE_LABEL",
        [
            ("en", "Target office"),
            ("ja", "対象オフィス"),
            ("ko", "대상 사무실"),
            ("zh", "目标办公室"),
            ("zh-Hant", "目標辦公室"),
        ],
    ),
    (
        "ANYWHERE_FORM_EMAIL_LABEL",
        [
            ("en", "Your email"),
            ("ja", "あなたのメール"),
            ("ko", "이메일"),
            ("zh", "您的邮箱"),
            ("zh-Hant", "您的郵箱"),
        ],
    ),
    (
        "ANYWHERE_FORM_REASON_LABEL",
        [
            ("en", "Reason"),
            ("ja", "申請理由"),
            ("ko", "신청 사유"),
            ("zh", "申请理由"),
            ("zh-Hant", "申請理由"),
        ],
    ),
    (
        "ANYWHERE_FORM_REASON_PLACEHOLDER",
        [
            ("en", "e.g., On business in Tokyo this week, need to print contracts..."),
            ("ja", "例：今週東京出張で契約書を印刷したい…"),
            ("ko", "예: 이번 주 도쿄 출장 중 계약서 인쇄 필요…"),
            ("zh", "例如：本周在东京出差，需打印合同…"),
            ("zh-Hant", "例如：本週在東京出差，需列印合約…"),
        ],
    ),
    (
        "ANYWHERE_SUBMIT",
        [
            ("en", "Submit request"),
            ("ja", "申請を送信"),
            ("ko", "신청 제출"),
            ("zh", "提交申请"),
            ("zh-Hant", "提交申請"),
        ],
    ),
    (
        "ANYWHERE_CANCEL",
        [
            ("en", "Cancel"),
            ("ja", "キャンセル"),
            ("ko", "취소"),
            ("zh", "取消"),
            ("zh-Hant", "取消"),
        ],
    ),
    (
        "ANYWHERE_FOOTER",
        [
            ("en", "Please check your email after submission"),
            ("ja", "送信後はメールをご確認ください"),
            ("ko", "제출 후 이메일을 확인해 주세요"),
            ("zh", "提交后请留意邮件通知"),
            ("zh-Hant", "提交後請留意郵件通知"),
        ],
    ),
    (
        "ANYWHERE_STATUS",
        [
            ("en", "Request submitted"),
            ("ja", "申請を送信しました"),
            ("ko", "신청이 제출되었습니다"),
            ("zh", "已提交申请"),
            ("zh-Hant", "已提交申請"),
        ],
    ),
    (
        "ANYWHERE_STATUS_DETAIL",
        [
            ("en", "Under review — you will receive a temporary password via email within 30 minutes."),
            ("ja", "審査中 — 30分以内に一時パスワードをメールでお届けします。"),
            ("ko", "심사 중 — 30분 이내에 임시 비밀번호를 이메일로 받게 됩니다."),
            ("zh", "审批中，预计30分钟内通过邮件收到临时密码。"),
            ("zh-Hant", "審批中，預計30分鐘內透過郵件收到臨時密碼。"),
        ],
    ),
    (
        "ANYWHERE_REASON_REQUIRED",
        [
            ("en", "Please enter a reason"),
            ("ja", "申請理由を入力してください"),
            ("ko", "신청 사유를 입력해 주세요"),
            ("zh", "请填写申请理由"),
            ("zh-Hant", "請填寫申請理由"),
        ],
    ),
    (
        "REQUIRED",
        [
            ("en", "Required"),
            ("ja", "必須"),
            ("ko", "필수"),
            ("zh", "必选"),
            ("zh-Hant", "必選"),
        ],
    ),
    (
        "MAX_TWO",
        [
            ("en", "Select at most 2"),
            ("ja", "最大2台まで選択"),
            ("ko", "최대 2대까지 선택"),
            ("zh", "最多选择2台"),
            ("zh-Hant", "最多選擇2台"),
        ],
    ),
    (
        "ANYWHERE_REQ",
        [
            ("en", "Request No."),
            ("ja", "申請番号"),
            ("ko", "신청 번호"),
            ("zh", "申请单号"),
            ("zh-Hant", "申請單號"),
        ],
    ),
    (
        "ANYWHERE_DETAIL_PRINTERS",
        [
            ("en", "Printer"),
            ("ja", "プリンター"),
            ("ko", "프린터"),
            ("zh", "打印机"),
            ("zh-Hant", "印表機"),
        ],
    ),
    (
        "ANYWHERE_CONFIRM",
        [
            ("en", "Confirm"),
            ("ja", "確認"),
            ("ko", "확인"),
            ("zh", "确认"),
            ("zh-Hant", "確認"),
        ],
    ),
    (
        "ANYWHERE_SUCCESS_QUOTA",
        [
            ("en", "Today requests: %d/2"),
            ("ja", "本日の申請回数：%d/2"),
            ("ko", "오늘 신청 횟수: %d/2"),
            ("zh", "今日申请次数：%d/2"),
            ("zh-Hant", "今日申請次數：%d/2"),
        ],
    ),
    (
        "ANYWHERE_SUCCESS_HINT",
        [
            ("en", "The request window will reopen tomorrow."),
            ("ja", "申請窓口は明日再開します。"),
            ("ko", "신청 창구는 내일 다시 열립니다."),
            ("zh", "明天将重新开放申请"),
            ("zh-Hant", "明天將重新開放申請"),
        ],
    ),
    (
        "ANYWHERE_ADVANCE_LABEL",
        [
            ("en", "Advance request (simulate no detection)"),
            ("ja", "事前申請モード（未検出をシミュレート）"),
            ("ko", "사전 신청 모드 (미감지 시뮬레이션)"),
            ("zh", "提前申请模式（模拟未检测到异地）"),
            ("zh-Hant", "提前申請模式（模擬未偵測到異地）"),
        ],
    ),
    (
        "ANYWHERE_ADVANCE_HINT",
        [
            ("en", " — hide banner, still applicable"),
            ("ja", " — バナーを非表示でも申請可"),
            ("ko", " — 배너 숨김, 신청 가능"),
            ("zh", " — 隐藏提示横幅，仍可申请"),
            ("zh-Hant", " — 隱藏提示橫幅，仍可申請"),
        ],
    ),
    (
        "REPAIR_TITLE",
        [
            ("en", "Printer Repair"),
            ("ja", "プリンタ修復"),
            ("ko", "프린터 복구"),
            ("zh", "打印机修复"),
            ("zh-Hant", "印表機修復"),
        ],
    ),
    (
        "REPAIR_HINT",
        [
            ("en", "Diagnose and fix printing issues"),
            ("ja", "印刷問題を診断・修復"),
            ("ko", "인쇄 문제 진단 및 수정"),
            ("zh", "诊断并修复打印问题"),
            ("zh-Hant", "診斷並修復列印問題"),
        ],
    ),
    (
        "REPAIR_ITEM1",
        [
            ("en", "Driver check"),
            ("ja", "ドライバーチェック"),
            ("ko", "드라이버 확인"),
            ("zh", "驱动状态检查"),
            ("zh-Hant", "驅動狀態檢查"),
        ],
    ),
    (
        "REPAIR_ITEM1_DETAIL",
        [
            ("en", "Check if FUJIFILM driver is fully installed"),
            ("ja", "FUJIFILMドライバーのインストール状態を確認"),
            ("ko", "FUJIFILM 드라이버 설치 상태 확인"),
            ("zh", "检查 FUJIFILM 驱动是否完整安装"),
            ("zh-Hant", "檢查 FUJIFILM 驅動是否完整安裝"),
        ],
    ),
    (
        "REPAIR_ITEM2",
        [
            ("en", "Filter permissions"),
            ("ja", "フィルター権限"),
            ("ko", "필터 권한"),
            ("zh", "过滤器权限"),
            ("zh-Hant", "過濾器權限"),
        ],
    ),
    (
        "REPAIR_ITEM2_DETAIL",
        [
            ("en", "Check FFACMMCFilter owner and permissions"),
            ("ja", "FFACMMCFilterのオーナーと権限を確認"),
            ("ko", "FFACMMCFilter 소유자 및 권한 확인"),
            ("zh", "检查 FFACMMCFilter owner 和权限"),
            ("zh-Hant", "檢查 FFACMMCFilter 所有者和權限"),
        ],
    ),
    (
        "REPAIR_ITEM3",
        [
            ("en", "CUPS service"),
            ("ja", "CUPSサービス"),
            ("ko", "CUPS 서비스"),
            ("zh", "CUPS 服务"),
            ("zh-Hant", "CUPS 服務"),
        ],
    ),
    (
        "REPAIR_ITEM3_DETAIL",
        [
            ("en", "Check print service status and cache"),
            ("ja", "印刷サービスの状態とキャッシュを確認"),
            ("ko", "인쇄 서비스 상태 및 캐시 확인"),
            ("zh", "检查打印服务运行状态和缓存"),
            ("zh-Hant", "檢查列印服務運行狀態和快取"),
        ],
    ),
    (
        "REPAIR_STATUS",
        [
            ("en", "Pending"),
            ("ja", "診断待ち"),
            ("ko", "진단 대기"),
            ("zh", "待诊断"),
            ("zh-Hant", "待診斷"),
        ],
    ),
    (
        "REPAIR_FOOTER",
        [
            ("en", "Click below to start diagnosis"),
            ("ja", "下記をクリックして診断開始"),
            ("ko", "아래를 클릭하여 진단 시작"),
            ("zh", "点击下方开始自动诊断"),
            ("zh-Hant", "點擊下方開始自動診斷"),
        ],
    ),
    (
        "REPAIR_BTN",
        [
            ("en", "Start diagnosis"),
            ("ja", "診断開始"),
            ("ko", "진단 시작"),
            ("zh", "开始诊断"),
            ("zh-Hant", "開始診斷"),
        ],
    ),
    (
        "REPAIR_DEV",
        [
            ("en", "Diagnosis feature in development..."),
            ("ja", "診断機能は開発中です..."),
            ("ko", "진단 기능 개발 중..."),
            ("zh", "诊断功能开发中..."),
            ("zh-Hant", "診斷功能開發中..."),
        ],
    ),
    // --- app-new 新增：确保所有非配置文案随语言切换翻译 ---
    (
        "OFFICE",
        [
            ("en", "Office"),
            ("ja", "オフィス"),
            ("ko", "오피스"),
            ("zh", "办公室"),
            ("zh-Hant", "辦公室"),
        ],
    ),
    (
        "AUTO_DETECT",
        [
            ("en", "Auto-detected"),
            ("ja", "自動検出"),
            ("ko", "자동 감지"),
            ("zh", "自动检测"),
            ("zh-Hant", "自動偵測"),
        ],
    ),
    (
        "MANUAL_SELECT",
        [
            ("en", "Manual selection"),
            ("ja", "手動選択"),
            ("ko", "수동 선택"),
            ("zh", "手动选择"),
            ("zh-Hant", "手動選擇"),
        ],
    ),
    (
        "CHANGE",
        [
            ("en", "Change"),
            ("ja", "変更"),
            ("ko", "변경"),
            ("zh", "更换"),
            ("zh-Hant", "更換"),
        ],
    ),
    (
        "AUTO_DETECT_MENU",
        [
            ("en", "Auto-detect (Recommended)"),
            ("ja", "自動検出（推奨）"),
            ("ko", "자동 감지 (권장)"),
            ("zh", "自动检测（推荐）"),
            ("zh-Hant", "自動偵測（推薦）"),
        ],
    ),
    (
        "LOCAL_IP",
        [
            ("en", "Local IP: "),
            ("ja", "ローカルIP："),
            ("ko", "로컬 IP: "),
            ("zh", "本机 IP："),
            ("zh-Hant", "本機 IP："),
        ],
    ),
    (
        "CAPTION_INSTALL",
        [
            ("en", "Available printers"),
            ("ja", "利用可能なプリンター"),
            ("ko", "사용 가능한 프린터"),
            ("zh", "可用打印机"),
            ("zh-Hant", "可用印表機"),
        ],
    ),
    (
        "CAPTION_INSTALL_HINT",
        [
            ("en", "Check to install; set as default on the right"),
            ("ja", "チェックしてインストール；右側で既定に設定"),
            ("ko", "선택하여 설치; 오른쪽에서 기본값으로 설정"),
            ("zh", "勾选安装；右侧可设为默认"),
            ("zh-Hant", "勾選安裝；右側可設為預設"),
        ],
    ),
    (
        "CURRENT_DEFAULT",
        [
            ("en", "Current default printer: "),
            ("ja", "現在の既定プリンター："),
            ("ko", "현재 기본 프린터: "),
            ("zh", "当前默认打印机："),
            ("zh-Hant", "目前預設印表機："),
        ],
    ),
    (
        "NONE",
        [
            ("en", "None"),
            ("ja", "未設定"),
            ("ko", "없음"),
            ("zh", "未设置"),
            ("zh-Hant", "未設定"),
        ],
    ),
    (
        "INSTALLED_TAG",
        [
            ("en", "Installed"),
            ("ja", "インストール済み"),
            ("ko", "설치됨"),
            ("zh", "已安装"),
            ("zh-Hant", "已安裝"),
        ],
    ),
    (
        "AVAILABLE_TAG",
        [
            ("en", "Available"),
            ("ja", "利用可能"),
            ("ko", "사용 가능"),
            ("zh", "可安装"),
            ("zh-Hant", "可安裝"),
        ],
    ),
    (
        "SET_DEFAULT",
        [
            ("en", "Set as default"),
            ("ja", "既定に設定"),
            ("ko", "기본값으로 설정"),
            ("zh", "设为默认"),
            ("zh-Hant", "設為預設"),
        ],
    ),
    (
        "CURRENT_DEFAULT_TAG",
        [
            ("en", "Current default"),
            ("ja", "現在の既定"),
            ("ko", "현재 기본"),
            ("zh", "当前默认"),
            ("zh-Hant", "目前預設"),
        ],
    ),
    (
        "SELECTION",
        [
            ("en", "Selected"),
            ("ja", "選択済み"),
            ("ko", "선택됨"),
            ("zh", "已选择"),
            ("zh-Hant", "已選擇"),
        ],
    ),
    (
        "UNIT",
        [
            ("en", ""),
            ("ja", "台"),
            ("ko", "대"),
            ("zh", "台"),
            ("zh-Hant", "台"),
        ],
    ),
    (
        "CANCEL",
        [
            ("en", "Cancel"),
            ("ja", "キャンセル"),
            ("ko", "취소"),
            ("zh", "取消"),
            ("zh-Hant", "取消"),
        ],
    ),
    (
        "INSTALL_BTN",
        [
            ("en", "Install"),
            ("ja", "インストール"),
            ("ko", "설치"),
            ("zh", "安装"),
            ("zh-Hant", "安裝"),
        ],
    ),
    (
        "INSTALLED_PRINTERS",
        [
            ("en", "Installed printers"),
            ("ja", "インストール済みプリンター"),
            ("ko", "설치된 프린터"),
            ("zh", "已安装的打印机"),
            ("zh-Hant", "已安裝的印表機"),
        ],
    ),
    (
        "CANCEL_SELECT_ALL",
        [
            ("en", "Deselect all"),
            ("ja", "選択を解除"),
            ("ko", "전체 선택 해제"),
            ("zh", "取消全选"),
            ("zh-Hant", "取消全選"),
        ],
    ),
    (
        "REMOVE_NOTE",
        [
            ("en", "If the current default is removed, the system will automatically select another available printer."),
            ("ja", "現在の既定を削除すると、システムが自動的に別のプリンターを選択します。"),
            ("ko", "현재 기본을 제거하면 시스템이 자동으로 다른 프린터를 선택합니다."),
            ("zh", "移除当前默认设备后，系统将自动选择其他可用打印机。"),
            ("zh-Hant", "移除目前預設裝置後，系統將自動選擇其他可用印表機。"),
        ],
    ),
    (
        "REMOVE_BTN",
        [
            ("en", "Remove"),
            ("ja", "削除"),
            ("ko", "제거"),
            ("zh", "移除"),
            ("zh-Hant", "移除"),
        ],
    ),
    (
        "SERVER_OK",
        [
            ("en", "Remote connection normal"),
            ("ja", "リモート接続正常"),
            ("ko", "원격 연결 정상"),
            ("zh", "远端连接正常"),
            ("zh-Hant", "遠端連線正常"),
        ],
    ),
    (
        "SERVER_ERR",
        [
            ("en", "Remote connection failed"),
            ("ja", "リモート接続異常"),
            ("ko", "원격 연결 실패"),
            ("zh", "远端连接异常"),
            ("zh-Hant", "遠端連線異常"),
        ],
    ),
    (
        "TOAST_INSTALL",
        [
            ("en", "Installed %d printers"),
            ("ja", "%d 台のプリンターをインストールしました"),
            ("ko", "%d대의 프린터를 설치했습니다"),
            ("zh", "已安装 %d 台打印机"),
            ("zh-Hant", "已安裝 %d 台印表機"),
        ],
    ),
    (
        "TOAST_REMOVE",
        [
            ("en", "Removed %d printers"),
            ("ja", "%d 台のプリンターを削除しました"),
            ("ko", "%d대의 프린터를 제거했습니다"),
            ("zh", "已移除 %d 台打印机"),
            ("zh-Hant", "已移除 %d 台印表機"),
        ],
    ),
    (
        "TOAST_CANCEL",
        [
            ("en", "Cancelled"),
            ("ja", "キャンセルしました"),
            ("ko", "취소되었습니다"),
            ("zh", "已取消本次操作"),
            ("zh-Hant", "已取消本次操作"),
        ],
    ),
    (
        "TOAST_SWITCH",
        [
            ("en", "Switched to %s"),
            ("ja", "%s に切り替えました"),
            ("ko", "%s(으)로 전환했습니다"),
            ("zh", "已切换到 %s"),
            ("zh-Hant", "已切換到 %s"),
        ],
    ),
    (
        "TOAST_AUTO",
        [
            ("en", "Auto-detected as %s"),
            ("ja", "自動的に %s として認識されました"),
            ("ko", "자동으로 %s(으)로 인식되었습니다"),
            ("zh", "已自动识别为 %s"),
            ("zh-Hant", "已自動識別為 %s"),
        ],
    ),
];
