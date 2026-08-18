package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFilterJQ(t *testing.T) {
	input := []byte(`[
		{"name": "node", "port": 3000, "project": "golively-app"},
		{"name": "postgres", "port": 5432},
		{"name": "mitmdump", "port": 8081}
	]`)

	t.Run("single scalar result", func(t *testing.T) {
		out, err := filterJQ(input, `.[0].name`)
		if err != nil {
			t.Fatalf("filterJQ error: %v", err)
		}
		if got, want := strings.TrimSpace(string(out)), `"node"`; got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	})

	t.Run("filter by predicate wraps multiple results in an array", func(t *testing.T) {
		out, err := filterJQ(input, `[.[] | select(.port > 4000) | .name]`)
		if err != nil {
			t.Fatalf("filterJQ error: %v", err)
		}
		var names []string
		if err := json.Unmarshal(out, &names); err != nil {
			t.Fatalf("output is not valid JSON: %v\n%s", err, out)
		}
		if len(names) != 2 || names[0] != "postgres" || names[1] != "mitmdump" {
			t.Fatalf("unexpected result: %v", names)
		}
	})

	t.Run("empty result prints empty array", func(t *testing.T) {
		out, err := filterJQ(input, `.[] | select(.port > 99999)`)
		if err != nil {
			t.Fatalf("filterJQ error: %v", err)
		}
		if got, want := strings.TrimSpace(string(out)), `[]`; got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	})

	t.Run("invalid filter returns a parse error", func(t *testing.T) {
		if _, err := filterJQ(input, `.[][`); err == nil {
			t.Fatal("expected an error for an invalid filter")
		} else if !strings.Contains(err.Error(), "invalid --jq filter") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid JSON input returns an error", func(t *testing.T) {
		if _, err := filterJQ([]byte(`{not json`), `.`); err == nil {
			t.Fatal("expected an error for invalid JSON input")
		}
	})
}

func TestParseArgsJQValidation(t *testing.T) {
	if _, err := parseArgs([]string{"--json", "--jq", ".[0].name"}); err != nil {
		t.Fatalf("valid combination rejected: %v", err)
	}
	if _, err := parseArgs([]string{"--jq", ".[0].name"}); err == nil {
		t.Fatal("--jq without --json should be rejected")
	} else if !strings.Contains(err.Error(), "--jq requires --json") {
		t.Fatalf("unexpected error: %v", err)
	}
}
