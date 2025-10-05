package codec

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"sort"
	"strconv"
	"strings"

	"photofield/internal/codec/jpeg"
	"photofield/internal/codec/png"
	webpjack "photofield/internal/codec/webp/jack"
	webpjackdyn "photofield/internal/codec/webp/jack/dynamic"
	webpjacktra "photofield/internal/codec/webp/jack/transpiled"
)

type ImageMem int

const (
	ImageMemRGBA ImageMem = iota
	ImageMemPaletted
	ImageMemNRGBA
)

// testEncoder checks if an encoder actually works at runtime
func testEncoder(encFunc EncodeFunc) bool {
	// Create a minimal 1x1 test image
	testImg := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	testImg.Set(0, 0, color.RGBA{255, 0, 0, 255})

	// Try to encode it
	var buf bytes.Buffer
	err := encFunc(&buf, testImg, 80)
	return err == nil
}

// MediaRange represents a parsed media range from an Accept header
type MediaRange struct {
	Type       string            // e.g., "text", "*"
	Subtype    string            // e.g., "html", "*"
	Parameters map[string]string // media type parameters (excluding q and extensions)
	Quality    float64           // q parameter value (0.0-1.0)
	Extensions map[string]string // accept-ext parameters
}

type EncodeFunc func(w io.Writer, m image.Image, quality int) error

type Encoder struct {
	Func        EncodeFunc
	Mem         ImageMem
	ContentType string
	Type        EncoderType
}

type EncoderType struct {
	Subtype string
	Encoder string
}

func (et EncoderType) String() string {
	if et.Encoder != "" {
		return et.Subtype + "-" + et.Encoder
	}
	return et.Subtype
}

type MediaRanges []MediaRange

var encoderMap = map[EncoderType]Encoder{
	{"jpeg", ""}: {Func: jpeg.Encode, Mem: ImageMemRGBA, ContentType: "image/jpeg"},
	{"png", ""}:  {Func: png.Encode, Mem: ImageMemRGBA, ContentType: "image/png"},
	// {"avif", ""}:        {Func: avif.Encode, Mem: ImageMemRGBA, ContentType: "image/avif"},
	// {"webp", "chai"}:    {Func: webpchai.Encode, Mem: ImageMemRGBA, ContentType: "image/webp"},
	{"webp", ""}:        {Func: webpjack.Encode, Mem: ImageMemNRGBA, ContentType: "image/webp"},
	{"webp", "jack"}:    {Func: webpjack.Encode, Mem: ImageMemNRGBA, ContentType: "image/webp"},
	{"webp", "jackdyn"}: {Func: webpjackdyn.Encode, Mem: ImageMemNRGBA, ContentType: "image/webp"},
	{"webp", "jacktra"}: {Func: webpjacktra.Encode, Mem: ImageMemNRGBA, ContentType: "image/webp"},
	// {"webp", "hugo"}:    {Func: webphugo.Encode, Mem: ImageMemNRGBA, ContentType: "image/webp"},
	{"*", ""}: {Func: jpeg.Encode, Mem: ImageMemRGBA, ContentType: "image/jpeg"},
}

type Encoders []EncoderType

func (et Encoders) String() string {
	var sb strings.Builder
	for i, e := range et {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(e.String())
	}
	return sb.String()
}

// FastestEncoders is a list of encoders in order of speed
var FastestEncoders = Encoders{
	{"jpeg", ""},
	{"webp", "jackdyn"},
	{"webp", "jacktra"},
	{"png", ""},
}

// AlphaEncoders is a list of encoders that support transparency
var AlphaEncoders = Encoders{
	{"webp", "jackdyn"},
	{"webp", "jacktra"},
	{"png", ""},
}

var supportedEncoders map[EncoderType]bool

func init() {
	// Test which encoders actually work on this platform
	supportedEncoders = make(map[EncoderType]bool)
	for encType, encoder := range encoderMap {
		if testEncoder(encoder.Func) {
			supportedEncoders[encType] = true
		}
	}

	// Remove unsupported encoders from the encoder map
	for encType := range encoderMap {
		if !supportedEncoders[encType] {
			delete(encoderMap, encType)
		}
	}

	// Filter encoder lists to only include supported encoders
	FastestEncoders = filterSupportedEncoders(FastestEncoders)
	AlphaEncoders = filterSupportedEncoders(AlphaEncoders)
}

func filterSupportedEncoders(encoders Encoders) Encoders {
	filtered := make(Encoders, 0, len(encoders))
	for _, et := range encoders {
		if supportedEncoders[et] {
			filtered = append(filtered, et)
		}
	}
	return filtered
}

func (ets Encoders) FirstMatch(ranges MediaRanges) (Encoder, MediaRange, bool) {
	for _, et := range ets {
		for _, mr := range ranges {
			if mr.Matches("image", et.Subtype, et.Encoder) {
				enc, ok := encoderMap[et]
				if ok {
					enc.Type = et
					return enc, mr, true
				}
			}
		}
	}
	return Encoder{}, MediaRange{}, false
}

// String returns the string representation of the media range
func (mr MediaRange) String() string {
	result := mr.Type + "/" + mr.Subtype

	for key, value := range mr.Parameters {
		result += "; " + key + "=" + value
	}

	if mr.Quality != 1.0 {
		result += fmt.Sprintf("; q=%.1f", mr.Quality)
	}

	for key, value := range mr.Extensions {
		result += "; " + key
		if value != "" {
			result += "=" + value
		}
	}

	return result
}

