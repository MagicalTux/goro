package standard

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strings"

	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/logopt"
	"github.com/MagicalTux/goro/core/phpobj"
	"github.com/MagicalTux/goro/core/phpv"
)

// Image type constants
// > const
const (
	IMAGETYPE_GIF      phpv.ZInt = 1
	IMAGETYPE_JPEG     phpv.ZInt = 2
	IMAGETYPE_PNG      phpv.ZInt = 3
	IMAGETYPE_SWF      phpv.ZInt = 4
	IMAGETYPE_PSD      phpv.ZInt = 5
	IMAGETYPE_BMP      phpv.ZInt = 6
	IMAGETYPE_TIFF_II  phpv.ZInt = 7
	IMAGETYPE_TIFF_MM  phpv.ZInt = 8
	IMAGETYPE_JPC      phpv.ZInt = 9
	IMAGETYPE_JP2      phpv.ZInt = 10
	IMAGETYPE_JPX      phpv.ZInt = 11
	IMAGETYPE_JB2      phpv.ZInt = 12
	IMAGETYPE_SWC      phpv.ZInt = 13
	IMAGETYPE_IFF      phpv.ZInt = 14
	IMAGETYPE_WBMP     phpv.ZInt = 15
	IMAGETYPE_XBM      phpv.ZInt = 16
	IMAGETYPE_ICO      phpv.ZInt = 17
	IMAGETYPE_WEBP     phpv.ZInt = 18
	IMAGETYPE_AVIF     phpv.ZInt = 19
	IMAGETYPE_HEIF     phpv.ZInt = 20
	IMAGETYPE_JPEG2000 phpv.ZInt = 9 // alias for JPC
)

// imageInfo holds the result of parsing an image header
type imageInfo struct {
	width    int
	height   int
	imgType  int
	bits     int  // bits per channel or color depth
	channels int  // number of channels (0 means not set)
	hasBits  bool // whether bits field should be included
}

// imageTypeMimeMap maps image type constants to MIME types
var imageTypeMimeMap = map[int]string{
	1:  "image/gif",
	2:  "image/jpeg",
	3:  "image/png",
	4:  "application/x-shockwave-flash",
	5:  "image/psd",
	6:  "image/bmp",
	7:  "image/tiff",
	8:  "image/tiff",
	9:  "application/octet-stream",
	10: "image/jp2",
	11: "application/octet-stream",
	12: "application/octet-stream",
	13: "application/x-shockwave-flash",
	14: "image/iff",
	15: "image/vnd.wap.wbmp",
	16: "image/xbm",
	17: "image/vnd.microsoft.icon",
	18: "image/webp",
	19: "image/avif",
	20: "image/heif",
}

// imageTypeExtMap maps image type constants to file extensions
var imageTypeExtMap = map[int]string{
	1:  "gif",
	2:  "jpeg",
	3:  "png",
	4:  "swf",
	5:  "psd",
	6:  "bmp",
	7:  "tiff",
	8:  "tiff",
	9:  "jpc",
	10: "jp2",
	11: "jpx",
	12: "jb2",
	13: "swf",
	14: "iff",
	15: "bmp", // WBMP extension is .bmp per PHP
	16: "xbm",
	17: "ico",
	18: "webp",
	19: "avif",
	20: "heif",
}

// > func array getimagesize(string $filename, array &$image_info = null)
func fncGetImageSize(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var filename phpv.ZString
	var imageInfoRef core.OptionalRef[*phpv.ZArray]

	_, err := core.Expand(ctx, args, &filename, &imageInfoRef)
	if err != nil {
		return nil, err
	}

	// Check for null bytes
	if strings.ContainsRune(string(filename), 0) {
		return nil, phpobj.ThrowError(ctx, phpobj.ValueError, "getimagesize(): Argument #1 ($filename) must not contain any null bytes")
	}

	// Open the file via stream system
	f, openErr := ctx.Global().Open(ctx, filename, "r", false)
	if openErr != nil {
		if os.IsNotExist(openErr) {
			ctx.Warn("%s(%s): Failed to open stream: No such file or directory", ctx.GetFuncName(), filename, logopt.NoFuncName(true))
			if imageInfoRef.HasArg() {
				imageInfoRef.Set(ctx, phpv.NewZArray())
			}
			return phpv.ZFalse.ZVal(), nil
		}
		ctx.Warn("%s(%s): Failed to open stream: %s", ctx.GetFuncName(), filename, openErr, logopt.NoFuncName(true))
		if imageInfoRef.HasArg() {
			imageInfoRef.Set(ctx, phpv.NewZArray())
		}
		return phpv.ZFalse.ZVal(), nil
	}
	defer f.Close()

	// Read the header data
	data, readErr := io.ReadAll(f)
	if readErr != nil || len(data) == 0 {
		if len(data) == 0 {
			ctx.Notice("getimagesize(): Error reading from %s!", filename, logopt.NoFuncName(true))
		}
		if imageInfoRef.HasArg() {
			imageInfoRef.Set(ctx, phpv.NewZArray())
		}
		return phpv.ZFalse.ZVal(), nil
	}

	return doGetImageSize(ctx, data, &imageInfoRef)
}

