package main

import "testing"

func TestParsePreviewDimensions_Clamp(t *testing.T) {
	tests := []struct {
		name    string
		origW   int
		origH   int
		reqW    *int
		reqH    *int
		wantW   int
		wantH   int
		wantErr bool
	}{
		{
			name:    "both specified exceed clamp",
			origW:   8000,
			origH:   6000,
			reqW:    intPtr(100000),
			reqH:    intPtr(100000),
			wantW:   4096,
			wantH:   4096,
			wantErr: false,
		},
		{
			name:    "both specified within limits",
			origW:   8000,
			origH:   6000,
			reqW:    intPtr(1000),
			reqH:    intPtr(2000),
			wantW:   1000,
			wantH:   2000,
			wantErr: false,
		},
		{
			name:    "width exceeds clamp, height within",
			origW:   8000,
			origH:   6000,
			reqW:    intPtr(5000),
			reqH:    intPtr(100),
			wantW:   4096,
			wantH:   100,
			wantErr: false,
		},
		{
			name:    "height exceeds clamp, width within",
			origW:   8000,
			origH:   6000,
			reqW:    intPtr(100),
			reqH:    intPtr(5000),
			wantW:   100,
			wantH:   4096,
			wantErr: false,
		},
		{
			name:    "only width specified exceeds clamp",
			origW:   8000,
			origH:   6000,
			reqW:    intPtr(10000),
			reqH:    nil,
			wantW:   4096,
			wantH:   3072,
			wantErr: false,
		},
		{
			name:    "only height specified exceeds clamp",
			origW:   8000,
			origH:   6000,
			reqW:    nil,
			reqH:    intPtr(10000),
			wantW:   4096,
			wantH:   3072,
			wantErr: false,
		},
		{
			name:    "neither specified, original within limits",
			origW:   4000,
			origH:   3000,
			reqW:    nil,
			reqH:    nil,
			wantW:   4000,
			wantH:   3000,
			wantErr: false,
		},
		{
			name:    "neither specified, original exceeds clamp",
			origW:   8000,
			origH:   6000,
			reqW:    nil,
			reqH:    nil,
			wantW:   4096,
			wantH:   3072,
			wantErr: false,
		},
		{
			name:    "zero dimensions rejected",
			origW:   8000,
			origH:   6000,
			reqW:    intPtr(0),
			reqH:    intPtr(100),
			wantW:   0,
			wantH:   0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotW, gotH, err := parsePreviewDimensions(tt.origW, tt.origH, tt.reqW, tt.reqH)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePreviewDimensions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotW != tt.wantW || gotH != tt.wantH {
				t.Errorf("parsePreviewDimensions() = (%d, %d), want (%d, %d)", gotW, gotH, tt.wantW, tt.wantH)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
