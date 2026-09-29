package avatar

import (
	"encoding/binary"
	"hash/crc32"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/emilijan/beljot/server/internal/apperr"
)

// --- header-only fixtures: enough for DecodeConfig and the predictor, never
// a decodable image ---

// jpegHeaderOnly is SOI, an Adobe APP14 marker (so a 4-component file reads
// as CMYK), one frame header (SOF0, or SOF2 when progressive) with the given
// sampling factors, and a scan header (DecodeConfig reads up to it), then EOI.
func jpegHeaderOnly(w, h int, progressive bool, comps ...jpegComponent) []byte {
	out := []byte{0xFF, 0xD8}
	adobe := []byte("Adobe\x00\x64\x00\x00\x00\x00\x00") // transform 0: unknown (CMYK)
	out = append(out, 0xFF, 0xEE, byte((len(adobe)+2)>>8), byte(len(adobe)+2))
	out = append(out, adobe...)

	sof := []byte{8, byte(h >> 8), byte(h), byte(w >> 8), byte(w), byte(len(comps))}
	for i, c := range comps {
		sof = append(sof, byte(i+1), byte(c.h<<4|c.v), 0)
	}
	marker := byte(0xC0)
	if progressive {
		marker = 0xC2
	}
	out = append(out, 0xFF, marker, byte((len(sof)+2)>>8), byte(len(sof)+2))
	out = append(out, sof...)

	sos := []byte{byte(len(comps))}
	for i := range comps {
		sos = append(sos, byte(i+1), 0)
	}
	sos = append(sos, 0, 63, 0)
	out = append(out, 0xFF, 0xDA, byte((len(sos)+2)>>8), byte(len(sos)+2))
	out = append(out, sos...)
	return append(out, 0xFF, 0xD9)
}

var (
	ycc420  = []jpegComponent{{2, 2}, {1, 1}, {1, 1}}
	ycc444  = []jpegComponent{{1, 1}, {1, 1}, {1, 1}}
	cmyk444 = []jpegComponent{{1, 1}, {1, 1}, {1, 1}, {1, 1}}
	grey    = []jpegComponent{{1, 1}}
)

func pngChunk(typ string, body []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(body)))
	chunk := append([]byte(typ), body...)
	out = append(out, chunk...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(chunk))
}

// pngIHDROnly is a signature, an IHDR with the given format, an optional tRNS
// and an empty IDAT: DecodeConfig accepts it, a decode cannot.
func pngIHDROnly(w, h int, depth, colour, interlace byte, trns bool) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h))
	ihdr[8], ihdr[9], ihdr[12] = depth, colour, interlace
	out := append([]byte("\x89PNG\r\n\x1a\n"), pngChunk("IHDR", ihdr)...)
	if colour == 3 {
		out = append(out, pngChunk("PLTE", []byte{0, 0, 0, 255, 255, 255})...)
	}
	if trns {
		out = append(out, pngChunk("tRNS", []byte{0, 0})...)
	}
	return append(out, pngChunk("IDAT", nil)...)
}

func riff(chunks ...[]byte) []byte {
	var body []byte
	for _, c := range chunks {
		body = append(body, c...)
	}
	out := []byte("RIFF")
	out = binary.LittleEndian.AppendUint32(out, uint32(4+len(body)))
	out = append(out, "WEBP"...)
	return append(out, body...)
}