// > func array getimagesizefromstring(string $string, array &$image_info = null)
func fncGetImageSizeFromString(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	var imageInfoRef core.OptionalRef[*phpv.ZArray]

	_, err := core.Expand(ctx, args, &data, &imageInfoRef)
	if err != nil {
		return nil, err
	}

	return doGetImageSize(ctx, []byte(data), &imageInfoRef)
}

// doGetImageSize is the common implementation for getimagesize and getimagesizefromstring
func doGetImageSize(ctx phpv.Context, data []byte, imageInfoRef *core.OptionalRef[*phpv.ZArray]) (*phpv.ZVal, error) {
	// Initialize imageinfo array if provided
	infoArr := phpv.NewZArray()
	if imageInfoRef.HasArg() {
		imageInfoRef.Set(ctx, infoArr)
	}

	info := detectImage(ctx, data, infoArr)
	if info == nil {
		return phpv.ZFalse.ZVal(), nil
	}

	// Build the result array
	result := phpv.NewZArray()
	result.OffsetSet(ctx, phpv.ZInt(0), phpv.ZInt(info.width).ZVal())
	result.OffsetSet(ctx, phpv.ZInt(1), phpv.ZInt(info.height).ZVal())
	result.OffsetSet(ctx, phpv.ZInt(2), phpv.ZInt(info.imgType).ZVal())
	result.OffsetSet(ctx, phpv.ZInt(3), phpv.ZString(fmt.Sprintf("width=\"%d\" height=\"%d\"", info.width, info.height)).ZVal())

	if info.hasBits && info.bits > 0 {
		result.OffsetSet(ctx, phpv.ZString("bits"), phpv.ZInt(info.bits).ZVal())
	}
	if info.channels > 0 {
		result.OffsetSet(ctx, phpv.ZString("channels"), phpv.ZInt(info.channels).ZVal())
	}

	mime := imageTypeMimeMap[info.imgType]
	if mime == "" {
		mime = "application/octet-stream"
	}
	result.OffsetSet(ctx, phpv.ZString("mime"), phpv.ZString(mime).ZVal())
	result.OffsetSet(ctx, phpv.ZString("width_unit"), phpv.ZString("px").ZVal())
	result.OffsetSet(ctx, phpv.ZString("height_unit"), phpv.ZString("px").ZVal())

	return result.ZVal(), nil
}

// detectImage detects the image type and dimensions from raw data
func detectImage(ctx phpv.Context, data []byte, infoArr *phpv.ZArray) *imageInfo {
	if len(data) < 4 {
		return nil
	}

	// Try each format
	if info := detectJPEG(ctx, data, infoArr); info != nil {
		return info
	}
	if info := detectPNG(data); info != nil {
		return info
	}
	if info := detectGIF(data); info != nil {
		return info
	}
	if info := detectBMP(data); info != nil {
		return info
	}
	if info := detectWebP(data); info != nil {
		return info
	}
	if info := detectTIFF(data); info != nil {
		return info
	}
	if info := detectPSD(data); info != nil {
		return info
	}
	if info := detectSWF(data); info != nil {
		return info
	}
	if info := detectIFF(data); info != nil {
		return info
	}
	if info := detectICO(data); info != nil {
		return info
	}
	if info := detectWBMP(data); info != nil {
		return info
	}
	if info := detectXBM(data); info != nil {
		return info
	}
	if info := detectJPC(data); info != nil {
		return info
	}
	if info := detectJP2(data); info != nil {
		return info
	}
	if info := detectAVIF(data); info != nil {
		return info
	}
	if info := detectHEIF(data); info != nil {
		return info
	}

	return nil
}

