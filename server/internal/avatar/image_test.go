package avatar

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/gen2brain/webp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	xwebp "golang.org/x/image/webp"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// --- fixtures -------------------------------------------------------------

var (
	red   = color.RGBA{R: 230, G: 20, B: 20, A: 255}
	green = color.RGBA{R: 20, G: 200, B: 20, A: 255}
	blue  = color.RGBA{R: 20, G: 20, B: 230, A: 255}
)

// gradientImage is a w×h image with a smooth gradient, a stand-in for a photo.
func gradientImage(w, h int) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.SetRGBA(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 128, A: 255})
		}
	}
	return m
}

// paintedImage is a w×h image coloured by paint(x, y).
func paintedImage(w, h int, paint func(x, y int) color.RGBA) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.SetRGBA(x, y, paint(x, y))
		}
	}
	return m
}

func jpegBytes(t *testing.T, m image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, m, &jpeg.Options{Quality: 90}))
	return buf.Bytes()
}

func pngBytes(t *testing.T, m image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, m))
	return buf.Bytes()
}

func webpBytes(t *testing.T, m image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, webp.Encode(&buf, m, webp.Options{Quality: 90}))
	return buf.Bytes()
}

func animatedWebPBytes(t *testing.T) []byte {
	t.Helper()
	anim := &webp.WEBP{
		Image: []image.Image{gradientImage(200, 200), paintedImage(200, 200, func(int, int) color.RGBA { return red })},
		Delay: []int{100, 100},
	}
	var buf bytes.Buffer
	require.NoError(t, webp.EncodeAll(&buf, anim, webp.Options{Quality: 80}))
	return buf.Bytes()
}

func gifBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, gif.Encode(&buf, gradientImage(200, 200), nil))
	return buf.Bytes()
}

// pngHeaderOnly is a PNG signature plus a valid IHDR claiming w×h and nothing
// else: DecodeConfig accepts it, a full decode cannot. It proves the size
// limits are enforced from the header, before any pixel is decoded.
func pngHeaderOnly(w, h int) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h))
	ihdr[8] = 8 // bit depth
	ihdr[9] = 2 // truecolour
	chunk := append([]byte("IHDR"), ihdr...)
	out := []byte("\x89PNG\r\n\x1a\n")
	out = binary.BigEndian.AppendUint32(out, uint32(len(ihdr)))
	out = append(out, chunk...)
	out = binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(chunk))
	return out
}

// withEXIFOrientation inserts an APP1 EXIF segment carrying only the
// orientation tag right after the JPEG's SOI marker.
func withEXIFOrientation(data []byte, orientation int, bo binary.ByteOrder) []byte {
	tiff := make([]byte, 8+2+12+4)
	if bo == binary.LittleEndian {
		copy(tiff, "II")
	} else {
		copy(tiff, "MM")
	}
	bo.PutUint16(tiff[2:], 42)
	bo.PutUint32(tiff[4:], 8) // IFD0 right after the header
	bo.PutUint16(tiff[8:], 1) // one entry
	entry := tiff[10:22]
	bo.PutUint16(entry[0:], 0x0112) // Orientation
	bo.PutUint16(entry[2:], 3)      // SHORT
	bo.PutUint32(entry[4:], 1)      // count
	bo.PutUint16(entry[8:], uint16(orientation))

	payload := append([]byte("Exif\x00\x00"), tiff...)
	segLen := len(payload) + 2
	out := append([]byte{}, data[:2]...)
	out = append(out, 0xFF, 0xE1, byte(segLen>>8), byte(segLen))
	out = append(out, payload...)
	return append(out, data[2:]...)
}

// --- assertions -----------------------------------------------------------

// decodeSquareWebP asserts data is a WebP of exactly size×size and returns it.
func decodeSquareWebP(t *testing.T, data []byte, size int) image.Image {
	t.Helper()
	require.Equal(t, "RIFF", string(data[0:4]))
	require.Equal(t, "WEBP", string(data[8:12]))
	m, err := xwebp.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, image.Rect(0, 0, size, size), m.Bounds())
	return m
}

