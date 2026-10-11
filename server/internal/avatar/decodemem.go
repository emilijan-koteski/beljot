package avatar

import (
	"encoding/binary"
	"errors"

	"github.com/emilijan/beljot/server/internal/user"
)

const (
	maxDecodeBytes = 150 << 20

	decodeSlackPercent = 107
	decodeFixedBytes   = 28 << 20

	float64x4Bytes             = 4 * 8
	resamplerBytesPerSourceRow = user.AvatarSizeLarge * float64x4Bytes
)

var errNoFrameHeader = errors.New("avatar: no JPEG frame header")

// predictDecodeBytes predicts the peak Go memory of one Render, pricing an unreadable header at its format's worst case.
// The constants are calibrated by TestPredictDecodeBytes_CoversEveryMeasuredPeak; recalibrate if the decoders change.
func predictDecodeBytes(data []byte, mime string, width, height int) (int64, error) {
	var decoded int64
	var err error
	switch mime {
	case mimeJPEG:
		decoded, err = jpegDecodeBytes(data, width, height)
	case mimePNG:
		decoded = pngDecodeBytes(data, width, height)
	case mimeWebP:
		decoded = webpDecodeBytes(data, width, height)
	}
	side := int64(min(width, height))
	raw := decoded + side*resamplerBytesPerSourceRow
	return raw*decodeSlackPercent/100 + decodeFixedBytes, err
}

// jpegDecodeBytes sizes image/jpeg's MCU-padded planes, a 4-component file's CMYK copy and progressive coefficient blocks.
// A missing frame header is priced as a progressive 4-component 4:4:4 file, the most expensive shape.
func jpegDecodeBytes(data []byte, width, height int) (int64, error) {
	w, h := int64(width), int64(height)
	f, err := readJPEGFrame(data)
	if err != nil {
		px := w * h
		return 4*px + px + 4*px + 4*4*px, err
	}

	if len(f.comps) == 1 {
		blocks := ((w + 7) / 8) * ((h + 7) / 8)
		total := blocks * 64
		if f.progressive {
			total += blocks * 256
		}
		return total, nil
	}

	h0, v0 := int64(f.comps[0].h), int64(f.comps[0].v)
	mxx := (w + 8*h0 - 1) / (8 * h0)
	myy := (h + 8*v0 - 1) / (8 * v0)
	var planes, blocks int64
	for _, c := range f.comps {
		hi, vi := int64(c.h), int64(c.v)
		planes += (8 * hi * mxx) * (8 * vi * myy)
		blocks += mxx * myy * hi * vi
	}
	total := planes
	if len(f.comps) == 4 {
		total += 4 * w * h
	}
	if f.progressive {
		total += blocks * 256
	}
	return total, nil
}

type jpegComponent struct{ h, v int }

type jpegFrame struct {
	progressive bool
	comps       []jpegComponent
}

// readJPEGFrame walks the marker segments to the first frame header (SOF0,
// SOF1 or SOF2, the three image/jpeg decodes), the same way jpegOrientation
// walks to APP1.
func readJPEGFrame(data []byte) (jpegFrame, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return jpegFrame{}, errNoFrameHeader
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return jpegFrame{}, errNoFrameHeader
		}
		marker := data[i+1]
		switch {
		case marker == 0xFF:
			i++
			continue
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD8):
			i += 2
			continue
		case marker == 0xDA || marker == 0xD9:
			return jpegFrame{}, errNoFrameHeader
		}
		segLen := int(binary.BigEndian.Uint16(data[i+2:]))
		if segLen < 2 || i+2+segLen > len(data) {
			return jpegFrame{}, errNoFrameHeader
		}
		if marker == 0xC0 || marker == 0xC1 || marker == 0xC2 {
			return parseJPEGFrame(data[i+4:i+2+segLen], marker == 0xC2)
		}
		i += 2 + segLen
	}
	return jpegFrame{}, errNoFrameHeader
}

// parseJPEGFrame reads a frame header payload: precision, height, width, the
// component count, then id / sampling factors / table per component.
func parseJPEGFrame(seg []byte, progressive bool) (jpegFrame, error) {
	if len(seg) < 6 {
		return jpegFrame{}, errNoFrameHeader
	}
	n := int(seg[5])
	if n != 1 && n != 3 && n != 4 || len(seg) < 6+3*n {
		return jpegFrame{}, errNoFrameHeader
	}
	f := jpegFrame{progressive: progressive, comps: make([]jpegComponent, n)}
	for k := 0; k < n; k++ {
		hv := seg[6+3*k+1]
		h, v := int(hv>>4), int(hv&0x0F)
		if h < 1 || h > 4 || v < 1 || v > 4 {
			return jpegFrame{}, errNoFrameHeader
		}
		f.comps[k] = jpegComponent{h: h, v: v}
	}
	return f, nil
}

