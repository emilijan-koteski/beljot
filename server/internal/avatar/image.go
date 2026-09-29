package avatar

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"

	"github.com/gen2brain/webp"
	"golang.org/x/image/draw"
	xwebp "golang.org/x/image/webp"

	"github.com/emilijan/beljot/server/internal/apperr"
	"github.com/emilijan/beljot/server/internal/user"
)

// Limits on what an upload may be. The client mirrors the first two before
// sending; the server is the authority for all of them.
const (
	// MaxFileBytes caps the uploaded file itself: 2 MiB = 2,097,152 bytes.
	MaxFileBytes = 2 << 20
	// minSourceSide is the smallest shorter side accepted, so the 128 image is
	// never an upscale of something smaller.
	minSourceSide = 128
	// maxSourceSide and maxSourcePixels bound the decode: a 4096×4096 RGBA
	// decode alone is 64 MiB, so both are checked from the header before any
	// pixel is decoded. 16 megapixels is decimal (16,000,000).
	maxSourceSide   = 4096
	maxSourcePixels = 16_000_000
)

// Output encoding: lossy WebP, the same parameters for both sizes.
const (
	webpQuality = 80
	webpMethod  = 4
)

const (
	mimeJPEG = "image/jpeg"
	mimePNG  = "image/png"
	mimeWebP = "image/webp"
)

// Source is an upload that passed every cheap check (size, sniffed type,
// header dimensions) and is ready to be decoded. Splitting Inspect from Render
// lets the handler reject bad uploads before they occupy a decode slot.
type Source struct {
	data   []byte
	mime   string
	width  int
	height int
}

// Process is the whole pipeline in one call: sniff, header limits, decode,
// EXIF orientation, centre crop, resample to 256 then 128, encode both.
func Process(r io.Reader) (large, small []byte, err error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxFileBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("reading avatar: %w", err)
	}
	src, err := Inspect(data)
	if err != nil {
		return nil, nil, err
	}
	return src.Render()
}

// Inspect runs the checks that cost no pixel decode. The type is decided by
// sniffing the bytes; any filename or declared content type is irrelevant.
func Inspect(data []byte) (*Source, error) {
	if len(data) > MaxFileBytes {
		return nil, apperr.ErrAvatarTooLarge
	}

	mime := http.DetectContentType(data)
	var decodeConfig func(io.Reader) (image.Config, error)
	switch mime {
	case mimeJPEG:
		decodeConfig = jpeg.DecodeConfig
	case mimePNG:
		decodeConfig = png.DecodeConfig
	case mimeWebP:
		if isAnimatedWebP(data) {
			return nil, apperr.ErrAvatarUnsupportedType
		}
		decodeConfig = xwebp.DecodeConfig
	default:
		return nil, apperr.ErrAvatarUnsupportedType
	}

	cfg, err := decodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, apperr.ErrAvatarUnsupportedType
	}
	if cfg.Width > maxSourceSide || cfg.Height > maxSourceSide ||
		int64(cfg.Width)*int64(cfg.Height) > maxSourcePixels {
		return nil, apperr.ErrAvatarDimensionsTooLarge
	}
	if min(cfg.Width, cfg.Height) < minSourceSide {
		return nil, apperr.ErrAvatarTooSmall
	}
	// The pixel caps bound the size, not the decode cost: a progressive,
	// 4-component or 16-bit source of the same size can need several times the
	// memory (see decodemem.go). A header the predictor cannot read is priced at
	// its format's worst case, so the error is only informative here.
	if need, _ := predictDecodeBytes(data, mime, cfg.Width, cfg.Height); need > maxDecodeBytes {
		return nil, apperr.ErrAvatarDimensionsTooLarge
	}

	return &Source{data: data, mime: mime, width: cfg.Width, height: cfg.Height}, nil
}

// Render decodes the source once and produces both WebP derivatives. The 128
// is resampled from the 256, not from the source, so the one expensive
// resample runs once. Neither output carries any metadata: they are encoded
// from pixels alone.
func (s *Source) Render() (large, small []byte, err error) {
	img, err := s.decode()
	if err != nil {
		// The header parsed but the pixels did not: truncated or corrupt data
		// behind a valid-looking header is as unsupported as the wrong type.
		return nil, nil, apperr.ErrAvatarUnsupportedType
	}

	// Centre square. The crop is the same region whichever way the EXIF
	// orientation turns the picture (the centre square maps onto itself under
	// every rotation and flip), so orientation is applied to the small result
	// below rather than to the full-size decode.
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	x0 := b.Min.X + (b.Dx()-side)/2
	y0 := b.Min.Y + (b.Dy()-side)/2
	crop := image.Rect(x0, y0, x0+side, y0+side)

	// Resample in premultiplied RGBA, which filters edges of transparent
	// images correctly, then hand the encoder straight (non-premultiplied)
	// NRGBA, the layout libwebp imports.
	big := image.NewRGBA(image.Rect(0, 0, user.AvatarSizeLarge, user.AvatarSizeLarge))
	draw.CatmullRom.Scale(big, big.Bounds(), img, crop, draw.Src, nil)
	if s.mime == mimeJPEG {
		big = orient(big, jpegOrientation(s.data))
	}

	little := image.NewRGBA(image.Rect(0, 0, user.AvatarSizeSmall, user.AvatarSizeSmall))
	draw.CatmullRom.Scale(little, little.Bounds(), big, big.Bounds(), draw.Src, nil)

	if large, err = encodeWebP(big); err != nil {
		return nil, nil, err
	}
	if small, err = encodeWebP(little); err != nil {
		return nil, nil, err
	}
	return large, small, nil
}