// detectJPEG detects JPEG images and extracts dimensions
func detectJPEG(ctx phpv.Context, data []byte, infoArr *phpv.ZArray) *imageInfo {
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		return nil
	}

	pos := 2
	var width, height, bits, channels int

	for pos < len(data)-1 {
		// Find next marker
		if data[pos] != 0xFF {
			// Search for the next 0xFF byte (skip extraneous bytes)
			extraBytes := 0
			for pos < len(data)-1 && data[pos] != 0xFF {
				pos++
				extraBytes++
			}
			if pos >= len(data)-1 {
				break
			}
			if extraBytes > 0 && ctx != nil {
				ctx.Warn("getimagesize(): Corrupt JPEG data: %d extraneous bytes before marker", extraBytes, logopt.NoFuncName(true))
			}
		}

		// Skip padding 0xFF bytes
		for pos < len(data)-1 && data[pos] == 0xFF {
			pos++
		}

		if pos >= len(data) {
			break
		}

		marker := data[pos]
		pos++

		// Markers that don't have a length
		if marker == 0x00 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) || marker == 0xD8 {
			continue
		}

		// End of image
		if marker == 0xD9 {
			break
		}

		// Start of scan - stop processing markers
		if marker == 0xDA {
			break
		}

		// Read segment length
		if pos+2 > len(data) {
			break
		}
		segLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		if segLen < 2 {
			break
		}

		// Collect APP markers for imageinfo
		if marker >= 0xE0 && marker <= 0xEF && infoArr != nil {
			appNum := int(marker - 0xE0)
			appKey := fmt.Sprintf("APP%d", appNum)
			if pos+segLen <= len(data) {
				// APP data is everything after the 2-byte length
				appData := data[pos+2 : pos+segLen]
				infoArr.OffsetSet(nil, phpv.ZString(appKey), phpv.ZString(string(appData)).ZVal())
			}
		}

		// SOF markers contain image dimensions
		if (marker >= 0xC0 && marker <= 0xC3) || (marker >= 0xC5 && marker <= 0xC7) ||
			(marker >= 0xC9 && marker <= 0xCB) || (marker >= 0xCD && marker <= 0xCF) {
			if pos+2+6 <= len(data) {
				bits = int(data[pos+2])
				height = int(binary.BigEndian.Uint16(data[pos+3 : pos+5]))
				width = int(binary.BigEndian.Uint16(data[pos+5 : pos+7]))
				if pos+2+7 <= len(data) {
					channels = int(data[pos+7])
				}
			}
		}

		pos += segLen
	}

	if width == 0 && height == 0 {
		return nil
	}

	return &imageInfo{
		width:    width,
		height:   height,
		imgType:  int(IMAGETYPE_JPEG),
		bits:     bits,
		channels: channels,
		hasBits:  true,
	}
}

// detectPNG detects PNG images and extracts dimensions
func detectPNG(data []byte) *imageInfo {
	// PNG signature: 89 50 4E 47 0D 0A 1A 0A
	if len(data) < 24 {
		return nil
	}
	pngSig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.Equal(data[:8], pngSig) {
		return nil
	}

	// IHDR chunk starts at offset 8
	// 4 bytes length, 4 bytes "IHDR", then width(4), height(4), bit_depth(1), color_type(1)
	if string(data[12:16]) != "IHDR" {
		return nil
	}

	width := int(binary.BigEndian.Uint32(data[16:20]))
	height := int(binary.BigEndian.Uint32(data[20:24]))

	bits := 0
	if len(data) > 24 {
		bits = int(data[24])
	}

	// Color type at byte 25 determines if we should report channels
	// For PNG, PHP does not report channels, only bits
	return &imageInfo{
		width:   width,
		height:  height,
		imgType: int(IMAGETYPE_PNG),
		bits:    bits,
		hasBits: true,
	}
}

// detectGIF detects GIF images and extracts dimensions
func detectGIF(data []byte) *imageInfo {
	if len(data) < 10 {
		return nil
	}
	header := string(data[:6])
	if header != "GIF87a" && header != "GIF89a" {
		return nil
	}

	width := int(binary.LittleEndian.Uint16(data[6:8]))
	height := int(binary.LittleEndian.Uint16(data[8:10]))

	// Packed byte at offset 10
	bits := 0
	channels := 3
	if len(data) > 10 {
		packed := data[10]
		// Global color table flag is bit 7
		// Color resolution is bits 4-6 (add 1 for actual number)
		bits = int((packed>>4)&0x07) + 1
	}

	return &imageInfo{
		width:    width,
		height:   height,
		imgType:  int(IMAGETYPE_GIF),
		bits:     bits,
		channels: channels,
		hasBits:  true,
	}
}

