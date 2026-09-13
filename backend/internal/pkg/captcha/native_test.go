package captcha

import (
	"bytes"
	"image/png"
	"testing"
)

// TestNativeGenerate 覆盖 doc91 §C1 验证要求：生成 100 张不 panic、像素确实被扰动。
func TestNativeGenerate(t *testing.T) {
	gen := NewNativeGenerator()
	levels := []string{LevelEasy, LevelNormal, LevelHard}
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		level := levels[i%len(levels)]
		answer, pngBytes, err := gen.Generate(level)
		if err != nil {
			t.Fatalf("generate(%s) failed: %v", level, err)
		}
		if len(answer) < 4 {
			t.Fatalf("answer too short: %q", answer)
		}
		for _, ch := range answer {
			if !bytes.ContainsRune([]byte(charset), ch) {
				t.Fatalf("answer contains char outside charset: %q", answer)
			}
		}
		seen[answer] = true

		img, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			t.Fatalf("decode png failed: %v", err)
		}
		if img.Bounds().Dx() != imgWidth || img.Bounds().Dy() != imgHeight {
			t.Fatalf("unexpected image size: %v", img.Bounds())
		}
		// 像素扰动断言：统计与纯背景色不同的像素占比，必须超过 2%（否则等于没画东西）。
		changed := 0
		b := img.Bounds()
		total := b.Dx() * b.Dy()
		first := img.At(b.Min.X, b.Min.Y)
		for x := b.Min.X; x < b.Max.X; x++ {
			for y := b.Min.Y; y < b.Max.Y; y++ {
				if img.At(x, y) != first {
					changed++
				}
			}
		}
		if changed*100/total < 2 {
			t.Fatalf("image looks blank for level %s: changed=%d/%d", level, changed, total)
		}
	}
	// 100 张答案应有足够随机性（避免常量答案这种致命缺陷）。
	if len(seen) < 90 {
		t.Fatalf("answers not random enough: only %d distinct", len(seen))
	}
}

func TestNormalizeLevel(t *testing.T) {
	cases := map[string]string{
		"":       LevelNormal,
		"EASY":   LevelEasy,
		" Hard ": LevelHard,
		"weird":  LevelNormal,
	}
	for in, want := range cases {
		if got := NormalizeLevel(in); got != want {
			t.Fatalf("NormalizeLevel(%q)=%q want %q", in, got, want)
		}
	}
}
