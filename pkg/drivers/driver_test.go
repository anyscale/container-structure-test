package drivers

import (
	"testing"
)

func TestConvertSliceToMapWithMissingEquals(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected map[string]string
	}{
		{
			name:  "empty slice",
			input: []string{},
			expected: map[string]string{},
		},
		{
			name:  "normal key=value pairs",
			input: []string{"KEY=value", "FOO=bar"},
			expected: map[string]string{
				"KEY": "value",
				"FOO": "bar",
			},
		},
		{
			name:  "bare env var without equals",
			input: []string{"EMPTY"},
			expected: map[string]string{
				"EMPTY": "",
			},
		},
		{
			name:  "mixed bare and normal pairs",
			input: []string{"EMPTY", "PATH=/usr/local/bin", "ANOTHER_EMPTY", "FOO=bar"},
			expected: map[string]string{
				"EMPTY":         "",
				"PATH":          "/usr/local/bin",
				"ANOTHER_EMPTY": "",
				"FOO":           "bar",
			},
		},
		{
			name:  "value with multiple equals signs",
			input: []string{"EQUATION=a=b=c"},
			expected: map[string]string{
				"EQUATION": "a=b=c",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertSliceToMap(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("map length: got %d, expected %d", len(got), len(tt.expected))
			}
			for k, v := range tt.expected {
				if gotVal, ok := got[k]; !ok {
					t.Fatalf("missing key %q", k)
				} else if gotVal != v {
					t.Fatalf("key %q: got %q, expected %q", k, gotVal, v)
				}
			}
		})
	}
}
