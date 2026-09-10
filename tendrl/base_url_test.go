package tendrl

import (
	"os"
	"testing"
)

// The base URL used to be assigned in code, so the SDK could only ever reach
// production and there was no way to exercise it against anything else. These
// pin the resolution order and the shapes accepted, because getting this wrong
// silently sends a customer's data to the wrong host.
func TestBaseURLResolution(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want string
	}{
		{"unset falls back to production", "", "https://app.tendrl.com/api"},
		{"bare origin gains /api", "http://localhost:8000", "http://localhost:8000/api"},
		{"trailing slash is trimmed", "http://localhost:8000/", "http://localhost:8000/api"},
		{"a full base URL is left alone", "http://localhost:8000/api", "http://localhost:8000/api"},
		{"trailing slash after /api", "http://localhost:8000/api/", "http://localhost:8000/api"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TENDRL_APP_URL", tc.env)
			if tc.env == "" {
				os.Unsetenv("TENDRL_APP_URL")
			}
			c, err := NewClient(false, "test-key-not-real")
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			if c.baseURL != tc.want {
				t.Errorf("baseURL = %q, want %q", c.baseURL, tc.want)
			}
		})
	}
}