// pngDecodeBytes sizes image/png's result from IHDR and whether a tRNS chunk widens it to NRGBA / NRGBA64.
// An Adam7 image is decoded pass by pass before the merge, so it is priced at twice its size.
func pngDecodeBytes(data []byte, width, height int) int64 {
	const (
		signatureLen   = 8
		chunkHeaderLen = 8
		ihdrFieldsLen  = 13
		colourGrey     = 0
		colourPalette  = 3
	)
	px := int64(width) * int64(height)
	if len(data) < signatureLen+chunkHeaderLen+ihdrFieldsLen || string(data[12:16]) != "IHDR" {
		return 2 * 8 * px
	}
	depth, colour, interlace := data[24], data[25], data[28]
	trns := pngHasTRNS(data)
	wide := depth == 16

	var bpp int64
	switch colour {
	case colourGrey:
		switch {
		case wide && trns:
			bpp = 8
		case wide:
			bpp = 2
		case trns:
			bpp = 4
		default:
			bpp = 1
		}
	case colourPalette:
		bpp = 1
	default:
		bpp = 4
		if wide {
			bpp = 8
		}
	}
	total := bpp * px
	if interlace != 0 {
		total *= 2
	}
	return total
}

// pngHasTRNS reports whether a tRNS chunk appears before the first IDAT.
func pngHasTRNS(data []byte) bool {
	for off := 8; off+8 <= len(data); {
		n := int64(binary.BigEndian.Uint32(data[off:]))
		typ := string(data[off+4 : off+8])
		switch typ {
		case "tRNS":
			return true
		case "IDAT", "IEND":
			return false
		}
		next := int64(off) + 12 + n
		if next > int64(len(data)) {
			return false
		}
		off = int(next)
	}
	return false
}

// webpDecodeBytes sizes x/image/webp's result: a 4:2:0 YCbCr lossy frame plus any ALPH plane, or a lossless NRGBA image.
// Lossless images and compressed alpha add the buffer a colour-indexing transform expands from (see vp8lTransformExtra8).
func webpDecodeBytes(data []byte, width, height int) int64 {
	px := int64(width) * int64(height)
	lossy := px + 2*(int64(width+1)/2)*(int64(height+1)/2)
	var vp8l, alph []byte
	for off := 12; off+8 <= len(data); {
		n := int64(binary.LittleEndian.Uint32(data[off+4:]))
		end := int64(off) + 8 + n
		if end > int64(len(data)) {
			end = int64(len(data))
		}
		switch string(data[off : off+4]) {
		case "VP8L":
			vp8l = data[off+8 : end]
		case "ALPH":
			alph = data[off+8 : end]
		}
		next := int64(off) + 8 + n + n%2
		if next > int64(len(data)) {
			break
		}
		off = int(next)
	}
	switch {
	case vp8l != nil:
		const vp8lHeaderLen = 5
		var stream []byte
		if len(vp8l) >= vp8lHeaderLen {
			stream = vp8l[vp8lHeaderLen:]
		}
		return 4*px + px*vp8lTransformExtra8(stream)/8
	case alph != nil:
		uncompressed := len(alph) == 0 || alph[0]&0x03 == 0
		if uncompressed {
			return lossy + px
		}
		return lossy + px + 4*px + px*vp8lTransformExtra8(alph[1:])/8
	default:
		return lossy
	}
}

// vp8lTransformExtra8 returns, in eighths of a byte per pixel, the packed index image a colour-indexing transform expands from.
// Transforms are read until one carries data it cannot skip, after which the worst case (2 B/px) is assumed.
func vp8lTransformExtra8(stream []byte) int64 {
	const (
		worst                   = 16
		transformSubtractGreen  = 2
		transformColourIndexing = 3
	)
	br := lsbBits{data: stream}
	for {
		present, ok := br.read(1)
		if !ok {
			return worst
		}
		if present == 0 {
			return 0
		}
		kind, ok := br.read(2)
		if !ok {
			return worst
		}
		switch kind {
		case transformSubtractGreen:
			continue
		case transformColourIndexing:
			size, ok := br.read(8)
			if !ok {
				return worst
			}
			switch colours := size + 1; {
			case colours <= 2:
				return 4
			case colours <= 4:
				return 8
			case colours <= 16:
				return 16
			default:
				return 0
			}
		default:
			return worst
		}
	}
}

// lsbBits reads a VP8L bitstream, least significant bit first.
type lsbBits struct {
	data []byte
	pos  uint
}

func (b *lsbBits) read(n uint) (uint32, bool) {
	var v uint32
	for i := uint(0); i < n; i++ {
		byteIdx := (b.pos + i) / 8
		if byteIdx >= uint(len(b.data)) {
			return 0, false
		}
		bit := (b.data[byteIdx] >> ((b.pos + i) % 8)) & 1
		v |= uint32(bit) << i
	}
	b.pos += n
	return v, true
}
