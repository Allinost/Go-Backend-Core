package goodser

import (
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

// exportFontEntry 缓存已解析的字体文件。
type exportFontEntry struct {
	font *sfnt.Font
}

var exportFontCache sync.Map

// exportFontCandidates 返回中文字体候选路径，custom 优先。
func exportFontCandidates(custom string) []string {
	out := make([]string, 0, 16)
	if s := strings.TrimSpace(custom); s != "" {
		out = append(out, s)
	}
	out = append(out,
		"assets/fonts/NotoSansSC-Regular.otf",
		"assets/fonts/NotoSansSC-Regular.ttf",
		"assets/fonts/NotoSansCJKsc-Regular.otf",
		"assets/fonts/SourceHanSansSC-Regular.otf",
	)
	if runtime.GOOS == "windows" {
		out = append(out,
			`C:\Windows\Fonts\msyh.ttc`,
			`C:\Windows\Fonts\msyhbd.ttc`,
			`C:\Windows\Fonts\simhei.ttf`,
			`C:\Windows\Fonts\simsun.ttc`,
		)
	} else {
		out = append(out,
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/opentype/noto/NotoSansCJKsc-Regular.otf",
			"/usr/share/fonts/opentype/noto/NotoSansSC-Regular.otf",
			"/usr/share/fonts/google-noto-cjk/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
			"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
			"/System/Library/Fonts/PingFang.ttc",
			"/System/Library/Fonts/STHeiti Light.ttc",
		)
	}
	return out
}

// resolveExportFont 按路径缓存解析结果；找不到可用字体时返回 nil。
func resolveExportFont(custom string) *sfnt.Font {
	key := strings.TrimSpace(custom)
	if key == "" {
		key = "<auto>"
	}
	if cached, ok := exportFontCache.Load(key); ok {
		entry, _ := cached.(exportFontEntry)
		return entry.font
	}

	var found *sfnt.Font
	for _, path := range exportFontCandidates(custom) {
		f, err := loadExportFontFile(path)
		if err == nil && f != nil {
			found = f
			break
		}
	}
	if found == nil {
		logger.Warn().Msg("goodser: 未找到中文字体，导出图中文将回退为方块；请配置 goodser.export.font_path 或安装 Noto Sans CJK")
	}
	exportFontCache.Store(key, exportFontEntry{font: found})
	return found
}

func loadExportFontFile(path string) (*sfnt.Font, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) >= 4 && string(data[:4]) == "ttcf" {
		collection, err := sfnt.ParseCollection(data)
		if err != nil {
			return nil, err
		}
		if collection.NumFonts() == 0 {
			return nil, os.ErrNotExist
		}
		return collection.Font(0)
	}
	return sfnt.Parse(data)
}

// exportFace 返回指定字号的字体；无可用字体时退化为 basicfont（仅 ASCII）。
func exportFace(custom string, size float64) font.Face {
	if size < 8 {
		size = 8
	}
	if f := resolveExportFont(custom); f != nil {
		face, err := opentype.NewFace(f, &opentype.FaceOptions{
			Size:    size,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err == nil && face != nil {
			return face
		}
	}
	return basicfont.Face7x13
}
