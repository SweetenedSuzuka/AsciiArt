package gui

// Breeze 是 KDE 的视觉风格：扁平的表面、单一的饱和蓝色强调色、细线边框、很小的圆角和较大的留白。下面的配色取自 Breeze Light，使窗口在 KDE 桌面环境中呈现出一致的观感。
// 虽然说我不用 KDE，但是我懒得思考怎么设计一个小工具，所以直接照抄配色。
type palette struct {
	window     uint32 // #EFF0F1 窗口背景色
	view       uint32 // #FFFFFF 视图与侧栏背景色
	card       uint32 // #FFFFFF 卡片背景色
	cardInset  uint32 // #F7F8F8 内嵌面板色
	canvas     uint32 // #1B1E20 字符画背景色
	border     uint32 // #C9CDD1 边框色
	divider    uint32 // #D6DBDF 分隔线色
	text       uint32 // #232627 主要文字色
	textDim    uint32 // #7F8C8D 次要文字色
	textInv    uint32 // #FFFFFF 强调色上的文字色
	accent     uint32 // #3DAEE9 Breeze 蓝
	accentHi   uint32 // #4CB8EF 悬停状态色
	accentLo   uint32 // #2980B9 按下状态色
	accentTint uint32 // #3DAEE9 12% 透明度，用于选中状态的淡色叠加
	hover      uint32 // #EDEFF0 轻微的悬停底色
	press      uint32 // #E2E5E7 按下底色
	track      uint32 // #C9CDD1 滑块凹槽色
	focus      uint32 // #3DAEE9 焦点色
	success    uint32 // #27AE60 成功状态色
	warn       uint32 // #DA4453 警告状态色
}

func breezePalette() palette {
	return palette{
		window:     argb(0xEF, 0xF0, 0xF1, 0xFF),
		view:       argb(0xFF, 0xFF, 0xFF, 0xFF),
		card:       argb(0xFF, 0xFF, 0xFF, 0xFF),
		cardInset:  argb(0xF7, 0xF8, 0xF8, 0xFF),
		canvas:     argb(0x1B, 0x1E, 0x20, 0xFF),
		border:     argb(0xC9, 0xCD, 0xD1, 0xFF),
		divider:    argb(0xD6, 0xDB, 0xDF, 0xFF),
		text:       argb(0x23, 0x26, 0x27, 0xFF),
		textDim:    argb(0x7F, 0x8C, 0x8D, 0xFF),
		textInv:    argb(0xFF, 0xFF, 0xFF, 0xFF),
		accent:     argb(0x3D, 0xAE, 0xE9, 0xFF),
		accentHi:   argb(0x4C, 0xB8, 0xEF, 0xFF),
		accentLo:   argb(0x29, 0x80, 0xB9, 0xFF),
		accentTint: argb(0x3D, 0xAE, 0xE9, 0x1F),
		hover:      argb(0xED, 0xEF, 0xF0, 0xFF),
		press:      argb(0xE2, 0xE5, 0xE7, 0xFF),
		track:      argb(0xC9, 0xCD, 0xD1, 0xFF),
		focus:      argb(0x3D, 0xAE, 0xE9, 0xFF),
		success:    argb(0x27, 0xAE, 0x60, 0xFF),
		warn:       argb(0xDA, 0x44, 0x53, 0xFF),
	}
}

// theme 把配色与当前显示器的像素缩放比，以及按该缩放比构建的字体打包在一起。
type theme struct {
	pal   palette
	scale float32

	uiSmall  *font
	uiBase   *font
	uiMedium *font
	uiBold   *font
	uiTitle  *font
}

func newTheme(dpi int32) *theme {
	if dpi <= 0 {
		dpi = 96
	}
	s := float32(dpi) / 96.0
	t := &theme{pal: breezePalette(), scale: s}

	// Breeze 本身使用 Noto Sans 字体；Segoe UI 是 Windows 上保证存在、且与它最接近的字体。
	family := "Segoe UI"
	if testFont := newFont("Noto Sans", 12, false); testFont != nil {
		testFont.free()
		family = "Noto Sans"
	}

	t.uiSmall = newFont(family, 11.5*s, false)
	t.uiBase = newFont(family, 13*s, false)
	t.uiMedium = newFont(family, 13.5*s, false)
	t.uiBold = newFont(family, 13*s, true)
	t.uiTitle = newFont(family, 20*s, false)
	return t
}

func (t *theme) free() {
	for _, f := range []*font{t.uiSmall, t.uiBase, t.uiMedium, t.uiBold, t.uiTitle} {
		f.free()
	}
}

// px 把设计像素（96 DPI）换算为设备像素。
func (t *theme) px(v float32) int32 { return int32(v*t.scale + 0.5) }

func (t *theme) fpx(v float32) float32 { return v * t.scale }