// detectBMP detects BMP images and extracts dimensions
func detectBMP(data []byte) *imageInfo {
	if len(data) < 26 {
		return nil
	}
	if data[0] != 'B' || data[1] != 'M' {
		return nil
	}

	// DIB header size at offset 14
	dibSize := binary.LittleEndian.Uint32(data[14:18])
	if dibSize < 12 {
		return nil
	}

	var width, height int
	var bits int

	if dibSize == 12 {
		// OS/2 BMP v1 (BITMAPCOREHEADER)
		width = int(binary.LittleEndian.Uint16(data[18:20]))
		height = int(binary.LittleEndian.Uint16(data[20:22]))
		if len(data) >= 26 {
			bits = int(binary.LittleEndian.Uint16(data[24:26]))
		}
	} else {
		// BITMAPINFOHEADER or later (40+ bytes)
		if len(data) < 30 {
			return nil
		}
		w := int32(binary.LittleEndian.Uint32(data[18:22]))
		h := int32(binary.LittleEndian.Uint32(data[22:26]))
		width = int(w)
		height = int(h)
		// Height can be negative (top-down bitmap)
		if height < 0 {
			height = -height
		}
		bits = int(binary.LittleEndian.Uint16(data[28:30]))
	}

	if width <= 0 || height <= 0 {
		return nil
	}

	return &imageInfo{
		width:   width,
		height:  height,
		imgType: int(IMAGETYPE_BMP),
		bits:    bits,
		hasBits: true,
	}
}

// detectWebP detects WebP images and extracts dimensions
func detectWebP(data []byte) *imageInfo {
	if len(data) < 12 {
		return nil
	}
	if string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return nil
	}

	if len(data) < 16 {
		return nil
	}

	chunk := string(data[12:16])
	bits := 8

	switch chunk {
	case "VP8 ":
		// Lossy VP8
		if len(data) < 30 {
			return nil
		}
		// VP8 bitstream starts at offset 20 (after chunk header + size)
		// Frame tag at offset 20: 3 bytes keyframe tag, then 7 bytes for width/height
		// Signature bytes: 9d 01 2a
		frameStart := 20
		if len(data) < frameStart+10 {
			return nil
		}
		if data[frameStart+3] != 0x9D || data[frameStart+4] != 0x01 || data[frameStart+5] != 0x2A {
			return nil
		}
		width := int(binary.LittleEndian.Uint16(data[frameStart+6:frameStart+8])) & 0x3FFF
		height := int(binary.LittleEndian.Uint16(data[frameStart+8:frameStart+10])) & 0x3FFF

		return &imageInfo{
			width:   width,
			height:  height,
			imgType: int(IMAGETYPE_WEBP),
			bits:    bits,
			hasBits: true,
		}

	case "VP8L":
		// Lossless WebP
		if len(data) < 25 {
			return nil
		}
		// VP8L data starts at offset 21 (after RIFF header + VP8L + chunk size + signature byte)
		// The signature byte at offset 20 should be 0x2F
		if data[20] != 0x2F {
			return nil
		}
		// Width and height encoded in first 4 bytes after signature
		b0 := uint32(data[21])
		b1 := uint32(data[22])
		b2 := uint32(data[23])
		b3 := uint32(data[24])

		width := int(b0 | (b1 << 8)) & 0x3FFF
		width += 1
		height := int((b1>>6)|(b2<<2)|(b3<<10)) & 0x3FFF
		height += 1

		return &imageInfo{
			width:   width,
			height:  height,
			imgType: int(IMAGETYPE_WEBP),
			bits:    bits,
			hasBits: true,
		}

	case "VP8X":
		// Extended WebP
		if len(data) < 30 {
			return nil
		}
		// Canvas width at offset 24 (3 bytes, little-endian) + 1
		width := int(data[24]) | int(data[25])<<8 | int(data[26])<<16
		width += 1
		// Canvas height at offset 27 (3 bytes, little-endian) + 1
		height := int(data[27]) | int(data[28])<<8 | int(data[29])<<16
		height += 1

		return &imageInfo{
			width:   width,
			height:  height,
			imgType: int(IMAGETYPE_WEBP),
			bits:    bits,
			hasBits: true,
		}
	}

	return nil
}

