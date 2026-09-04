// mock.js — 模拟 printer-core 的 InitialState 和 i18n，无需 Rust/Tauri
export const MOCK_STRINGS = {
  en: {
    TITLE: "Printer Driver Installer",
    DETECTING: "Detecting...",
    INSTALLING: "Installing/removing printers, please wait...",
    CONFIRM_FMT: "Detected at %s, click to choose another office",
    PICKER_PROMPT: "Select the correct location:",
    CONFLICT_LABEL: "Default printer already exists — **overwrite** or **skip** ?",
    SKIP_BTN: "Skip",
    OVERWRITE_LABEL: "Overwrite",
    SET_DEFAULT_LABEL: "Set as default printer",
    DEFAULT_CHOICE_LABEL: "Default printer:",
    EXISTING_PRINTERS: "**%d** printers found, check to remove:",
    OK_LABEL: "OK",
    CANCEL_LABEL: "Cancel",
    INSTALLED_LABEL: "✅ %s installed successfully",
    SKIP_INSTALL_MSG: "ℹ️ %s already exists, no action needed",
    OVERWRITTEN_MSG: "✅ %s updated successfully",
    REMOVED_MSG: "🗑️ %s removed successfully",
    FAIL_PREFIX: "❌ Installation failed:",
    NO_LOCATION: "No location detected",
    ADMIN_PROMPT: "Printer driver installation requires admin privileges",
    INSTALL_FAILED_MSG: "❌ %s failed to install after 2 attempts",
    REMOVE_FAILED_MSG: "❌ %s failed to remove after 2 attempts",
    FAIL_CAUSE_LPADMIN: "could not create the print queue (lpadmin error)",
    FAIL_CAUSE_VERIFY: "queue registration could not be verified",
    FAIL_CAUSE_ENABLE: "could not enable the print queue",
    FAIL_CAUSE_ACCEPT: "could not set the queue to accept new jobs",
    FAIL_CAUSE_DEFAULT: "could not set it as the default printer",
    FAIL_CAUSE_DELETE: "could not remove the printer",
    FAIL_CAUSE_UNKNOWN: "an unknown error occurred",
    REVIEW_TITLE: "Confirm",
    REVIEW_INSTALL: "Install:",
    REVIEW_ADD_INSTALL: "Additional install:",
    REVIEW_CONFLICT: "Conflict:",
    REVIEW_DEFAULT_PRINTER: "Default printer:",
    REVIEW_REMOVE: "Remove:",
    REVIEW_NONE: "None",
    REVIEW_SKIPPED_ADDED: "Skipped (duplicate):",
    REVIEW_FILTERED_REMOVE: "Filtered (to install):",
    BTN_ADD_MORE: "＋ Add more",
    BTN_ADD: "Add",
    BTN_CANCEL: "Cancel",
    SELECT_ALL: "Select all",
    NO_MORE_TO_ADD: "No more to add",
    TAB_INSTALL: "Install",
    TAB_REMOVE: "Remove",
  },
  zh: {
    TITLE: "打印机驱动安装",
    DETECTING: "检测中...",
    INSTALLING: "正在安装/删除打印机，请稍后...",
    CONFIRM_FMT: "检测到您在 %s，点击可选其他办公室",
    PICKER_PROMPT: "请选择正确的位置：",
    CONFLICT_LABEL: "所选办公室的默认打印机已存在，**覆盖** 或 **跳过** ？",
    SKIP_BTN: "跳过",
    OVERWRITE_LABEL: "覆盖安装",
    SET_DEFAULT_LABEL: "设为默认打印机",
    DEFAULT_CHOICE_LABEL: "选择默认打印机：",
    EXISTING_PRINTERS: "本机已存在 **%d** 台打印机，勾选可移除：",
    OK_LABEL: "好",
    CANCEL_LABEL: "取消",
    INSTALLED_LABEL: "✅ %s 已成功安装",
    SKIP_INSTALL_MSG: "ℹ️ %s 已存在，无需操作",
    OVERWRITTEN_MSG: "✅ %s 已成功覆盖安装",
    REMOVED_MSG: "🗑️ %s 已成功移除",
    FAIL_PREFIX: "❌ 安装失败：",
    NO_LOCATION: "未检测到位置",
    ADMIN_PROMPT: "打印机驱动安装需要管理员权限",
    INSTALL_FAILED_MSG: "❌ %s 两次尝试后仍安装失败",
    REMOVE_FAILED_MSG: "❌ %s 两次尝试后仍移除失败",
    FAIL_CAUSE_LPADMIN: "无法创建打印机队列（lpadmin 返回错误）",
    FAIL_CAUSE_VERIFY: "队列注册校验未通过（查无此打印机）",
    FAIL_CAUSE_ENABLE: "无法启用打印机队列",
    FAIL_CAUSE_ACCEPT: "无法设置为接受新作业",
    FAIL_CAUSE_DEFAULT: "无法设为默认打印机",
    FAIL_CAUSE_DELETE: "两轮尝试后仍无法删除打印机",
    FAIL_CAUSE_UNKNOWN: "发生未知错误",
    REVIEW_TITLE: "确认操作",
    REVIEW_INSTALL: "安装：",
    REVIEW_ADD_INSTALL: "追加安装：",
    REVIEW_CONFLICT: "冲突处理：",
    REVIEW_DEFAULT_PRINTER: "默认打印机：",
    REVIEW_REMOVE: "移除：",
    REVIEW_NONE: "无",
    REVIEW_SKIPPED_ADDED: "跳过（重复）：",
    REVIEW_FILTERED_REMOVE: "过滤（待安装）：",
    BTN_ADD_MORE: "＋ 继续添加",
    BTN_ADD: "添加",
    BTN_CANCEL: "取消",
    SELECT_ALL: "全选",
    NO_MORE_TO_ADD: "无更多可添加",
    TAB_INSTALL: "安装",
    TAB_REMOVE: "移除",
  },
  ja: {
    TITLE: "プリンタードライバーインストーラー",
    DETECTING: "検出中...",
    INSTALLING: "プリンターをインストール／削除中です。しばらくお待ちください...",
    CONFIRM_FMT: "%s を検出、クリックで他オフィスを選択",
    PICKER_PROMPT: "正しい場所を選択してください：",
    CONFLICT_LABEL: "選択オフィスの既定プリンターが既存。**上書き** か **スキップ** ？",
    SKIP_BTN: "スキップ",
    OVERWRITE_LABEL: "上書きインストール",
    SET_DEFAULT_LABEL: "既定のプリンターに設定",
    DEFAULT_CHOICE_LABEL: "既定のプリンター：",
    EXISTING_PRINTERS: "既存プリンター **%d** 台、削除するにはチェック：",
    OK_LABEL: "OK",
    CANCEL_LABEL: "キャンセル",
    INSTALLED_LABEL: "✅ %s をインストールしました",
    SKIP_INSTALL_MSG: "ℹ️ %s は既に存在します。操作不要",
    OVERWRITTEN_MSG: "✅ %s を上書きインストールしました",
    REMOVED_MSG: "🗑️ %s を削除しました",
    FAIL_PREFIX: "❌ インストール失敗：",
    NO_LOCATION: "場所が検出されませんでした",
    ADMIN_PROMPT: "プリンタードライバーのインストールには管理者権限が必要です",
    INSTALL_FAILED_MSG: "❌ %s が2回試行後もインストールに失敗しました",
    REMOVE_FAILED_MSG: "❌ %s が2回試行後も削除に失敗しました",
    FAIL_CAUSE_LPADMIN: "キューの作成に失敗しました（lpadmin エラー）",
    FAIL_CAUSE_VERIFY: "キューの登録を確認できませんでした",
    FAIL_CAUSE_ENABLE: "キューを有効化できませんでした",
    FAIL_CAUSE_ACCEPT: "新規ジョブを受け付ける設定にできませんでした",
    FAIL_CAUSE_DEFAULT: "既定プリンターに設定できませんでした",
    FAIL_CAUSE_DELETE: "プリンターを削除できませんでした",
    FAIL_CAUSE_UNKNOWN: "不明なエラーが発生しました",
    REVIEW_TITLE: "確認",
    REVIEW_INSTALL: "インストール：",
    REVIEW_ADD_INSTALL: "追加インストール：",
    REVIEW_CONFLICT: "競合：",
    REVIEW_DEFAULT_PRINTER: "既定プリンター：",
    REVIEW_REMOVE: "削除：",
    REVIEW_NONE: "なし",
    REVIEW_SKIPPED_ADDED: "スキップ（重複）：",
    REVIEW_FILTERED_REMOVE: "フィルター済（インストール対象）：",
    BTN_ADD_MORE: "＋ 追加",
    BTN_ADD: "追加",
    BTN_CANCEL: "キャンセル",
    SELECT_ALL: "すべて選択",
    NO_MORE_TO_ADD: "追加なし",
    TAB_INSTALL: "インストール",
    TAB_REMOVE: "削除",
  },
  ko: {
    TITLE: "프린터 드라이버 설치",
    DETECTING: "감지 중...",
    INSTALLING: "프린터 설치/제거 중입니다. 잠시만 기다려 주세요...",
    CONFIRM_FMT: "%s 감지, 클릭하여 다른 오피스 선택",
    PICKER_PROMPT: "올바른 위치를 선택하세요：",
    CONFLICT_LABEL: "선택 사무실의 기본 프린터가 이미 있음. **덮어쓰기** / **건너뛰기** ？",
    SKIP_BTN: "건너뛰기",
    OVERWRITE_LABEL: "덮어쓰기",
    SET_DEFAULT_LABEL: "기본 프린터로 설정",
    DEFAULT_CHOICE_LABEL: "기본 프린터:",
    EXISTING_PRINTERS: "기존 프린터 **%d** 대, 제거하려면 선택：",
    OK_LABEL: "확인",
    CANCEL_LABEL: "취소",
    INSTALLED_LABEL: "✅ %s 설치 완료",
    SKIP_INSTALL_MSG: "ℹ️ %s 이(가) 이미 존재합니다. 작업 불필요",
    OVERWRITTEN_MSG: "✅ %s 덮어쓰기 설치 완료",
    REMOVED_MSG: "🗑️ %s 제거 완료",
    FAIL_PREFIX: "❌ 설치 실패：",
    NO_LOCATION: "위치를 감지할 수 없음",
    ADMIN_PROMPT: "프린터 드라이버 설치를 위해 관리자 권한이 필요합니다",
    INSTALL_FAILED_MSG: "❌ %s 2회 시도 후에도 설치에 실패했습니다",
    REMOVE_FAILED_MSG: "❌ %s 2회 시도 후에도 제거에 실패했습니다",
    FAIL_CAUSE_LPADMIN: "인쇄 큐를 만들지 못했습니다 (lpadmin 오류)",
    FAIL_CAUSE_VERIFY: "큐 등록을 확인할 수 없습니다",
    FAIL_CAUSE_ENABLE: "인쇄 큐를 활성화할 수 없습니다",
    FAIL_CAUSE_ACCEPT: "새 작업을 받도록 큐를 설정할 수 없습니다",
    FAIL_CAUSE_DEFAULT: "기본 프린터로 설정할 수 없습니다",
    FAIL_CAUSE_DELETE: "프린터를 제거할 수 없습니다",
    FAIL_CAUSE_UNKNOWN: "알 수 없는 오류가 발생했습니다",
    REVIEW_TITLE: "확인",
    REVIEW_INSTALL: "설치：",
    REVIEW_ADD_INSTALL: "추가 설치：",
    REVIEW_CONFLICT: "충돌：",
    REVIEW_DEFAULT_PRINTER: "기본 프린터:",
    REVIEW_REMOVE: "제거：",
    REVIEW_NONE: "없음",
    REVIEW_SKIPPED_ADDED: "건너뜀 (중복):",
    REVIEW_FILTERED_REMOVE: "필터됨 (설치 대상):",
    BTN_ADD_MORE: "＋ 추가",
    BTN_ADD: "추가",
    BTN_CANCEL: "취소",
    SELECT_ALL: "전체 선택",
    NO_MORE_TO_ADD: "추가 없음",
    TAB_INSTALL: "설치",
    TAB_REMOVE: "제거",
  },
  "zh-Hant": {
    TITLE: "印表機驅動程式安裝程式",
    DETECTING: "偵測中...",
    INSTALLING: "正在安裝/移除印表機，請稍候...",
    CONFIRM_FMT: "偵測到您位於 %s，點擊可選其他辦公室",
    PICKER_PROMPT: "請選擇正確的位置：",
    CONFLICT_LABEL: "所選辦公室的預設印表機已存在，**覆蓋** 或 **跳過** ？",
    SKIP_BTN: "跳過",
    OVERWRITE_LABEL: "覆蓋安裝",
    SET_DEFAULT_LABEL: "設為預設印表機",
    DEFAULT_CHOICE_LABEL: "選擇預設印表機：",
    EXISTING_PRINTERS: "本機已存在 **%d** 台印表機，勾選可移除：",
    OK_LABEL: "好",
    CANCEL_LABEL: "取消",
    INSTALLED_LABEL: "✅ %s 已成功安裝",
    SKIP_INSTALL_MSG: "ℹ️ %s 已存在，無需操作",
    OVERWRITTEN_MSG: "✅ %s 已成功覆蓋安裝",
    REMOVED_MSG: "🗑️ %s 已成功移除",
    FAIL_PREFIX: "❌ 安裝失敗：",
    NO_LOCATION: "未偵測到位置",
    ADMIN_PROMPT: "印表機驅動程式安裝需要管理員權限",
    INSTALL_FAILED_MSG: "❌ %s 兩次嘗試後仍安裝失敗",
    REMOVE_FAILED_MSG: "❌ %s 兩次嘗試後仍移除失敗",
    FAIL_CAUSE_LPADMIN: "無法建立印表機佇列（lpadmin 回傳錯誤）",
    FAIL_CAUSE_VERIFY: "佇列註冊驗證未通過（查無此印表機）",
    FAIL_CAUSE_ENABLE: "無法啟用印表機佇列",
    FAIL_CAUSE_ACCEPT: "無法設定為接受新作業",
    FAIL_CAUSE_DEFAULT: "無法設為預設印表機",
    FAIL_CAUSE_DELETE: "兩次嘗試後仍無法刪除印表機",
    FAIL_CAUSE_UNKNOWN: "發生未知錯誤",
    REVIEW_TITLE: "確認操作",
    REVIEW_INSTALL: "安裝：",
    REVIEW_ADD_INSTALL: "追加安裝：",
    REVIEW_CONFLICT: "衝突處理：",
    REVIEW_DEFAULT_PRINTER: "預設印表機：",
    REVIEW_REMOVE: "移除：",
    REVIEW_NONE: "無",
    REVIEW_SKIPPED_ADDED: "跳過（重複）：",
    REVIEW_FILTERED_REMOVE: "過濾（待安裝）：",
    BTN_ADD_MORE: "＋ 繼續新增",
    BTN_ADD: "新增",
    BTN_CANCEL: "取消",
    SELECT_ALL: "全選",
    NO_MORE_TO_ADD: "無更多可新增",
    TAB_INSTALL: "安裝",
    TAB_REMOVE: "移除",
  },
};