// assertNoMetadata walks the RIFF chunks and fails on any metadata chunk.
func assertNoMetadata(t *testing.T, data []byte) {
	t.Helper()
	for off := 12; off+8 <= len(data); {
		fourCC := string(data[off : off+4])
		assert.NotContains(t, []string{"EXIF", "XMP ", "ICCP"}, fourCC, "output carries a %s chunk", fourCC)
		n := int(binary.LittleEndian.Uint32(data[off+4:]))
		off += 8 + n + n%2
	}
}

func assertColorNear(t *testing.T, m image.Image, x, y int, want color.RGBA, msg string) {
	t.Helper()
	r, g, b, _ := m.At(x, y).RGBA()
	got := color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8)}
	near := func(a, b uint8) bool { return int(a)-int(b) < 60 && int(b)-int(a) < 60 }
	assert.True(t, near(got.R, want.R) && near(got.G, want.G) && near(got.B, want.B),
		"%s: pixel (%d,%d) = %v, want about %v", msg, x, y, got, want)
}

// --- Inspect --------------------------------------------------------------

func TestInspect_AcceptsStillJPEGPNGWebP(t *testing.T) {
	src := gradientImage(300, 200)
	for name, data := range map[string][]byte{
		"jpeg": jpegBytes(t, src),
		"png":  pngBytes(t, src),
		"webp": webpBytes(t, src),
	} {
		t.Run(name, func(t *testing.T) {
			s, err := Inspect(data)
			require.NoError(t, err)
			assert.Equal(t, 300, s.width)
			assert.Equal(t, 200, s.height)
		})
	}
}

func TestInspect_RejectsUnsupportedTypes(t *testing.T) {
	corruptJPEG := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte("not a jpeg "), 50)...)
	cases := map[string][]byte{
		"gif":           gifBytes(t),
		"svg":           []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" width="200" height="200"><rect width="200" height="200"/></svg>`),
		"pdf":           []byte("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\ntrailer << /Root 1 0 R >>\n%%EOF"),
		"text":          []byte("just some text pretending to be an image"),
		"animated webp": animatedWebPBytes(t),
		"corrupt jpeg":  corruptJPEG,
		"empty":         {},
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Inspect(data)
			assert.ErrorIs(t, err, apperr.ErrAvatarUnsupportedType)
		})
	}
}