// detectTIFF detects TIFF images and extracts dimensions
func detectTIFF(data []byte) *imageInfo {
	if len(data) < 8 {
		return nil
	}

	var bo binary.ByteOrder
	var imgType int

	if data[0] == 'I' && data[1] == 'I' && data[2] == 0x2A && data[3] == 0x00 {
		bo = binary.LittleEndian
		imgType = int(IMAGETYPE_TIFF_II)
	} else if data[0] == 'M' && data[1] == 'M' && data[2] == 0x00 && data[3] == 0x2A {
		bo = binary.BigEndian
		imgType = int(IMAGETYPE_TIFF_MM)
	} else {
		return nil
	}

	// Read IFD offset
	ifdOffset := int(bo.Uint32(data[4:8]))
	if ifdOffset < 0 || ifdOffset+2 > len(data) {
		return nil
	}

	// Read number of directory entries
	numEntries := int(bo.Uint16(data[ifdOffset : ifdOffset+2]))
	pos := ifdOffset + 2

	var width, height int

	for i := 0; i < numEntries; i++ {
		if pos+12 > len(data) {
			break
		}
		tag := bo.Uint16(data[pos : pos+2])
		fieldType := bo.Uint16(data[pos+2 : pos+4])
		// count := bo.Uint32(data[pos+4 : pos+8])

		var value uint32
		switch fieldType {
		case 3: // SHORT
			value = uint32(bo.Uint16(data[pos+8 : pos+10]))
		case 4: // LONG
			value = bo.Uint32(data[pos+8 : pos+12])
		default:
			value = bo.Uint32(data[pos+8 : pos+12])
		}

		switch tag {
		case 0x0100: // ImageWidth
			width = int(value)
		case 0x0101: // ImageHeight/ImageLength
			height = int(value)
		}

		pos += 12
	}

	if width <= 0 || height <= 0 {
		return nil
	}

	return &imageInfo{
		width:   width,
		height:  height,
		imgType: imgType,
		hasBits: false,
	}
}

// detectPSD detects PSD (Photoshop) images and extracts dimensions
func detectPSD(data []byte) *imageInfo {
	if len(data) < 22 {
		return nil
	}
	// PSD signature: "8BPS"
	if string(data[:4]) != "8BPS" {
		return nil
	}

	// Version at offset 4 (2 bytes, big-endian) should be 1
	version := binary.BigEndian.Uint16(data[4:6])
	if version != 1 {
		return nil
	}

	// Height at offset 14 (4 bytes, big-endian)
	height := int(binary.BigEndian.Uint32(data[14:18]))
	// Width at offset 18 (4 bytes, big-endian)
	width := int(binary.BigEndian.Uint32(data[18:22]))

	return &imageInfo{
		width:   width,
		height:  height,
		imgType: int(IMAGETYPE_PSD),
		hasBits: false,
	}
}

// detectSWF detects SWF (Flash) images and extracts dimensions
func detectSWF(data []byte) *imageInfo {
	if len(data) < 8 {
		return nil
	}

	sig := string(data[:3])
	if sig != "FWS" && sig != "CWS" {
		return nil
	}

	imgType := int(IMAGETYPE_SWF)
	if sig == "CWS" {
		imgType = int(IMAGETYPE_SWC)
		// CWS is zlib-compressed from byte 8 onwards - skip for now
		return nil
	}

	// SWF RECT structure starts at offset 8
	// RECT is a variable-length bit field
	if len(data) < 9 {
		return nil
	}

	// Read the RECT record
	nBits := int(data[8] >> 3) // number of bits per field
	if nBits == 0 {
		return nil
	}

	// We need to read 4 fields of nBits each, plus the 5-bit nBits header
	totalBits := 5 + nBits*4
	totalBytes := (totalBits + 7) / 8
	if len(data) < 8+totalBytes {
		return nil
	}

	// Read bit fields from RECT
	rectData := data[8 : 8+totalBytes]
	bitPos := 5 // skip the 5-bit nBits header

	readBits := func(n int) int {
		val := 0
		for i := 0; i < n; i++ {
			byteIdx := bitPos / 8
			bitIdx := 7 - (bitPos % 8)
			if byteIdx < len(rectData) {
				val = val<<1 | int((rectData[byteIdx]>>uint(bitIdx))&1)
			}
			bitPos++
		}
		return val
	}

	xMin := readBits(nBits)
	xMax := readBits(nBits)
	yMin := readBits(nBits)
	yMax := readBits(nBits)

	// Convert from twips (1/20 of a pixel) to pixels
	width := (xMax - xMin) / 20
	height := (yMax - yMin) / 20

	if width <= 0 || height <= 0 {
		return nil
	}

	return &imageInfo{
		width:   width,
		height:  height,
		imgType: imgType,
		hasBits: false,
	}
}

// detectIFF detects IFF/ILBM images and extracts dimensions
func detectIFF(data []byte) *imageInfo {
	if len(data) < 12 {
		return nil
	}
	if string(data[:4]) != "FORM" {
		return nil
	}
	if string(data[8:12]) != "ILBM" {
		return nil
	}

	// Look for BMHD chunk
	pos := 12
	for pos+8 <= len(data) {
		chunkID := string(data[pos : pos+4])
		chunkSize := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		pos += 8

		if chunkID == "BMHD" && chunkSize >= 4 && pos+4 <= len(data) {
			width := int(binary.BigEndian.Uint16(data[pos : pos+2]))
			height := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))

			bits := 0
			if chunkSize >= 12 && pos+12 <= len(data) {
				bits = int(data[pos+8]) // nPlanes
			}

			return &imageInfo{
				width:   width,
				height:  height,
				imgType: int(IMAGETYPE_IFF),
				bits:    bits,
				hasBits: bits > 0,
			}
		}

		pos += chunkSize
		// IFF chunks are word-aligned (2-byte)
		if chunkSize%2 != 0 {
			pos++
		}
	}

	return nil
}

