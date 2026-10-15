package app

import "testing"

func TestLoadConfig(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("GIN_MODE", "")
	t.Setenv("LOG_LEVEL", "")
	c, err := LoadConfig("8081")
	if err != nil || c.Port != "8081" || c.Mode != "release" {
		t.Fatalf("%+v %v", c, err)
	}
	for _, tc := range []struct{ key, value string }{{"PORT", "0"}, {"PORT", "abc"}, {"PORT", "65536"}, {"GIN_MODE", "invalid"}, {"LOG_LEVEL", "invalid"}} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if _, err := LoadConfig("8081"); err == nil {
				t.Fatal("expected config error")
			}
		})
	}
	t.Setenv("PORT", "9000")
	t.Setenv("GIN_MODE", "debug")
	t.Setenv("LOG_LEVEL", "debug")
	if c, err := LoadConfig("8081"); err != nil || c.Port != "9000" {
		t.Fatalf("%+v %v", c, err)
	}
}
