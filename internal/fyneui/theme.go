package fyneui

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"printer-installer/internal/i18n"
)

// cjkFontTheme overrides the app text fonts with a CJK-capable system font so
// that full-width characters share the same line metrics as Latin glyphs.
//
// Fyne's bundled fonts (Inter/NotoSans) have no CJK glyphs. When a rune is
// missing it falls back to a scanned system font (internal/painter/font.go
// lookupRuneFont), and each shaped run is placed using that font's own
// Ascent. A Japanese system font therefore shifts full-width glyphs up or
// down relative to the label box. Using a single font for every style keeps
// all glyphs aligned.
type cjkFontTheme struct {
	fyne.Theme
	font fyne.Resource
}

func (t *cjkFontTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Symbol {
		return theme.DefaultSymbolFont()
	}
	return t.font
}

func applyCJKTheme(a fyne.App) {
	if runtime.GOOS != "windows" {
		return
	}
	path := cjkFontPath(i18n.Lang())
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	a.Settings().SetTheme(&cjkFontTheme{
		Theme: theme.DefaultTheme(),
		font:  fyne.NewStaticResource("cjk-font", extractFirstFace(data)),
	})
}

// extractFirstFace returns a standalone TTF for the first face of a TrueType
// collection (.ttc). Windows CJK fonts (Yu Gothic, Meiryo, MS Gothic, YaHei,
// SimSun) ship as collections, and Fyne loads theme fonts with font.ParseTTF,
// which rejects them.
//
// Within a .ttc each face is a complete sfnt whose table directory is written
// with absolute file offsets. Converting a face to a standalone TTF therefore
// means rebuilding the directory with offsets relative to the face start and
// copying each table. checkSumAdjustment is not re-derived; parsers used by
// Fyne do not validate it.
func extractFirstFace(data []byte) []byte {
	if len(data) < 12 || string(data[:4]) != "ttcf" {
		return data
	}
	numFonts := binary.BigEndian.Uint32(data[8:12])
	if numFonts == 0 {
		return data
	}
	start := binary.BigEndian.Uint32(data[12:16])
	if int(start)+16 > len(data) {
		return data
	}

	numTables := int(binary.BigEndian.Uint16(data[start+4 : start+6]))
	headerSize := 12 + 16*numTables
	if int(start)+headerSize > len(data) {
		return data
	}

	type table struct {
		tag      [4]byte
		checksum uint32
		offset   uint32
		length   uint32
	}
	tables := make([]table, 0, numTables)
	maxEnd := int(start)
	for i := 0; i < numTables; i++ {
		rec := int(start) + 12 + i*16
		var t table
		copy(t.tag[:], data[rec:rec+4])
		t.checksum = binary.BigEndian.Uint32(data[rec+4 : rec+8])
		t.offset = binary.BigEndian.Uint32(data[rec+8 : rec+12])
		t.length = binary.BigEndian.Uint32(data[rec+12 : rec+16])
		if int(t.offset)+int(t.length) > maxEnd {
			maxEnd = int(t.offset) + int(t.length)
		}
		tables = append(tables, t)
	}

	out := make([]byte, headerSize+(maxEnd-int(start)))
	copy(out[:4], data[start:start+4])
	copy(out[4:6], data[start+4:start+6])
	copy(out[6:12], data[start+6:start+12])

	for i, t := range tables {
		rec := 12 + i*16
		copy(out[rec:rec+4], t.tag[:])
		binary.BigEndian.PutUint32(out[rec+4:], t.checksum)
		newOff := int(t.offset) - int(start)
		if newOff < headerSize {
			return data
		}
		binary.BigEndian.PutUint32(out[rec+8:], uint32(newOff))
		binary.BigEndian.PutUint32(out[rec+12:], t.length)
		copy(out[newOff:], data[t.offset:t.offset+t.length])
	}
	return out
}

func cjkFontPath(lang string) string {
	fontsDir := filepath.Join(os.Getenv("SystemRoot"), "Fonts")
	if _, err := os.Stat(fontsDir); err != nil {
		fontsDir = `C:\Windows\Fonts`
	}

	var candidates []string
	switch lang {
	case "ja":
		candidates = []string{"YuGothR.ttc", "msgothic.ttc", "meiryo.ttc"}
	case "ko":
		candidates = []string{"malgun.ttf", "gulim.ttc"}
	case "zh":
		candidates = []string{"msyh.ttc", "msjh.ttc", "simsun.ttc"}
	default:
		return ""
	}
	for _, c := range candidates {
		p := filepath.Join(fontsDir, c)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