// 基于 config.json 的真实位置数据
export const LOCATIONS = [
  { name: "Osaka - JP Tower", ips: ["30.61.40.40"], names: ["Printer-Osaka"] },
  { name: "Tokyo - Business Tower", ips: ["30.61.30.30"], names: ["Printer-Tencent"] },
  { name: "Tokyo - Mori Tower", ips: ["30.61.34.29", "30.61.34.30"], names: ["Printer-BG", "Printer-Game"] },
];

export function makeInitialState(opts = {}) {
  const {
    lang = "zh",
    detected_location = "Osaka - JP Tower",
    existing = [
      { name: "Printer-Old-A", ip: "30.61.40.99" },
      { name: "Printer-Osaka", ip: "30.61.40.40" },
      { name: "My HP Printer", ip: "192.168.1.10" },
    ],
  } = opts;

  const locations = LOCATIONS.map((l) => l.name);
  const loc_ips = {};
  const loc_names = {};
  const conflict = {};
  const byIp = new Map(existing.map((p) => [p.ip, p.name]));

  for (const l of LOCATIONS) {
    loc_ips[l.name] = l.ips;
    loc_names[l.name] = l.names;
    conflict[l.name] = l.ips.some((ip) => byIp.has(ip));
  }

  const det = LOCATIONS.find((l) => l.name === detected_location);
  return {
    lang,
    strings: MOCK_STRINGS[lang] || MOCK_STRINGS.en,
    detected_location: detected_location || null,
    detected_name: det ? det.names[0] : "",
    detected_ip: det ? det.ips[0] : "",
    locations,
    loc_ips,
    loc_names,
    conflict,
    existing,
    has_driver_ppd: true,
  };
}

