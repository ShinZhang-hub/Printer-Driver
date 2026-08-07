package fyneui

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-text/typesetting/font"
)

func TestCJKTTCParse(t *testing.T) {
	all := []string{
		"YuGothR.ttc", "msgothic.ttc", "meiryo.ttc", // ja
		"msyh.ttc", "msjh.ttc", "simsun.ttc", // zh
		"malgun.ttf", "gulim.ttc", // ko
	}
	for _, name := range all {
		p := filepath.Join(`C:\Windows\Fonts`, name)
		if _, err := os.Stat(p); err != nil {
			t.Logf("skip %s (not installed)", name)
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		f, err := font.ParseTTF(bytes.NewReader(extractFirstFace(data)))
		if err != nil {
			t.Fatalf("ParseTTF(%s): %v", name, err)
		}
		if _, ok := f.NominalGlyph('日'); !ok {
			t.Fatalf("%s missing CJK glyph", name)
		}
		if _, ok := f.NominalGlyph('A'); !ok {
			t.Fatalf("%s missing Latin glyph", name)
		}
		t.Logf("OK %s", name)
	}
}

func TestCJKFontPath(t *testing.T) {
	for _, lang := range []string{"ja", "zh", "ko", "en"} {
		p := cjkFontPath(lang)
		t.Logf("lang=%s font=%s", lang, p)
		if lang == "en" && p != "" {
			t.Fatalf("en should have no font override, got %s", p)
		}
	}
}