// Matches returns true if this media range matches the given media type
func (mr MediaRange) Matches(mediaType, mediaSubtype, encoder string) bool {
	mrenc := mr.Encoder()
	if mrenc != "" && mrenc != encoder {
		return false
	}
	if mr.Type == "*" {
		return true
	}
	if mr.Type != mediaType {
		return false
	}
	return mr.Subtype == "*" || mr.Subtype == mediaSubtype
}

func (mr MediaRange) Encoder() string {
	return mr.Parameters["encoder"]
}

// Specificity returns the specificity level for precedence ordering
func (mr MediaRange) Specificity() int {
	if mr.Type == "*" {
		return 0
	}
	if mr.Subtype == "*" {
		return 1
	}
	specificity := 2
	// Add parameter count for more specific matching
	return specificity + len(mr.Parameters)
}

func (mr MediaRange) QualityParam() int {
	quality := 0
	qualityStr := mr.Parameters["quality"]
	if qualityStr != "" {
		if q, err := strconv.Atoi(qualityStr); err == nil {
			quality = q
		}
	}
	return quality
}

func (ranges MediaRanges) FirstSupported() (Encoder, MediaRange, bool) {
	for _, mr := range ranges {
		if mr.Type != "image" && mr.Type != "*" {
			continue
		}
		encName := mr.Parameters["encoder"]
		encType := EncoderType{mr.Subtype, encName}
		enc, ok := encoderMap[encType]
		if !ok {
			continue
		}
		enc.Type = encType
		return enc, mr, true
	}
	return Encoder{}, MediaRange{}, false
}

func (ranges MediaRanges) FastestEncoder() (Encoder, MediaRange, bool) {
	return FastestEncoders.FirstMatch(ranges)
}

func (ranges MediaRanges) AlphaEncoder() (Encoder, MediaRange, bool) {
	return AlphaEncoders.FirstMatch(ranges)
}

func EncodeAccepted(w io.Writer, m image.Image, acceptHeader string) error {
	ranges, err := ParseAccept(acceptHeader)
	if err != nil {
		return fmt.Errorf("parsing Accept header: %w", err)
	}

	if len(ranges) == 0 {
		return fmt.Errorf("no valid media ranges found in Accept header")
	}

	encoder, mr, ok := ranges.FirstSupported()
	if !ok {
		return fmt.Errorf("no suitable encoder found for Accept header")
	}

	quality := mr.QualityParam()

	if err := encoder.Func(w, m, quality); err != nil {
		return fmt.Errorf("encoding image: %w", err)
	}

	return nil
}

// ParseAccept parses an Accept header value and returns sorted media ranges
func ParseAccept(header string) (MediaRanges, error) {
	if header == "" {
		return []MediaRange{{Type: "*", Subtype: "*", Quality: 1.0, Parameters: make(map[string]string), Extensions: make(map[string]string)}}, nil
	}

	var ranges []MediaRange
	parts := strings.Split(header, ",")

	for _, part := range parts {
		mr, err := parseMediaRange(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, mr)
	}

	// Sort by quality (descending) then by specificity (descending)
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].Quality != ranges[j].Quality {
			return ranges[i].Quality > ranges[j].Quality
		}
		return ranges[i].Specificity() > ranges[j].Specificity()
	})

	return ranges, nil
}

func parseMediaRange(s string) (MediaRange, error) {
	mr := MediaRange{
		Quality:    1.0,
		Parameters: make(map[string]string),
		Extensions: make(map[string]string),
	}

	// Split on semicolon for media type and parameters
	parts := strings.Split(s, ";")
	if len(parts) == 0 {
		return mr, fmt.Errorf("invalid media range: %s", s)
	}

	// Parse media type
	mediaType := strings.TrimSpace(parts[0])
	typeParts := strings.Split(mediaType, "/")
	if len(typeParts) != 2 {
		return mr, fmt.Errorf("invalid media type: %s", mediaType)
	}

	mr.Type = strings.TrimSpace(typeParts[0])
	mr.Subtype = strings.TrimSpace(typeParts[1])

	// Parse parameters
	qFound := false
	for i := 1; i < len(parts); i++ {
		param := strings.TrimSpace(parts[i])
		if param == "" {
			continue
		}

		key, value, err := parseParameter(param)
		if err != nil {
			return mr, err
		}

		if key == "q" {
			q, err := strconv.ParseFloat(value, 64)
			if err != nil || q < 0 || q > 1 {
				return mr, fmt.Errorf("invalid q value: %s", value)
			}
			mr.Quality = q
			qFound = true
		} else if qFound {
			// After q parameter, everything is an extension
			mr.Extensions[key] = value
		} else {
			// Before q parameter, it's a media type parameter
			mr.Parameters[key] = value
		}
	}

	return mr, nil
}

func parseParameter(param string) (string, string, error) {
	if !strings.Contains(param, "=") {
		return param, "", nil
	}

	parts := strings.SplitN(param, "=", 2)
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	// Handle quoted strings
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
		// Unescape quoted pairs
		value = strings.ReplaceAll(value, `\"`, `"`)
		value = strings.ReplaceAll(value, `\\`, `\`)
	}

	return key, value, nil
}