// 模拟的后端调用
export function createMockApi(stateRef) {
  return {
    getState: async () => stateRef.current,
    getStrings: async (lang) => MOCK_STRINGS[lang] || MOCK_STRINGS.en,
    runInstall: async (req) => {
      await new Promise((r) => setTimeout(r, 800));
      // 根据请求生成模拟结果（含设为默认）
      const msgs = [];
      const defInfo = req.setDefault === false ? "（未设为默认）" : req.defaultPrinter ? `（默认：${req.defaultPrinter}）` : "";
      if (req.overwrite) {
        msgs.push({ kind: "installed", text: `✅ ${req.location} 已成功覆盖安装${defInfo}` });
      } else if (req.location) {
        const loc = LOCATIONS.find((l) => l.name === req.location);
        const hasConflict = loc && loc.ips.some((ip) => stateRef.current.existing.some((p) => p.ip === ip));
        if (hasConflict && !req.overwrite) {
          msgs.push({ kind: "skipped", text: `ℹ️ ${req.location} 已存在，无需操作` });
        } else {
          msgs.push({ kind: "installed", text: `✅ ${req.location} 已成功安装${defInfo}` });
        }
      }
      if (req.delete && req.delete.length) {
        msgs.push({ kind: "removed", text: `🗑️ ${req.delete.join(", ")} 已成功移除` });
      }
      // 随机模拟失败
      if (stateRef.current._forceFail) {
        return { messages: [{ kind: "install-failed", text: "❌ Printer-Osaka 两次尝试后仍安装失败：无法创建打印机队列（lpadmin 返回错误）" }], cancelled: false };
      }
      return { messages: msgs, cancelled: false, skipped_all: msgs.length === 0 };
    },
  };
}
