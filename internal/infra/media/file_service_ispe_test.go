package media

import (
	"encoding/binary"
	"testing"
)

func TestFindISPEAcceptsLegalDimensions(t *testing.T) {
	cases := []struct {
		name   string
		width  uint32
		height uint32
	}{
		{name: "normal", width: 1536, height: 1024},
		{name: "boundary-at-limit", width: 30000, height: 30000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, h, ok := findISPE(buildISPEBox(tc.width, tc.height))
			if !ok {
				t.Fatalf("findISPE missed for %dx%d", tc.width, tc.height)
			}
			if w != int(tc.width) || h != int(tc.height) {
				t.Fatalf("findISPE = %dx%d, want %dx%d", w, h, tc.width, tc.height)
			}
		})
	}
}

func TestFindISPERejectsOversizedDimensions(t *testing.T) {
	cases := []struct {
		name   string
		width  uint32
		height uint32
	}{
		{name: "width-over-limit", width: 30001, height: 100},
		{name: "height-over-limit", width: 100, height: 30001},
		{name: "both-over-limit", width: 40000, height: 40000},
		{name: "uint32-max", width: 4294967295, height: 4294967295},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if w, h, ok := findISPE(buildISPEBox(tc.width, tc.height)); ok {
				t.Fatalf("findISPE accepted illegal size %dx%d (got %dx%d)", tc.width, tc.height, w, h)
			}
		})
	}
}

func TestDecodeImageConfigRejectsOversizedHEIFDimensions(t *testing.T) {
	data := buildHEIFTestFile("heic", 4294967295, 4294967295, false)
	if _, _, err := decodeImageConfig(data); err == nil {
		t.Fatal("decodeImageConfig accepted oversized HEIC dimensions")
	}

	data = buildHEIFTestFile("mif1", 40000, 100, true)
	if _, _, err := decodeImageConfig(data); err == nil {
		t.Fatal("decodeImageConfig accepted oversized HEIF width")
	}

	data = buildHEIFTestFile("heic", 100, 40000, false)
	if _, _, err := decodeImageConfig(data); err == nil {
		t.Fatal("decodeImageConfig accepted oversized HEIF height")
	}
}

func TestDecodeImageConfigAcceptsHEIFAtDimensionLimit(t *testing.T) {
	data := buildHEIFTestFile("heic", 30000, 30000, false)

	config, format, err := decodeImageConfig(data)
	if err != nil {
		t.Fatalf("decodeImageConfig error: %v", err)
	}
	if format != "heic" {
		t.Fatalf("format = %q, want heic", format)
	}
	if config.Width != 30000 || config.Height != 30000 {
		t.Fatalf("config = %dx%d, want 30000x30000", config.Width, config.Height)
	}
}

func buildISPEBox(width uint32, height uint32) []byte {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[4:8], width)
	binary.BigEndian.PutUint32(payload[8:12], height)
	return makeBox("ispe", payload)
}