// detectICO detects ICO files and extracts dimensions
func detectICO(data []byte) *imageInfo {
	if len(data) < 22 {
		return nil
	}

	// ICO header: reserved=0, type=1 (icon), count>=1
	reserved := binary.LittleEndian.Uint16(data[0:2])
	icoType := binary.LittleEndian.Uint16(data[2:4])
	count := binary.LittleEndian.Uint16(data[4:6])

	if reserved != 0 || icoType != 1 || count == 0 {
		return nil
	}

	// First entry starts at offset 6
	// Width and height are single bytes (0 means 256)
	width := int(data[6])
	height := int(data[7])
	if width == 0 {
		width = 256
	}
	if height == 0 {
		height = 256
	}

	// Color count (0 means >=256)
	// Bits per pixel at offset 12 (2 bytes)
	bits := int(binary.LittleEndian.Uint16(data[12:14]))
	if bits == 0 {
		// Try to determine from color count
		colorCount := int(data[8])
		if colorCount > 0 {
			bits = int(math.Ceil(math.Log2(float64(colorCount))))
		} else {
			bits = 8
		}
	}

	return &imageInfo{
		width:   width,
		height:  height,
		imgType: int(IMAGETYPE_ICO),
		bits:    bits,
		hasBits: true,
	}
}

// detectWBMP detects WBMP images and extracts dimensions
func detectWBMP(data []byte) *imageInfo {
	if len(data) < 4 {
		return nil
	}

	// WBMP type: first byte should be 0x00
	if data[0] != 0x00 {
		return nil
	}

	// Second byte: fixed header (should be 0x00)
	if data[1] != 0x00 {
		return nil
	}

	// Width and height are encoded as multi-byte integers
	pos := 2
	width, newPos := readWBMPInt(data, pos)
	if newPos < 0 || width <= 0 || width > 2048 {
		return nil
	}
	pos = newPos

	height, newPos := readWBMPInt(data, pos)
	if newPos < 0 || height <= 0 || height > 2048 {
		return nil
	}

	return &imageInfo{
		width:   width,
		height:  height,
		imgType: int(IMAGETYPE_WBMP),
		hasBits: false,
	}
}

// readWBMPInt reads a multi-byte integer from WBMP data
func readWBMPInt(data []byte, pos int) (int, int) {
	val := 0
	maxBytes := 5 // prevent infinite loop
	for i := 0; i < maxBytes; i++ {
		if pos >= len(data) {
			return 0, -1
		}
		b := data[pos]
		pos++
		val = (val << 7) | int(b&0x7F)
		if b&0x80 == 0 {
			return val, pos
		}
	}
	return 0, -1
}

// xbmWidthRe and xbmHeightRe are used to detect XBM images
var xbmWidthRe = regexp.MustCompile(`#define\s+\S+_width\s+(\d+)`)
var xbmHeightRe = regexp.MustCompile(`#define\s+\S+_height\s+(\d+)`)

// detectXBM detects XBM images and extracts dimensions
func detectXBM(data []byte) *imageInfo {
	// XBM files are text-based C source files
	// Look for #define ..._width and #define ..._height
	if len(data) < 10 || data[0] != '#' {
		return nil
	}

	// Only check first 512 bytes
	checkLen := len(data)
	if checkLen > 512 {
		checkLen = 512
	}
	header := string(data[:checkLen])

	widthMatch := xbmWidthRe.FindStringSubmatch(header)
	heightMatch := xbmHeightRe.FindStringSubmatch(header)

	if widthMatch == nil || heightMatch == nil {
		return nil
	}

	width := 0
	height := 0
	fmt.Sscanf(widthMatch[1], "%d", &width)
	fmt.Sscanf(heightMatch[1], "%d", &height)

	if width <= 0 || height <= 0 {
		return nil
	}

	return &imageInfo{
		width:   width,
		height:  height,
		imgType: int(IMAGETYPE_XBM),
		hasBits: false,
	}
}

