package ingest

import "testing"

func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				APIBaseURL:     "https://bark.internal",
				APIToken:       "tok-secret",
				SigningKeyPath: "/keys/cosign.key",
				PolicyPath:     "/policy/license-policy.yaml",
			},
			wantErr: false,
		},
		{
			name:    "all fields empty",
			cfg:     Config{},
			wantErr: true,
		},
		{
			name: "missing APIToken",
			cfg: Config{
				APIBaseURL:     "https://bark.internal",
				SigningKeyPath: "/keys/cosign.key",
				PolicyPath:     "/policy/license-policy.yaml",
			},
			wantErr: true,
		},
		{
			name: "missing SigningKeyPath",
			cfg: Config{
				APIBaseURL: "https://bark.internal",
				APIToken:   "tok",
				PolicyPath: "/policy/license-policy.yaml",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.cfg.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequest_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     Request
		wantErr bool
	}{
		{
			name:    "valid request",
			req:     Request{BottlePath: "/tmp/tool.tar.gz", Name: "tool", Version: "1.0.0"},
			wantErr: false,
		},
		{
			name:    "missing BottlePath",
			req:     Request{Name: "tool", Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "missing Name",
			req:     Request{BottlePath: "/tmp/tool.tar.gz", Version: "1.0.0"},
			wantErr: true,
		},
		{
			name:    "missing Version",
			req:     Request{BottlePath: "/tmp/tool.tar.gz", Name: "tool"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.req.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
