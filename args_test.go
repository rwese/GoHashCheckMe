package main

import (
	"testing"
)

func TestParseExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[int]bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single code",
			input:    "0",
			expected: map[int]bool{0: true},
		},
		{
			name:     "multiple codes",
			input:    "0,1,2",
			expected: map[int]bool{0: true, 1: true, 2: true},
		},
		{
			name:     "with spaces",
			input:    " 0 , 1 , 2 ",
			expected: map[int]bool{0: true, 1: true, 2: true},
		},
		{
			name:     "with invalid codes",
			input:    "0,abc,2",
			expected: map[int]bool{0: true, 2: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseExitCodes(tt.input)
			if tt.expected == nil && result != nil {
				t.Errorf("expected nil, got %v", result)
				return
			}
			if tt.expected != nil && result == nil {
				t.Errorf("expected %v, got nil", tt.expected)
				return
			}
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d codes, got %d", len(tt.expected), len(result))
				return
			}
			for code := range tt.expected {
				if !result[code] {
					t.Errorf("expected code %d to be present", code)
				}
			}
		})
	}
}
