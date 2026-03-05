package cli

import "testing"

func TestSplitPackageArg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input       string
		wantName    string
		wantVersion string
	}{
		{input: "foo", wantName: "foo", wantVersion: ""},
		{input: "foo@1.2.3", wantName: "foo", wantVersion: "1.2.3"},
		{input: "foo@", wantName: "foo", wantVersion: ""},
		{input: "@bar", wantName: "", wantVersion: "bar"},
		// Only the first '@' splits; remainder is the version string.
		{input: "foo@1.2.3@extra", wantName: "foo", wantVersion: "1.2.3@extra"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			gotName, gotVersion := splitPackageArg(tt.input)

			if gotName != tt.wantName {
				t.Errorf("name = %q, want %q", gotName, tt.wantName)
			}

			if gotVersion != tt.wantVersion {
				t.Errorf("version = %q, want %q", gotVersion, tt.wantVersion)
			}
		})
	}
}
