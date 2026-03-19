package main

import "testing"

func TestPickNodeFromPTable(t *testing.T) {
	if got := pickNodeFromPTable(nil); got != "" {
		t.Fatalf("expected empty string for nil slice, got %q", got)
	}

	if got := pickNodeFromPTable([]string{}); got != "" {
		t.Fatalf("expected empty string for empty slice, got %q", got)
	}

	if got := pickNodeFromPTable([]string{"", "", "127.0.0.1:5050"}); got != "127.0.0.1:5050" {
		t.Fatalf("expected the only non-empty node, got %q", got)
	}

	got := pickNodeFromPTable([]string{"127.0.0.1:5051", "127.0.0.1:5050"})
	if got != "127.0.0.1:5051" && got != "127.0.0.1:5050" {
		t.Fatalf("unexpected node picked: %q", got)
	}
}