func (s *Source) decode() (image.Image, error) {
	r := bytes.NewReader(s.data)
	switch s.mime {
	case mimeJPEG:
		return jpeg.Decode(r)
	case mimePNG:
		return png.Decode(r)
	case mimeWebP:
		return xwebp.Decode(r)
	default:
		return nil, fmt.Errorf("unsupported source type %q", s.mime)
	}
}

func encodeWebP(m *image.RGBA) ([]byte, error) {
	straight := image.NewNRGBA(m.Bounds())
	draw.Draw(straight, straight.Bounds(), m, m.Bounds().Min, draw.Src)
	var buf bytes.Buffer
	if err := webp.Encode(&buf, straight, webp.Options{Quality: webpQuality, Method: webpMethod}); err != nil {
		return nil, fmt.Errorf("encoding webp: %w", err)
	}
	return buf.Bytes(), nil
}

// isAnimatedWebP reports whether the extended (VP8X) header sets the
// animation flag. An animated WebP has no still image at the top level, so it
// is rejected by type before anything tries to decode it.
func isAnimatedWebP(data []byte) bool {
	const animationFlag = 1 << 1
	return len(data) >= 21 && string(data[12:16]) == "VP8X" && data[20]&animationFlag != 0
}

// jpegOrientation returns the EXIF orientation (1-8) from a JPEG's APP1
// segment, or 1 when there is none or it cannot be read. A missing or
// malformed tag never fails an upload; the picture is just kept as stored.
func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		switch {
		case marker == 0xFF: // fill byte before a marker
			i++
			continue
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD8): // no length field
			i += 2
			continue
		case marker == 0xDA || marker == 0xD9: // scan data or end: metadata comes first
			return 1
		}
		segLen := int(binary.BigEndian.Uint16(data[i+2:]))
		if segLen < 2 || i+2+segLen > len(data) {
			return 1
		}
		if marker == 0xE1 {
			if o := exifOrientation(data[i+4 : i+2+segLen]); o != 0 {
				return o
			}
		}
		i += 2 + segLen
	}
	return 1
}

// exifOrientation reads tag 0x0112 from IFD0 of an APP1 "Exif" payload, or 0
// when the payload is not EXIF or carries no valid orientation.
func exifOrientation(seg []byte) int {
	const header = "Exif\x00\x00"
	if len(seg) < len(header)+8 || string(seg[:len(header)]) != header {
		return 0
	}
	tiff := seg[len(header):]
	var bo binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0
	}
	if bo.Uint16(tiff[2:]) != 42 {
		return 0
	}
	ifd := int64(bo.Uint32(tiff[4:]))
	if ifd < 8 || ifd+2 > int64(len(tiff)) {
		return 0
	}
	entries := int64(bo.Uint16(tiff[ifd:]))
	for k := int64(0); k < entries; k++ {
		e := ifd + 2 + k*12
		if e+12 > int64(len(tiff)) {
			return 0
		}
		if bo.Uint16(tiff[e:]) != 0x0112 {
			continue
		}
		const typeShort = 3
		if bo.Uint16(tiff[e+2:]) != typeShort {
			return 0
		}
		if v := int(bo.Uint16(tiff[e+8:])); v >= 1 && v <= 8 {
			return v
		}
		return 0
	}
	return 0
}

// orient returns the square image m transformed for display according to an
// EXIF orientation value (1 and unknown values return m unchanged).
func orient(m *image.RGBA, orientation int) *image.RGBA {
	if orientation <= 1 || orientation > 8 {
		return m
	}
	n := m.Bounds().Dx()
	// src maps a destination pixel to the stored pixel shown there.
	var src func(x, y int) (int, int)
	switch orientation {
	case 2: // mirror horizontal
		src = func(x, y int) (int, int) { return n - 1 - x, y }
	case 3: // rotate 180
		src = func(x, y int) (int, int) { return n - 1 - x, n - 1 - y }
	case 4: // mirror vertical
		src = func(x, y int) (int, int) { return x, n - 1 - y }
	case 5: // transpose
		src = func(x, y int) (int, int) { return y, x }
	case 6: // rotate 90 clockwise
		src = func(x, y int) (int, int) { return y, n - 1 - x }
	case 7: // transverse
		src = func(x, y int) (int, int) { return n - 1 - y, n - 1 - x }
	case 8: // rotate 90 counter-clockwise
		src = func(x, y int) (int, int) { return n - 1 - y, x }
	}
	out := image.NewRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			sx, sy := src(x, y)
			so := m.PixOffset(m.Rect.Min.X+sx, m.Rect.Min.Y+sy)
			do := out.PixOffset(x, y)
			copy(out.Pix[do:do+4], m.Pix[so:so+4])
		}
	}
	return out
}
