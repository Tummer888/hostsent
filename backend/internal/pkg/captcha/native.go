package captcha

import (
	"bytes"
	"crypto/rand"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	mathrand "math/rand"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// 字符集刻意去掉易混字符 0/O/1/I/l（doc91 §3.2）。
const charset = "2345678ABCDEFGHJKLMNPQRSTUVWXYZ"

// 图像尺寸。
const (
	imgWidth  = 160
	imgHeight = 56
)

// 干扰强度等级。
const (
	LevelEasy   = "easy"
	LevelNormal = "normal"
	LevelHard   = "hard"
)

// NativeGenerator 原生图形验证码生成器（内置兜底，永远可用）。
type NativeGenerator struct{}

// NewNativeGenerator 创建原生生成器。
func NewNativeGenerator() *NativeGenerator { return &NativeGenerator{} }

// Generate 生成图形验证码：返回答案与 PNG 字节。
//
// 答案仅供调用方写入缓存，**绝不能随响应返回**（doc91 §3.2 关键安全点）。
func (g *NativeGenerator) Generate(level string) (answer string, pngBytes []byte, err error) {
	length := 4
	if level == LevelHard {
		length = 5
	}
	answer = randomChars(length)

	img, err := render(answer, level)
	if err != nil {
		return "", nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", nil, err
	}
	return answer, buf.Bytes(), nil
}

// randomChars 使用 crypto/rand 取字符（不用 math/rand 生成安全相关随机量）。
func randomChars(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 失败时退化为时间熵，仍不暴露给调用方：这里只保证不 panic。
		for i := range b {
			b[i] = byte(time.Now().UnixNano() >> (i * 3))
		}
	}
	out := make([]byte, n)
	for i, v := range b {
		out[i] = charset[int(v)%len(charset)]
	}
	return string(out)
}

// render 渲染一张带随机扰动与干扰的验证码图。
//
// 防识别设计（doc91 §3.2）逐条落实：
//  1. 每字符独立随机字号（22–30px）、旋转（-30°~+30°）、颜色、x/y 偏移；
//  2. 背景随机浅色 + 噪点；干扰线（贝塞尔）；干扰字（透明）；
//  3. 整图正弦波扭曲，A/ω/φ 随机；
//  4. level 控制强度：easy 无扭曲 2 条线 / normal 默认 / hard 强扭曲 + 噪点加倍。
func render(answer, level string) (image.Image, error) {
	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano() ^ int64(len(answer))))
	bg := image.NewRGBA(image.Rect(0, 0, imgWidth, imgHeight))
	bgColor := color.RGBA{
		R: uint8(225 + rng.Intn(25)),
		G: uint8(225 + rng.Intn(25)),
		B: uint8(225 + rng.Intn(25)),
		A: 255,
	}
	draw.Draw(bg, bg.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	drawNoise(bg, rng, level)
	drawLines(bg, rng, level)
	drawDecoys(bg, rng, level)
	if err := drawChars(bg, rng, answer); err != nil {
		return nil, err
	}
	return distort(bg, rng, level), nil
}

// newFace 按字号创建字体面（Go Regular，内置 TTF，镜像无需额外字体文件）。
func newFace(size float64) (font.Face, error) {
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

// colorPalette 深色前景候选（每字符随机取一）。
var colorPalette = []color.RGBA{
	{R: 31, G: 41, B: 55, A: 255},
	{R: 127, G: 29, B: 29, A: 255},
	{R: 21, G: 94, B: 117, A: 255},
	{R: 46, G: 84, B: 30, A: 255},
	{R: 88, G: 28, B: 135, A: 255},
	{R: 120, G: 53, B: 15, A: 255},
}

// drawNoise 背景噪点：normal 120 个，hard 加倍。
func drawNoise(img *image.RGBA, rng *mathrand.Rand, level string) {
	n := 120
	if level == LevelHard {
		n = 240
	}
	for i := 0; i < n; i++ {
		x := rng.Intn(imgWidth)
		y := rng.Intn(imgHeight)
		img.Set(x, y, color.RGBA{
			R: uint8(rng.Intn(200)),
			G: uint8(rng.Intn(200)),
			B: uint8(rng.Intn(200)),
			A: uint8(80 + rng.Intn(120)),
		})
	}
}

// drawLines 干扰线：easy 2 条，normal 3–5 条，hard 5–7 条（二次贝塞尔，半透明）。
func drawLines(img *image.RGBA, rng *mathrand.Rand, level string) {
	count := 3 + rng.Intn(3)
	switch level {
	case LevelEasy:
		count = 2
	case LevelHard:
		count = 5 + rng.Intn(3)
	}
	for i := 0; i < count; i++ {
		c := color.RGBA{
			R: uint8(rng.Intn(160)),
			G: uint8(rng.Intn(160)),
			B: uint8(rng.Intn(160)),
			A: uint8(120 + rng.Intn(80)),
		}
		x0, y0 := rng.Intn(imgWidth), rng.Intn(imgHeight)
		x1, y1 := rng.Intn(imgWidth), rng.Intn(imgHeight)
		cx, cy := rng.Intn(imgWidth), rng.Intn(imgHeight)
		steps := 60
		for t := 0; t <= steps; t++ {
			ft := float64(t) / float64(steps)
			x := quad(float64(x0), float64(cx), float64(x1), ft)
			y := quad(float64(y0), float64(cy), float64(y1), ft)
			plot(img, int(x), int(y), c)
		}
	}
}

// drawDecoys 干扰字：2 个与答案无关的字符，透明度 25%，hard 时 4 个。
func drawDecoys(img *image.RGBA, rng *mathrand.Rand, level string) {
	if level == LevelEasy {
		return
	}
	n := 2
	if level == LevelHard {
		n = 4
	}
	for i := 0; i < n; i++ {
		face, err := newFace(float64(20 + rng.Intn(10)))
		if err != nil {
			return
		}
		ch := string(charset[rng.Intn(len(charset))])
		x := rng.Intn(imgWidth - 20)
		y := 18 + rng.Intn(imgHeight-28)
		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(color.RGBA{R: 90, G: 90, B: 90, A: 64}),
			Face: face,
			Dot:  fixed.P(x, y),
		}
		d.DrawString(ch)
		_ = face.Close()
	}
}