func webpChunk(fourCC string, body []byte) []byte {
	out := append([]byte(fourCC), binary.LittleEndian.AppendUint32(nil, uint32(len(body)))...)
	out = append(out, body...)
	if len(body)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

// vp8HeaderOnly is a lossy WebP with just a key frame header.
func vp8HeaderOnly(w, h int) []byte {
	frame := []byte{0x00, 0x00, 0x00, 0x9d, 0x01, 0x2a}
	frame = binary.LittleEndian.AppendUint16(frame, uint16(w))
	frame = binary.LittleEndian.AppendUint16(frame, uint16(h))
	return riff(webpChunk("VP8 ", frame))
}

// vp8lHeaderOnly is a lossless WebP whose bitstream holds the header word and
// then the given transform bits (LSB first, as VP8L writes them).
func vp8lHeaderOnly(w, h int, transformBits ...uint32) []byte {
	word := uint32(w-1) | uint32(h-1)<<14
	body := []byte{0x2f}
	body = binary.LittleEndian.AppendUint32(body, word)
	body = append(body, packLSB(transformBits)...)
	return riff(webpChunk("VP8L", body))
}

// packLSB packs (value, width) pairs given as alternating entries.
func packLSB(pairs []uint32) []byte {
	var out []byte
	pos := 0
	for i := 0; i+1 < len(pairs); i += 2 {
		v, n := pairs[i], int(pairs[i+1])
		for b := 0; b < n; b++ {
			if pos/8 >= len(out) {
				out = append(out, 0)
			}
			if v>>b&1 == 1 {
				out[pos/8] |= 1 << (pos % 8)
			}
			pos++
		}
	}
	return append(out, 0, 0)
}

func mib(v int64) float64 { return float64(v) / (1 << 20) }

// --- calibration ---

// The predictor must never undershoot a measured peak. These are the peaks of
// a full Inspect + Render measured on real 16 MP files (review iteration 2,
// Go 1.26), each rebuilt here as a header of the same shape.
func TestPredictDecodeBytes_CoversEveryMeasuredPeak(t *testing.T) {
	cases := []struct {
		name     string
		data     []byte
		mime     string
		w, h     int
		measured float64 // MiB
		accepted bool
	}{
		{"baseline 4:2:0 JPEG", jpegHeaderOnly(4096, 3906, false, ycc420...), mimeJPEG, 4096, 3906, 74, true},
		{"8-bit RGBA PNG", pngIHDROnly(4000, 4000, 8, 6, 0, false), mimePNG, 4000, 4000, 110, true},
		{"lossy WebP", vp8HeaderOnly(4000, 4000), mimeWebP, 4000, 4000, 69, true},
		{"baseline CMYK JPEG", jpegHeaderOnly(4096, 3906, false, cmyk444...), mimeJPEG, 4096, 3906, 174, false},
		{"16-bit RGBA PNG", pngIHDROnly(4000, 4000, 16, 6, 0, false), mimePNG, 4000, 4000, 182, false},
		{"progressive 4:4:4 JPEG", jpegHeaderOnly(4096, 3906, true, ycc444...), mimeJPEG, 4096, 3906, 286, false},
		{"progressive CMYK JPEG", jpegHeaderOnly(4096, 3906, true, cmyk444...), mimeJPEG, 4096, 3906, 431, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := predictDecodeBytes(tc.data, tc.mime, tc.w, tc.h)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, mib(got), tc.measured, "prediction below the measured peak")
			assert.Equal(t, tc.accepted, got <= maxDecodeBytes, "predicted %.0f MiB", mib(got))
		})
	}
}

// --- Inspect ---

func TestInspect_HeavyEncodingsAreRejectedBeforeDecode(t *testing.T) {
	cases := map[string][]byte{
		"progressive 4:4:4 JPEG":         jpegHeaderOnly(4096, 3906, true, ycc444...),
		"progressive 4:2:0 JPEG":         jpegHeaderOnly(4096, 3906, true, ycc420...),
		"baseline CMYK JPEG":             jpegHeaderOnly(4096, 3906, false, cmyk444...),
		"progressive CMYK JPEG":          jpegHeaderOnly(4096, 3906, true, cmyk444...),
		"16-bit RGBA PNG":                pngIHDROnly(4000, 4000, 16, 6, 0, false),
		"interlaced 8-bit RGBA PNG":      pngIHDROnly(4000, 4000, 8, 6, 1, false),
		"16-bit grey PNG with tRNS":      pngIHDROnly(4000, 4000, 16, 0, 0, true),
		"lossless WebP, 16-colour table": vp8lHeaderOnly(4000, 4000, 1, 1, 3, 2, 15, 8),
		"lossless WebP, predictor first": vp8lHeaderOnly(4000, 4000, 1, 1, 0, 2),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Inspect(data)
			assert.ErrorIs(t, err, apperr.ErrAvatarDimensionsTooLarge)
		})
	}
}

