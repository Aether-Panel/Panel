package models

import (
	"strings"
	"testing"
)

func TestIsValidServerIdentifier(t *testing.T) {
	valid := []string{
		"1A2B3C4D",
		"abc",
		"ABC123",
		"a-b_c",
		"0123456789abcdefghij",
		strings.Repeat("A", 20),
	}
	for _, id := range valid {
		if !IsValidServerIdentifier(id) {
			t.Errorf("expected %q to be a valid identifier", id)
		}
	}

	invalid := []string{
		"",
		"-leadingdash",
		"_leadingunderscore",
		"has space",
		"has/slash",
		"has.dot",
		"has\\back",
		"caf\u00e9",
		"tab\tchar",
		strings.Repeat("A", 21),
	}
	for _, id := range invalid {
		if IsValidServerIdentifier(id) {
			t.Errorf("expected %q to be an invalid identifier", id)
		}
	}
}

func TestGenerateServerIdentifier(t *testing.T) {
	const runs = 2000
	seen := make(map[string]struct{}, runs)

	for i := 0; i < runs; i++ {
		id, err := GenerateServerIdentifier()
		if err != nil {
			t.Fatalf("GenerateServerIdentifier returned error: %v", err)
		}

		if len(id) != 12 {
			t.Fatalf("expected 12 characters, got %d (%q)", len(id), id)
		}
		if !IsValidServerIdentifier(id) {
			t.Fatalf("generated identifier %q is not valid", id)
		}
		if id != strings.ToUpper(id) {
			t.Fatalf("expected uppercase identifier, got %q", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate identifier generated: %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestGenerateInternalID(t *testing.T) {
	const runs = 2000
	seen := make(map[string]struct{}, runs)

	for i := 0; i < runs; i++ {
		id, err := GenerateInternalID()
		if err != nil {
			t.Fatalf("GenerateInternalID returned error: %v", err)
		}
		if len(id) != 36 {
			t.Fatalf("expected 36 characters, got %d (%q)", len(id), id)
		}
		if strings.Count(id, "-") != 4 {
			t.Fatalf("expected canonical UUID form, got %q", id)
		}
		if id[14] != '7' {
			t.Fatalf("expected a UUIDv7 (version nibble 7), got %q", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate internal id generated: %q", id)
		}
		seen[id] = struct{}{}
	}
}