// drawChars 逐字符独立随机字号/颜色/y 偏移/字间距/旋转。
func drawChars(img *image.RGBA, rng *mathrand.Rand, answer string) error {
	step := float64(imgWidth-24) / float64(len(answer))
	for i, ch := range answer {
		size := float64(24 + rng.Intn(6))
		f, err := newFace(size)
		if err != nil {
			return err
		}
		x := 12 + int(float64(i)*step) + rng.Intn(4)
		y := 36 + rng.Intn(13) - 6
		angle := (rng.Float64()*60 - 30) * math.Pi / 180
		col := colorPalette[rng.Intn(len(colorPalette))]
		drawRotatedChar(img, f, string(ch), x, y, angle, col)
		_ = f.Close()
	}
	return nil
}

// drawRotatedChar 画一个旋转后的字符（旋转中心为字符基线点）。
func drawRotatedChar(dst *image.RGBA, face font.Face, ch string, x, y int, angle float64, col color.RGBA) {
	bounds := &image.RGBA{
		Pix:    dst.Pix,
		Stride: dst.Stride,
		Rect:   dst.Rect,
	}
	target := image.NewRGBA(image.Rect(x-20, y-40, x+40, y+20))
	d := &font.Drawer{
		Dst:  target,
		Src:  image.NewUniform(col),
		Face: face,
		Dot:  fixed.P(20, 40),
	}
	d.DrawString(ch)

	sin, cos := math.Sincos(angle)
	b := target.Bounds()
	for ty := b.Min.Y; ty < b.Max.Y; ty++ {
		for tx := b.Min.X; tx < b.Max.X; tx++ {
			r, g, bl, a := target.At(tx, ty).RGBA()
			if a == 0 {
				continue
			}
			dx := tx - 20
			dy := ty - 40
			nx := int(float64(dx)*cos-float64(dy)*sin) + x
			ny := int(float64(dx)*sin+float64(dy)*cos) + y
			plot(bounds, nx, ny, color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(bl >> 8), A: uint8(a >> 8)})
		}
	}
}

// distort 正弦波扭曲：y' = y + A*sin(ωx + φ)，参数随机；easy 不扭曲。
func distort(src *image.RGBA, rng *mathrand.Rand, level string) image.Image {
	if level == LevelEasy {
		return src
	}
	amp := 2.0 + rng.Float64()*2
	omega := 0.03 + rng.Float64()*0.04
	phi := rng.Float64() * 2 * math.Pi
	if level == LevelHard {
		amp = 4.0 + rng.Float64()*2
	}
	out := image.NewRGBA(src.Bounds())
	draw.Draw(out, out.Bounds(), image.NewUniform(color.RGBA{255, 255, 255, 255}), image.Point{}, draw.Src)
	for x := 0; x < imgWidth; x++ {
		offset := int(amp * math.Sin(omega*float64(x)+phi))
		for y := 0; y < imgHeight; y++ {
			sy := y - offset
			if sy < 0 || sy >= imgHeight {
				continue
			}
			out.Set(x, y, src.At(x, sy))
		}
	}
	return out
}

// quad 二次贝塞尔插值。
func quad(p0, p1, p2, t float64) float64 {
	u := 1 - t
	return u*u*p0 + 2*u*t*p1 + t*t*p2
}

// plot 越界安全的写点 + 简单抗锯齿（写上下左右半透明邻点）。
func plot(img *image.RGBA, x, y int, c color.RGBA) {
	if x < 0 || y < 0 || x >= imgWidth || y >= imgHeight {
		return
	}
	img.Set(x, y, c)
	soft := color.RGBA{R: c.R, G: c.G, B: c.B, A: 96}
	for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		nx, ny := x+d[0], y+d[1]
		if nx >= 0 && ny >= 0 && nx < imgWidth && ny < imgHeight {
			img.Set(nx, ny, soft)
		}
	}
}

// NormalizeLevel 归一干扰强度；未知值回落 normal。
func NormalizeLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case LevelEasy:
		return LevelEasy
	case LevelHard:
		return LevelHard
	default:
		return LevelNormal
	}
}
