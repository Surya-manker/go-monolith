package services

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"unicode"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/ean"
	"github.com/boombuler/barcode/qr"
)

// BarcodeService generates barcode and QR code images as PNG bytes.
type BarcodeService struct{}

func NewBarcodeService() *BarcodeService {
	return &BarcodeService{}
}

// GenerateCode128 returns a PNG of a Code128 barcode for any printable ASCII text.
func (s *BarcodeService) GenerateCode128(text string, width, height int) ([]byte, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("barcode text cannot be empty")
	}
	for _, r := range text {
		if !unicode.IsPrint(r) || r > 127 {
			return nil, fmt.Errorf("Code128 only supports printable ASCII characters")
		}
	}
	bc, err := code128.Encode(text)
	if err != nil {
		return nil, err
	}
	return scalePNG(bc, width, height)
}

// GenerateEAN13 returns a PNG of an EAN-13 barcode.
// code must be exactly 13 digits; if 12 are given the check digit is appended.
func (s *BarcodeService) GenerateEAN13(code string, width, height int) ([]byte, error) {
	code = strings.TrimSpace(code)
	for _, r := range code {
		if r < '0' || r > '9' {
			return nil, errors.New("EAN-13 code must contain digits only")
		}
	}
	switch len(code) {
	case 12:
		code = s.addEAN13CheckDigit(code)
	case 13:
		if !s.ValidateEAN13(code) {
			return nil, errors.New("EAN-13 check digit is invalid")
		}
	default:
		return nil, fmt.Errorf("EAN-13 requires 12 or 13 digits, got %d", len(code))
	}
	bc, err := ean.Encode(code)
	if err != nil {
		return nil, err
	}
	return scalePNG(bc, width, height)
}

// GenerateQR returns a PNG of a QR code for any text (URL, barcode value, etc.).
func (s *BarcodeService) GenerateQR(text string, size int) ([]byte, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("QR text cannot be empty")
	}
	bc, err := qr.Encode(text, qr.M, qr.Auto)
	if err != nil {
		return nil, err
	}
	return scalePNG(bc, size, size)
}

// AutoGenerateEAN13 produces a deterministic, valid EAN-13 code for a business+product pair.
// Format: 0 + bizID(3 digits) + productID(7 digits) + checkDigit(1 digit)
func (s *BarcodeService) AutoGenerateEAN13(bizID, productID int) string {
	base := fmt.Sprintf("0%03d%07d", bizID%1000, productID%10_000_000)
	return s.addEAN13CheckDigit(base)
}

// ValidateEAN13 returns true if the 13-digit code has a correct check digit.
func (s *BarcodeService) ValidateEAN13(code string) bool {
	if len(code) != 13 {
		return false
	}
	sum := 0
	for i, ch := range code[:12] {
		d := int(ch - '0')
		if i%2 == 0 {
			sum += d
		} else {
			sum += d * 3
		}
	}
	expected := (10 - (sum % 10)) % 10
	got := int(code[12] - '0')
	return expected == got
}

// addEAN13CheckDigit appends the EAN-13 check digit to a 12-digit string.
func (s *BarcodeService) addEAN13CheckDigit(code12 string) string {
	if len(code12) != 12 {
		return code12
	}
	sum := 0
	for i, ch := range code12 {
		d := int(ch - '0')
		if i%2 == 0 {
			sum += d
		} else {
			sum += d * 3
		}
	}
	check := (10 - (sum % 10)) % 10
	return fmt.Sprintf("%s%d", code12, check)
}

// scalePNG scales a barcode.Barcode to the requested size and encodes it as PNG bytes
// with a white background to ensure visibility on all backgrounds.
func scalePNG(bc barcode.Barcode, w, h int) ([]byte, error) {
	if w <= 0 {
		w = 300
	}
	if h <= 0 {
		h = 80
	}
	scaled, err := barcode.Scale(bc, w, h)
	if err != nil {
		return nil, err
	}

	// Add a 4px white padding on all sides for better readability.
	const pad = 4
	padded := image.NewRGBA(image.Rect(0, 0, w+2*pad, h+2*pad))
	draw.Draw(padded, padded.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	draw.Draw(padded, image.Rect(pad, pad, w+pad, h+pad), scaled, image.Point{}, draw.Src)

	var buf bytes.Buffer
	if err = png.Encode(&buf, padded); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