func TestInspect_LightEncodingsKeepTheFull16MP(t *testing.T) {
	cases := map[string][]byte{
		"baseline 4:2:0 JPEG":           jpegHeaderOnly(4096, 3906, false, ycc420...),
		"baseline 4:4:4 JPEG":           jpegHeaderOnly(4096, 3906, false, ycc444...),
		"greyscale JPEG":                jpegHeaderOnly(4096, 3906, false, grey...),
		"8-bit RGBA PNG":                pngIHDROnly(4000, 4000, 8, 6, 0, false),
		"8-bit RGB PNG":                 pngIHDROnly(4000, 4000, 8, 2, 0, false),
		"8-bit grey PNG with tRNS":      pngIHDROnly(4000, 4000, 8, 0, 0, true),
		"16-bit grey PNG":               pngIHDROnly(4000, 4000, 16, 0, 0, false),
		"palette PNG":                   pngIHDROnly(4000, 4000, 8, 3, 0, false),
		"lossy WebP":                    vp8HeaderOnly(4000, 4000),
		"lossless WebP, no transforms":  vp8lHeaderOnly(4000, 4000, 0, 1),
		"lossless WebP, 2-colour table": vp8lHeaderOnly(4000, 4000, 1, 1, 3, 2, 1, 8),
		"lossless WebP, 200 colours":    vp8lHeaderOnly(4000, 4000, 1, 1, 2, 2, 1, 1, 3, 2, 199, 8),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Inspect(data)
			assert.NoError(t, err)
		})
	}
}

// Smaller heavy sources still fit: the cap is on memory, not on the encoding.
func TestInspect_HeavyEncodingsFitAtSmallerSizes(t *testing.T) {
	for name, data := range map[string][]byte{
		"progressive 4:4:4 JPEG, 2000x2000": jpegHeaderOnly(2000, 2000, true, ycc444...),
		"baseline CMYK JPEG, 3000x3000":     jpegHeaderOnly(3000, 3000, false, cmyk444...),
		"16-bit RGBA PNG, 3000x3000":        pngIHDROnly(3000, 3000, 16, 6, 0, false),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Inspect(data)
			assert.NoError(t, err)
		})
	}
}

// A JPEG whose frame header cannot be read is priced at the worst shape.
func TestPredictDecodeBytes_UnreadableJPEGFrameIsWorstCase(t *testing.T) {
	got, err := predictDecodeBytes([]byte{0xFF, 0xD8, 0xFF, 0xD9}, mimeJPEG, 2000, 2000)
	assert.Error(t, err)
	worst, err := predictDecodeBytes(jpegHeaderOnly(2000, 2000, true, cmyk444...), mimeJPEG, 2000, 2000)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, got, worst)
}

func TestReadJPEGFrame(t *testing.T) {
	f, err := readJPEGFrame(jpegHeaderOnly(640, 480, true, ycc420...))
	require.NoError(t, err)
	assert.True(t, f.progressive)
	assert.Equal(t, ycc420, f.comps)

	f, err = readJPEGFrame(jpegBytes(t, gradientImage(160, 120)))
	require.NoError(t, err, "a real encoder's file")
	assert.False(t, f.progressive)
	assert.Len(t, f.comps, 3)
}

func TestVP8LTransformExtra8(t *testing.T) {
	cases := []struct {
		name string
		bits []uint32
		want int64
	}{
		{"no transforms", []uint32{0, 1}, 0},
		{"subtract green only", []uint32{1, 1, 2, 2, 0, 1}, 0},
		{"2 colours", []uint32{1, 1, 3, 2, 1, 8}, 4},
		{"4 colours", []uint32{1, 1, 3, 2, 3, 8}, 8},
		{"16 colours", []uint32{1, 1, 3, 2, 15, 8}, 16},
		{"17 colours, expanded in place", []uint32{1, 1, 3, 2, 16, 8}, 0},
		{"predictor first", []uint32{1, 1, 0, 2}, 16},
		{"colour transform first", []uint32{1, 1, 1, 2}, 16},
		{"truncated", nil, 16},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stream []byte
			if tc.bits != nil {
				stream = packLSB(tc.bits)
			}
			assert.Equal(t, tc.want, vp8lTransformExtra8(stream))
		})
	}
}