// detectJPC detects JPEG 2000 codestream (JPC) images
func detectJPC(data []byte) *imageInfo {
	if len(data) < 2 {
		return nil
	}
	// JPC starts with SOC marker FF4F
	if data[0] != 0xFF || data[1] != 0x4F {
		return nil
	}
	// Look for SIZ marker (FF51)
	if len(data) < 4 || data[2] != 0xFF || data[3] != 0x51 {
		return nil
	}
	// SIZ marker: length(2) + capabilities(2) + Xsiz(4) + Ysiz(4) + XOsiz(4) + YOsiz(4) + ...
	if len(data) < 24 {
		return nil
	}

	// Width = Xsiz - XOsiz, Height = Ysiz - YOsiz
	xsiz := int(binary.BigEndian.Uint32(data[8:12]))
	ysiz := int(binary.BigEndian.Uint32(data[12:16]))
	xosiz := int(binary.BigEndian.Uint32(data[16:20]))
	yosiz := int(binary.BigEndian.Uint32(data[20:24]))

	width := xsiz - xosiz
	height := ysiz - yosiz

	if width <= 0 || height <= 0 {
		return nil
	}

	// Number of components and bit depth
	var bits, channels int
	if len(data) >= 40 {
		numComponents := int(binary.BigEndian.Uint16(data[38:40]))
		channels = numComponents
		if numComponents > 0 && len(data) >= 41 {
			// Bit depth of first component: Ssiz field (1 byte), value+1 gives bits
			bits = int(data[40]&0x7F) + 1
		}
	}

	return &imageInfo{
		width:    width,
		height:   height,
		imgType:  int(IMAGETYPE_JPC),
		bits:     bits,
		channels: channels,
		hasBits:  bits > 0,
	}
}

// detectJP2 detects JPEG 2000 (JP2) images
func detectJP2(data []byte) *imageInfo {
	if len(data) < 12 {
		return nil
	}
	// JP2 signature box: 0000 000C 6A50 2020 0D0A 870A
	jp2Sig := []byte{0x00, 0x00, 0x00, 0x0C, 0x6A, 0x50, 0x20, 0x20, 0x0D, 0x0A, 0x87, 0x0A}
	if !bytes.Equal(data[:12], jp2Sig) {
		return nil
	}

	// Search for the ihdr box (image header)
	pos := 12
	for pos+8 <= len(data) {
		boxLen := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		boxType := string(data[pos+4 : pos+8])

		if boxLen == 0 {
			break
		}
		if boxLen == 1 && pos+16 <= len(data) {
			// Extended box length
			boxLen = int(binary.BigEndian.Uint64(data[pos+8 : pos+16]))
		}

		if boxType == "ihdr" && pos+22 <= len(data) {
			height := int(binary.BigEndian.Uint32(data[pos+8 : pos+12]))
			width := int(binary.BigEndian.Uint32(data[pos+12 : pos+16]))
			numComp := int(binary.BigEndian.Uint16(data[pos+16 : pos+18]))
			bpc := int(data[pos+18]&0x7F) + 1

			return &imageInfo{
				width:    width,
				height:   height,
				imgType:  int(IMAGETYPE_JP2),
				bits:     bpc,
				channels: numComp,
				hasBits:  true,
			}
		}

		// For container boxes, search inside
		if boxType == "jp2h" || boxType == "ftyp" {
			// Search inside the box
			innerEnd := pos + boxLen
			if innerEnd > len(data) {
				innerEnd = len(data)
			}
			innerPos := pos + 8
			for innerPos+8 <= innerEnd {
				innerBoxLen := int(binary.BigEndian.Uint32(data[innerPos : innerPos+4]))
				innerBoxType := string(data[innerPos+4 : innerPos+8])

				if innerBoxLen == 0 {
					break
				}

				if innerBoxType == "ihdr" && innerPos+22 <= len(data) {
					height := int(binary.BigEndian.Uint32(data[innerPos+8 : innerPos+12]))
					width := int(binary.BigEndian.Uint32(data[innerPos+12 : innerPos+16]))
					numComp := int(binary.BigEndian.Uint16(data[innerPos+16 : innerPos+18]))
					bpc := int(data[innerPos+18]&0x7F) + 1

					return &imageInfo{
						width:    width,
						height:   height,
						imgType:  int(IMAGETYPE_JP2),
						bits:     bpc,
						channels: numComp,
						hasBits:  true,
					}
				}

				innerPos += innerBoxLen
			}
		}

		pos += boxLen
	}

	return nil
}

// detectAVIF detects AVIF images by checking ftyp box for "avif" or "avis" brands
func detectAVIF(data []byte) *imageInfo {
	return detectISOBMFF(data, "avif")
}

// detectHEIF detects HEIF/HEIC images by checking ftyp box
func detectHEIF(data []byte) *imageInfo {
	return detectISOBMFF(data, "heif")
}

