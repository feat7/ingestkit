package schema

import (
	"testing"
)

func TestValidateSchemaVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		// Valid versions
		{"major.minor", "1.0", false},
		{"major.minor.patch", "1.0.0", false},
		{"higher version", "2.5", false},
		{"three part version", "2.5.1", false},
		{"large numbers", "10.20.30", false},

		// Invalid versions
		{"missing minor", "1", true},
		{"too many parts", "1.0.0.0", true},
		{"empty string", "", true},
		{"non-numeric major", "v1.0", true},
		{"non-numeric minor", "1.x", true},
		{"non-numeric patch", "1.0.x", true},
		{"empty component", "1..0", true},
		{"trailing dot", "1.0.", true},
		{"leading dot", ".1.0", true},
		{"letters", "one.two", true},
		{"special chars", "1.0-beta", true},
		{"spaces", "1.0 ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSchemaVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSchemaVersion(%q) error = %v, wantErr %v",
					tt.version, err, tt.wantErr)
			}
		})
	}
}

func TestSchemaVersionValidationInParsing(t *testing.T) {
	// Test that schema parsing validates version
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid version",
			yaml: `version: "1.0"
events:
  test_event:
    description: Test
    fields:
      id:
        type: string
        required: true`,
			wantErr: false,
		},
		{
			name: "invalid version format",
			yaml: `version: "v1.0"
events:
  test_event:
    description: Test
    fields:
      id:
        type: string
        required: true`,
			wantErr: true,
		},
		{
			name: "missing version",
			yaml: `events:
  test_event:
    description: Test
    fields:
      id:
        type: string
        required: true`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSchema([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSchema() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
