package cmd

import (
	"testing"
)

func TestGetEnvString(t *testing.T) {
	t.Setenv("TEST_DIFFM_STR", "custom_val")
	if got := getEnvString("TEST_DIFFM_STR", "fallback"); got != "custom_val" {
		t.Fatalf("expected custom_val, got %s", got)
	}

	if got := getEnvString("TEST_DIFFM_UNSET", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %s", got)
	}

	t.Setenv("TEST_DIFFM_EMPTY", "   ")
	if got := getEnvString("TEST_DIFFM_EMPTY", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback for whitespace string, got %s", got)
	}
}

func TestGetEnvInt(t *testing.T) {
	t.Setenv("TEST_DIFFM_INT", "42")
	if got := getEnvInt("TEST_DIFFM_INT", 10); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}

	if got := getEnvInt("TEST_DIFFM_UNSET", 10); got != 10 {
		t.Fatalf("expected fallback 10, got %d", got)
	}

	t.Setenv("TEST_DIFFM_INVALID", "not-a-number")
	if got := getEnvInt("TEST_DIFFM_INVALID", 10); got != 10 {
		t.Fatalf("expected fallback 10 for invalid int, got %d", got)
	}
}

func TestGetEnvBool(t *testing.T) {
	truthyCases := []string{"1", "true", "True", "TRUE", "t", "T", "yes", "YES", "y"}
	for _, tc := range truthyCases {
		t.Setenv("TEST_DIFFM_BOOL", tc)
		if got := getEnvBool("TEST_DIFFM_BOOL", false); !got {
			t.Errorf("expected true for %q, got false", tc)
		}
	}

	falsyCases := []string{"0", "false", "False", "FALSE", "f", "F", "no", "NO", "n"}
	for _, tc := range falsyCases {
		t.Setenv("TEST_DIFFM_BOOL", tc)
		if got := getEnvBool("TEST_DIFFM_BOOL", true); got {
			t.Errorf("expected false for %q, got true", tc)
		}
	}

	t.Setenv("TEST_DIFFM_BOOL", "invalid")
	if got := getEnvBool("TEST_DIFFM_BOOL", true); !got {
		t.Fatalf("expected fallback true for invalid value, got %v", got)
	}
	if got := getEnvBool("TEST_DIFFM_UNSET", false); got {
		t.Fatalf("expected fallback false for unset value, got %v", got)
	}
}