// detectISOBMFF detects AVIF or HEIF images from ISOBMFF container
func detectISOBMFF(data []byte, targetFormat string) *imageInfo {
	if len(data) < 12 {
		return nil
	}

	// Check ftyp box
	ftypSize := int(binary.BigEndian.Uint32(data[0:4]))
	if ftypSize < 8 || string(data[4:8]) != "ftyp" {
		return nil
	}
	if ftypSize > len(data) {
		ftypSize = len(data)
	}

	// Check major brand and compatible brands
	isAVIF := false
	isHEIF := false

	if ftypSize >= 12 {
		majorBrand := string(data[8:12])
		if isAVIFBrand(majorBrand) {
			isAVIF = true
		} else if isHEIFBrand(majorBrand) {
			isHEIF = true
		}

		// Check compatible brands (starting at offset 16, each 4 bytes)
		for i := 16; i+4 <= ftypSize; i += 4 {
			brand := string(data[i : i+4])
			if isAVIFBrand(brand) {
				isAVIF = true
			}
			if isHEIFBrand(brand) && !isAVIF {
				isHEIF = true
			}
		}
	}

	if targetFormat == "avif" && !isAVIF {
		return nil
	}
	if targetFormat == "heif" && !isHEIF {
		return nil
	}
	// AVIF takes priority over HEIF
	if targetFormat == "heif" && isAVIF {
		return nil
	}

	imgType := int(IMAGETYPE_AVIF)
	if targetFormat == "heif" {
		imgType = int(IMAGETYPE_HEIF)
	}

	// Search for ispe box in the ipco container to get dimensions
	width, height, bits, channels := parseISOBMFFDimensions(data)
	if width <= 0 || height <= 0 {
		return nil
	}

	return &imageInfo{
		width:    width,
		height:   height,
		imgType:  imgType,
		bits:     bits,
		channels: channels,
		hasBits:  bits > 0,
	}
}

func isAVIFBrand(brand string) bool {
	return brand == "avif" || brand == "avis"
}

func isHEIFBrand(brand string) bool {
	return brand == "heic" || brand == "heis" || brand == "heix" ||
		brand == "heim" || brand == "hevc" || brand == "hevx" ||
		brand == "mif1"
}

// parseISOBMFFDimensions finds ispe and pixi boxes in an ISOBMFF file to get width/height/bits/channels
func parseISOBMFFDimensions(data []byte) (int, int, int, int) {
	// Search for ispe and pixi boxes anywhere in the data
	width, height := 0, 0
	bits, channels := 0, 0

	// Find ispe (image spatial extents) box
	for i := 0; i+12 <= len(data); i++ {
		if string(data[i:i+4]) == "ispe" && i+12 <= len(data) {
			// ispe box: 4 bytes version/flags, 4 bytes width, 4 bytes height
			w := int(binary.BigEndian.Uint32(data[i+4 : i+8]))
			h := int(binary.BigEndian.Uint32(data[i+8 : i+12]))
			if w > 0 && h > 0 {
				width = w
				height = h
				break
			}
		}
	}

	// Find pixi (pixel information) box
	for i := 0; i+4 <= len(data); i++ {
		if string(data[i:i+4]) == "pixi" && i+8 <= len(data) {
			// pixi box: 4 bytes version/flags, 1 byte num_channels, then N bytes of bits_per_channel
			numCh := int(data[i+7])
			if numCh > 0 && i+8+numCh <= len(data) {
				channels = numCh
				bits = int(data[i+8]) // bits per channel from first channel
				break
			}
		}
	}

	return width, height, bits, channels
}

// > func string image_type_to_mime_type(int $image_type)
func fncImageTypeToMimeType(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var imgType phpv.ZInt
	_, err := core.Expand(ctx, args, &imgType)
	if err != nil {
		return nil, err
	}

	mime, ok := imageTypeMimeMap[int(imgType)]
	if !ok {
		mime = "application/octet-stream"
	}

	return phpv.ZString(mime).ZVal(), nil
}

// > func string|false image_type_to_extension(int $image_type, bool $include_dot = true)
func fncImageTypeToExtension(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var imgType phpv.ZInt
	if err := core.ExpandAt(ctx, args, 0, &imgType); err != nil {
		return nil, err
	}

	includeDot := true
	if len(args) >= 2 {
		var dot phpv.ZBool
		if err := core.ExpandAt(ctx, args, 1, &dot); err != nil {
			return nil, err
		}
		includeDot = bool(dot)
	}

	ext, ok := imageTypeExtMap[int(imgType)]
	if !ok {
		return phpv.ZFalse.ZVal(), nil
	}

	if includeDot {
		ext = "." + ext
	}

	return phpv.ZString(ext).ZVal(), nil
}
