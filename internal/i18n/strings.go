package i18n

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func Detect() string {
	if runtime.GOOS == "windows" {
		return detectWindows()
	}
	out, err := exec.Command("osascript", "-e", "user locale of (system info)").Output()
	if err == nil {
		lang := strings.TrimSpace(string(out))
		lang = strings.Split(lang, "_")[0]
		lang = strings.Split(lang, "-")[0]
		switch lang {
		case "ja", "ko", "zh":
			return lang
		}
		return "en"
	}
	lang := os.Getenv("LANG")
	if lang != "" {
		lang = strings.Split(lang, "_")[0]
		lang = strings.Split(lang, "-")[0]
		switch lang {
		case "ja", "ko", "zh":
			return lang
		}
	}
	return "en"
}

func detectWindows() string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-WinSystemLocale).Name").Output()
	if err == nil {
		lang := strings.TrimSpace(string(out))
		lang = strings.Split(lang, "-")[0]
		switch lang {
		case "ja", "ko", "zh":
			return lang
		}
	}
	return "en"
}

var stringsMap = map[string]map[string]string{
	"ADMIN_INSTALL_PROMPT": {
		"en": "Printer driver installation requires admin privileges",
		"ja": "プリンタードライバーのインストールには管理者権限が必要です",
		"ko": "프린터 드라이버 설치를 위해 관리자 권한이 필요합니다",
		"zh": "打印机驱动安装需要管理员权限",
	},
	"ADMIN_DELETE_PROMPT": {
		"en": "Printer deletion requires admin privileges",
		"ja": "プリンターの削除には管理者権限が必要です",
		"ko": "프린터 삭제를 위해 관리자 권한이 필요합니다",
		"zh": "打印机删除需要管理员权限",
	},
	"DEL_MSG": {
		"en": "If you need to delete other printers, please select them.\\nAuto-cancelling in 10s...",
		"ja": "他のプリンターを削除する場合は選択してください。\\n10秒後に自動キャンセル...",
		"ko": "다른 프린터를 삭제하려면 선택하세요.\\n10초 후 자동 취소...",
		"zh": "如果需要删除其他打印机，请选择。\\n10秒后自动取消...",
	},
	"DEL_BTN": {
		"en": "Select to Delete",
		"ja": "削除するを選択",
		"ko": "삭제 선택",
		"zh": "选择删除",
	},
	"SKIP_BTN": {
		"en": "Skip",
		"ja": "スキップ",
		"ko": "건너뛰기",
		"zh": "跳过",
	},
	"CHOOSE_PROMPT": {
		"en": "If needed, check printers to remove:",
		"ja": "必要に応じて削除するプリンターをチェックしてください：",
		"ko": "필요한 경우 제거할 프린터를 선택하세요：",
		"zh": "如果需要移除，请勾选需要移除的打印机：",
	},
	"DELETED_PREFIX": {
		"en": "Deleted:",
		"ja": "削除完了：",
		"ko": "삭제됨：",
		"zh": "已删除：",
	},
	"CONFIRM_FMT": {
		"en": "Detected at %s, uncheck to pick another location",
		"ja": "%s を検出、チェックを外すと別の場所を選択できます",
		"ko": "%s 감지됨, 체크 해제 시 다른 위치 선택 가능",
		"zh": "检测到您在%s，取消勾选也可选择其他位置",
	},
	"YES_LABEL": {
		"en": "Yes",
		"ja": "はい",
		"ko": "예",
		"zh": "是",
	},
	"NO_LABEL": {
		"en": "No",
		"ja": "いいえ",
		"ko": "아니오",
		"zh": "否",
	},
	"PICKER_PROMPT": {
		"en": "Select the correct location:",
		"ja": "正しい場所を選択してください：",
		"ko": "올바른 위치를 선택하세요：",
		"zh": "请选择正确的位置：",
	},
	"NAME_PROMPT": {
		"en": "Enter a name for this printer:",
		"ja": "プリンター名を入力してください：",
		"ko": "프린터 이름을 입력하세요：",
		"zh": "请输入打印机名称：",
	},
	"CONFLICT_LABEL": {
		"en": "A printer exists at this IP, choose:",
		"ja": "このIPにプリンターが既存、選択：",
		"ko": "이 IP에 프린터 존재, 선택：",
		"zh": "同IP打印机已存在，请选择：",
	},
	"OVERWRITE_LABEL": {
		"en": "Overwrite",
		"ja": "上書きインストール",
		"ko": "덮어쓰기",
		"zh": "覆盖安装",
	},
	"AUTO_CLOSE": {
		"en": "\\nAuto-closing in 5s...",
		"ja": "\\n5秒後に自動で閉じます...",
		"ko": "\\n5초 후 자동으로 닫힙니다...",
		"zh": "\\n5秒后自动关闭...",
	},
	"FAIL_PREFIX": {
		"en": "❌ Installation failed:",
		"ja": "❌ インストール失敗：",
		"ko": "❌ 설치 실패：",
		"zh": "❌ 安装失败：",
	},
	"INSTALLED_LABEL": {
		"en": "✅ %s installed successfully",
		"ja": "✅ %s をインストールしました",
		"ko": "✅ %s 설치 완료",
		"zh": "✅ %s 已成功安装",
	},
	"OTHER_PRINTERS_LABEL": {
		"en": "Other printers: ",
		"ja": "他のプリンター：",
		"ko": "다른 프린터：",
		"zh": "其他打印机：",
	},
	"NONE_LABEL": {
		"en": "none",
		"ja": "なし",
		"ko": "없음",
		"zh": "无",
	},
	"OK_LABEL": {
		"en": "OK",
		"ja": "OK",
		"ko": "확인",
		"zh": "好",
	},
	"ROSETTA_PROMPT": {
		"en": "Rosetta 2 is required to install the printer driver.\\nInstall now?",
		"ja": "プリンタードライバーのインストールにRosetta 2が必要です。\\n今すぐインストールしますか？",
		"ko": "프린터 드라이버 설치에 Rosetta 2가 필요합니다.\\n지금 설치하시겠습니까?",
		"zh": "打印机驱动安装需要 Rosetta 2。\\n立即安装？",
	},
	"INSTALL_LABEL": {
		"en": "Install",
		"ja": "インストール",
		"ko": "설치",
		"zh": "安装",
	},
	"CANCEL_LABEL": {
		"en": "Cancel",
		"ja": "キャンセル",
		"ko": "취소",
		"zh": "取消",
	},
	"TITLE": {
		"en": "Printer Driver Installer",
		"ja": "プリンタードライバーインストーラー",
		"ko": "프린터 드라이버 설치",
		"zh": "打印机驱动安装",
	},
	"SKIP_INSTALL_MSG": {
		"en": "ℹ️ %s already exists, no action needed",
		"ja": "ℹ️ %s は既に存在します。操作不要",
		"ko": "ℹ️ %s 이(가) 이미 존재합니다. 작업 불필요",
		"zh": "ℹ️ %s 已存在，无需操作",
	},
	"OVERWRITTEN_MSG": {
		"en": "✅ %s updated successfully",
		"ja": "✅ %s を上書きインストールしました",
		"ko": "✅ %s 덮어쓰기 설치 완료",
		"zh": "✅ %s 已成功覆盖安装",
	},
	"REMOVED_MSG": {
		"en": "🗑️ %s removed successfully",
		"ja": "🗑️ %s を削除しました",
		"ko": "🗑️ %s 제거 완료",
		"zh": "🗑️ %s 已成功移除",
	},
	"WINDOW_TITLE": {
		"en": "Printer Installer",
		"ja": "プリンターインストーラー",
		"ko": "프린터 설치 프로그램",
		"zh": "打印机安装程序",
	},
	"LOCATION_PREFIX": {
		"en": "Location: %s",
		"ja": "場所: %s",
		"ko": "위치: %s",
		"zh": "位置: %s",
	},
	"EXISTING_PRINTERS": {
		"en": "Existing printers (%d), check to remove:",
		"ja": "既存プリンター (%d)、削除する場合はチェック：",
		"ko": "기존 프린터 (%d), 제거하려면 선택：",
		"zh": "现有打印机 (%d)，如需移除请勾选：",
	},
	"DETECTING": {
		"en": "Detecting...",
		"ja": "検出中...",
		"ko": "감지 중...",
		"zh": "检测中...",
	},
	"NO_LOCATION": {
		"en": "No location detected",
		"ja": "場所が検出されませんでした",
		"ko": "위치를 감지할 수 없음",
		"zh": "未检测到位置",
	},
}

var detectedLang string

func init() {
	detectedLang = Detect()
}

func T(key string, args ...interface{}) string {
	s := Get(key, detectedLang)
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

func Get(key, lang string) string {
	if m, ok := stringsMap[key]; ok {
		if v, ok := m[lang]; ok {
			return v
		}
		return m["en"]
	}
	return ""
}

func AllEnv(lang string) string {
	if lang == "" {
		lang = Detect()
	}
	var b strings.Builder
	for key := range stringsMap {
		val := Get(key, lang)
		b.WriteString(key)
		b.WriteString("='")
		b.WriteString(val)
		b.WriteString("'\n")
	}
	return b.String()
}