func TestInspect_SizeAndDimensionLimits(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"64x64 is too small", pngBytes(t, gradientImage(64, 64)), apperr.ErrAvatarTooSmall},
		{"shorter side 127 is too small", pngBytes(t, gradientImage(500, 127)), apperr.ErrAvatarTooSmall},
		{"128x128 is the minimum", pngBytes(t, gradientImage(128, 128)), nil},
		{"5000 px on a side", pngHeaderOnly(5000, 300), apperr.ErrAvatarDimensionsTooLarge},
		{"4097 px on a side", pngHeaderOnly(200, 4097), apperr.ErrAvatarDimensionsTooLarge},
		{"4000x4001 is over 16 MP", pngHeaderOnly(4000, 4001), apperr.ErrAvatarDimensionsTooLarge},
		{"4096x3900 is under 16 MP", pngHeaderOnly(4096, 3900), nil},
		{"one byte over 2 MiB", append(pngBytes(t, gradientImage(200, 200)), make([]byte, MaxFileBytes)...), apperr.ErrAvatarTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Inspect(tc.data)
			if tc.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

// A header that passes the cheap checks but whose pixels cannot be decoded is
// rejected by Render as unsupported, not as a server error.
func TestRender_UndecodablePixelsAreUnsupported(t *testing.T) {
	s, err := Inspect(pngHeaderOnly(300, 300))
	require.NoError(t, err)
	_, _, err = s.Render()
	assert.ErrorIs(t, err, apperr.ErrAvatarUnsupportedType)
}

// --- Process --------------------------------------------------------------

func TestProcess_ProducesTwoSquareWebPsWithoutMetadata(t *testing.T) {
	src := withEXIFOrientation(jpegBytes(t, gradientImage(1600, 1200)), 1, binary.BigEndian)
	large, small, err := Process(bytes.NewReader(src))
	require.NoError(t, err)

	decodeSquareWebP(t, large, 256)
	decodeSquareWebP(t, small, 128)
	assertNoMetadata(t, large)
	assertNoMetadata(t, small)
	assert.Less(t, len(small), len(large), "the 128 image is the lighter one")
}

func TestProcess_AcceptsEveryInputType(t *testing.T) {
	src := gradientImage(400, 300)
	for name, data := range map[string][]byte{
		"jpeg": jpegBytes(t, src),
		"png":  pngBytes(t, src),
		"webp": webpBytes(t, src),
	} {
		t.Run(name, func(t *testing.T) {
			large, small, err := Process(bytes.NewReader(data))
			require.NoError(t, err)
			decodeSquareWebP(t, large, 256)
			decodeSquareWebP(t, small, 128)
		})
	}
}

// 1000×600 with 200 px green bands left and right: the centre 600×600 crop is
// all blue, so no green may reach any corner of either output.
func TestProcess_CentreCropsNonSquare(t *testing.T) {
	src := paintedImage(1000, 600, func(x, _ int) color.RGBA {
		if x < 200 || x >= 800 {
			return green
		}
		return blue
	})
	large, small, err := Process(bytes.NewReader(pngBytes(t, src)))
	require.NoError(t, err)

	for _, out := range []struct {
		data []byte
		size int
	}{{large, 256}, {small, 128}} {
		m := decodeSquareWebP(t, out.data, out.size)
		last := out.size - 3
		for _, p := range [][2]int{{2, 2}, {last, 2}, {2, last}, {last, last}, {out.size / 2, out.size / 2}} {
			assertColorNear(t, m, p[0], p[1], blue, "centre crop")
		}
	}
}

// A phone portrait is stored landscape with EXIF orientation 6 ("rotate 90°
// clockwise to display"). Stored top half red, bottom half blue: displayed,
// red is on the right and blue on the left, in both derivatives.
func TestProcess_AppliesEXIFOrientation(t *testing.T) {
	stored := paintedImage(400, 300, func(_, y int) color.RGBA {
		if y < 150 {
			return red
		}
		return blue
	})
	plain := jpegBytes(t, stored)

	for _, bo := range []binary.ByteOrder{binary.BigEndian, binary.LittleEndian} {
		large, small, err := Process(bytes.NewReader(withEXIFOrientation(plain, 6, bo)))
		require.NoError(t, err)
		for _, out := range []struct {
			data []byte
			size int
		}{{large, 256}, {small, 128}} {
			m := decodeSquareWebP(t, out.data, out.size)
			mid := out.size / 2
			assertColorNear(t, m, out.size/8, mid, blue, "left after rotation")
			assertColorNear(t, m, out.size-out.size/8, mid, red, "right after rotation")
		}
	}

	// Without the tag the stored layout is kept: red on top.
	large, _, err := Process(bytes.NewReader(plain))
	require.NoError(t, err)
	m := decodeSquareWebP(t, large, 256)
	assertColorNear(t, m, 128, 32, red, "top without orientation")
	assertColorNear(t, m, 128, 224, blue, "bottom without orientation")
}

// --- EXIF and orientation helpers ----------------------------------------

func TestJPEGOrientation(t *testing.T) {
	plain := jpegBytes(t, gradientImage(16, 16))
	assert.Equal(t, 1, jpegOrientation(plain), "no EXIF")
	assert.Equal(t, 1, jpegOrientation([]byte("not a jpeg")), "not a JPEG")
	for o := 1; o <= 8; o++ {
		assert.Equal(t, o, jpegOrientation(withEXIFOrientation(plain, o, binary.BigEndian)), "MM, orientation %d", o)
		assert.Equal(t, o, jpegOrientation(withEXIFOrientation(plain, o, binary.LittleEndian)), "II, orientation %d", o)
	}
	assert.Equal(t, 1, jpegOrientation(withEXIFOrientation(plain, 9, binary.BigEndian)), "out-of-range value")
	truncated := withEXIFOrientation(plain, 6, binary.BigEndian)[:20]
	assert.Equal(t, 1, jpegOrientation(truncated), "truncated segment")
}

// Where stored pixels A (0,0) and B (1,0) of a 3×3 image land for each EXIF
// orientation, per the EXIF definitions.
func TestOrient(t *testing.T) {
	a := color.RGBA{R: 255, A: 255}
	b := color.RGBA{G: 255, A: 255}
	stored := image.NewRGBA(image.Rect(0, 0, 3, 3))
	stored.SetRGBA(0, 0, a)
	stored.SetRGBA(1, 0, b)

	cases := map[int][2]image.Point{
		1: {{0, 0}, {1, 0}},
		2: {{2, 0}, {1, 0}}, // mirror horizontal
		3: {{2, 2}, {1, 2}}, // rotate 180
		4: {{0, 2}, {1, 2}}, // mirror vertical
		5: {{0, 0}, {0, 1}}, // transpose
		6: {{2, 0}, {2, 1}}, // rotate 90 CW
		7: {{2, 2}, {2, 1}}, // transverse
		8: {{0, 2}, {0, 1}}, // rotate 90 CCW
	}
	for o, want := range cases {
		out := orient(stored, o)
		assert.Equal(t, a, out.RGBAAt(want[0].X, want[0].Y), "orientation %d: A", o)
		assert.Equal(t, b, out.RGBAAt(want[1].X, want[1].Y), "orientation %d: B", o)
	}
}

// Transparency survives: a PNG whose left half is fully transparent comes out
// as a WebP whose left half still is.
func TestProcess_KeepsTransparency(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 300, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 300; x++ {
			if x >= 150 {
				src.SetNRGBA(x, y, color.NRGBA{R: 200, G: 30, B: 30, A: 255})
			}
		}
	}
	large, small, err := Process(bytes.NewReader(pngBytes(t, src)))
	require.NoError(t, err)
	for _, out := range []struct {
		data []byte
		size int
	}{{large, 256}, {small, 128}} {
		m := decodeSquareWebP(t, out.data, out.size)
		_, _, _, clear := m.At(out.size/8, out.size/2).RGBA()
		_, _, _, opaque := m.At(out.size-out.size/8, out.size/2).RGBA()
		assert.Less(t, clear>>8, uint32(16), "the transparent half stays transparent")
		assert.Greater(t, opaque>>8, uint32(240), "the opaque half stays opaque")
	}
}

// A greyscale JPEG (one component, decoded to image.Gray) goes through the
// same pipeline and stays grey.
func TestProcess_AcceptsGreyscaleJPEG(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			src.SetGray(x, y, color.Gray{Y: uint8(x * 255 / 400)})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, src, &jpeg.Options{Quality: 90}))
	f, err := readJPEGFrame(buf.Bytes())
	require.NoError(t, err)
	require.Len(t, f.comps, 1, "the fixture really is single-component")

	large, small, err := Process(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	m := decodeSquareWebP(t, large, 256)
	decodeSquareWebP(t, small, 128)
	r, g, b, _ := m.At(128, 128).RGBA()
	near := func(a, b uint32) bool { return a>>8 < b>>8+12 && b>>8 < a>>8+12 }
	assert.True(t, near(r, g) && near(g, b), "grey in, grey out: %d %d %d", r>>8, g>>8, b>>8)
}
