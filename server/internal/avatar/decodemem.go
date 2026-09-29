package avatar

import (
	"encoding/binary"
	"errors"
)

// Decode-memory prediction. The pixel caps (4096 px a side, 16 MP) bound the
// SIZE of a source, not what decoding it costs: image/jpeg keeps every DCT
// coefficient of a progressive scan (256 B per 8x8 block per component), a
// 4-component JPEG is decoded twice over (planes, then a CMYK copy), a 16-bit
// PNG is 8 B per pixel, and all of that fits in a file of a few hundred KB. So
// Inspect reads the encoding from the header and predicts the peak Go memory
// the one decode will need, and refuses above maxDecodeBytes before the decode
// slot is taken.
//
// The model is the decoder's own allocations (the decoded image and any
// working copies, sized exactly as the standard decoders size them) plus the
// resampler's buffer, times a slack factor, plus a fixed allowance. The two
// constants were calibrated against the measured peak Go runtime memory
// (resident: total minus released) of a full Inspect + Render on 16 MP
// fixtures of every encoding, so each prediction is at or above what was
// measured. Measured 2026-09-29 with Go 1.26 on generated solid-colour images;
// rerun the calibration if the decoders or x/image change:
//
//	source (4096x3906 JPEG, 4000x4000 PNG/WebP)   measured   predicted
//	baseline 4:2:0 JPEG                             74 MiB      85 MiB
//	lossy WebP                                      69 MiB      86 MiB
//	8-bit RGBA PNG                                 110 MiB     127 MiB
//	lossless WebP, one colour                      114 MiB     135 MiB
//	progressive greyscale JPEG                     122 MiB     142 MiB
//	lossy WebP with compressed alpha               149 MiB     176 MiB   rejected
//	baseline CMYK JPEG                             174 MiB     191 MiB   rejected
//	16-bit RGBA PNG                                182 MiB     192 MiB   rejected
//	progressive 4:4:4 JPEG                         286 MiB     306 MiB   rejected
//	progressive CMYK JPEG                          431 MiB     453 MiB   rejected
const (
	// maxDecodeBytes is the most one decode may be predicted to need: with the
	// single decode slot it keeps the backend well inside its 256 MiB limit.
	maxDecodeBytes = 150 << 20

	// decodeSlackPercent and decodeFixedBytes turn the decoder's allocations
	// into a peak: garbage the GC has not yet collected (the read buffers, the
	// multipart copy, a progressive decoder's scratch), and the encoder.
	decodeSlackPercent = 107
	decodeFixedBytes   = 28 << 20

	// resamplerBytesPerSourceRow: x/image/draw's two-pass CatmullRom holds a
	// [4]float64 (32 B) per destination column per source row, and the first
	// pass is 256 columns wide over the whole crop height.
	resamplerBytesPerSourceRow = 256 * 32
)

var errNoFrameHeader = errors.New("avatar: no JPEG frame header")

// predictDecodeBytes predicts the peak Go memory of one Render of this
// source. An encoding the parser cannot read is predicted at the worst case
// its format allows, never waved through.
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

// jpegDecodeBytes sizes image/jpeg's allocations from the frame header: the
// component planes at their MCU-padded sizes (a Gray or YCbCr image), a
// 4-component file's black plane plus the CMYK image it is converted into,
// and, for a progressive scan, one 256 B coefficient block per 8x8 block of
// every component. A missing frame header is priced as a progressive
// 4-component 4:4:4 file, the most expensive shape.
func jpegDecodeBytes(data []byte, width, height int) (int64, error) {
	w, h := int64(width), int64(height)
	f, err := readJPEGFrame(data)
	if err != nil {
		px := w * h
		return 4*px + px + 4*px + 4*4*px, err
	}

	if len(f.comps) == 1 {
		// image/jpeg ignores a single component's sampling factors (the data is
		// non-interleaved by definition), so it is plain 8x8 blocks.
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
		// The black plane is one of `planes`; the conversion adds a full-size
		// 4 B/px CMYK image on top.
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

// pngDecodeBytes sizes image/png's result from IHDR (bit depth, colour type,
// interlace) and whether a tRNS chunk precedes the image data, which turns a
// grey or truecolour image into NRGBA / NRGBA64. An Adam7 image is also
// decoded pass by pass into separate images before the merge, so it is priced
// at twice its size.
func pngDecodeBytes(data []byte, width, height int) int64 {
	px := int64(width) * int64(height)
	// Signature (8) + IHDR length (4) + type (4) + 13 bytes of fields.
	if len(data) < 8+8+13 || string(data[12:16]) != "IHDR" {
		return 2 * 8 * px
	}
	depth, colour, interlace := data[24], data[25], data[28]
	trns := pngHasTRNS(data)
	wide := depth == 16

	var bpp int64
	switch colour {
	case 0: // greyscale
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
	case 3: // palette
		bpp = 1
	default: // truecolour, grey + alpha, truecolour + alpha
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

// webpDecodeBytes sizes x/image/webp's result. A lossy frame is a 4:2:0 YCbCr
// image (1.5 B/px); an ALPH chunk adds a full-size alpha plane, and when that
// alpha is compressed it is itself decoded as a lossless image first. A
// lossless image is NRGBA (4 B/px), plus the packed buffer a colour-indexing
// transform expands from (see vp8lTransformExtra8).
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
		// Skip the signature byte and the 32-bit size / alpha / version word.
		var stream []byte
		if len(vp8l) >= 5 {
			stream = vp8l[5:]
		}
		return 4*px + px*vp8lTransformExtra8(stream)/8
	case alph != nil:
		if len(alph) == 0 || alph[0]&0x03 == 0 {
			return lossy + px // uncompressed alpha: just the plane
		}
		// A compressed alpha stream starts straight at the transforms.
		return lossy + px + 4*px + px*vp8lTransformExtra8(alph[1:])/8
	default:
		return lossy
	}
}

// vp8lTransformExtra8 returns, in eighths of a byte per pixel, the extra
// buffer x/image's lossless decoder allocates while undoing a colour-indexing
// transform: the packed index image it expands from, 4 B per packed pixel with
// 2, 4 or 8 pixels per packed pixel (so 2, 1 or 0.5 B/px). A palette of more
// than 16 colours is expanded in place (0). The transform list is read up to
// the first transform that carries data it cannot skip (predictor or colour
// transform); if a colour-indexing transform could still follow, the worst
// case (2 B/px) is assumed.
func vp8lTransformExtra8(stream []byte) int64 {
	const worst = 16
	br := lsbBits{data: stream}
	for {
		present, ok := br.read(1)
		if !ok {
			return worst
		}
		if present == 0 {
			return 0 // no (further) transforms: no colour indexing at all
		}
		kind, ok := br.read(2)
		if !ok {
			return worst
		}
		switch kind {
		case 2: // subtract green: no data, keep reading
			continue
		case 3: // colour indexing: the table size decides the packing
			size, ok := br.read(8)
			if !ok {
				return worst
			}
			switch colours := size + 1; {
			case colours <= 2:
				return 4 // 8 px per packed pixel: 0.5 B/px
			case colours <= 4:
				return 8 // 4 px: 1 B/px
			case colours <= 16:
				return 16 // 2 px: 2 B/px
			default:
				return 0 // expanded in place
			}
		default: // predictor or colour transform: an entropy-coded sub-image follows
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
