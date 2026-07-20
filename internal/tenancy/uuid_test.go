package tenancy

import (
	"strings"
	"testing"
)

func TestIsValidUUID(t *testing.T) {
	// Boundary value analysis on length (35/36/37) and separator positions,
	// equivalence partitions for case, plus error-guessing cases shaped like
	// injection payloads — the function guards path/body parameters before
	// they reach SQL.
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"canonical_lowercase_is_valid", "0d4907ff-5787-4a3b-b00e-3bf74d224c39", true},
		{"canonical_uppercase_is_valid", "0D4907FF-5787-4A3B-B00E-3BF74D224C39", true},
		{"mixed_case_is_valid", "0d4907FF-5787-4a3b-B00E-3bf74d224c39", true},
		{"all_zeros_is_valid", "00000000-0000-0000-0000-000000000000", true},

		{"empty_string_is_invalid", "", false},
		{"length_35_is_invalid", "0d4907ff-5787-4a3b-b00e-3bf74d224c3", false},
		{"length_37_is_invalid", "0d4907ff-5787-4a3b-b00e-3bf74d224c39a", false},
		{"missing_hyphens_is_invalid", "0d4907ff57874a3bb00e3bf74d224c39", false},
		{"hyphen_in_wrong_position_is_invalid", "0d4907f-f5787-4a3b-b00e-3bf74d224c39", false},
		{"non_hex_character_is_invalid", "0d4907gg-5787-4a3b-b00e-3bf74d224c39", false},
		{"braced_uuid_is_invalid", "{0d4907ff-5787-4a3b-b00e-3bf74d224c3}", false},
		{"urn_prefix_is_invalid", "urn:uuid:0d4907ff-5787-4a3b-b00e-3bf7", false},
		{"whitespace_padding_is_invalid", " 0d4907ff-5787-4a3b-b00e-3bf74d224c3", false},
		{"sql_injection_shape_is_invalid", "'; DROP TABLE teams; --             ", false},
		{"unicode_hex_lookalike_is_invalid", "0d4907ff-5787-4a3b-b00e-3bf74d224c3①", false},
		{"very_long_input_is_invalid", strings.Repeat("a", 1024), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidUUID(tc.input); got != tc.want {
				t.Fatalf("IsValidUUID(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
