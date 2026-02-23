package main

import "testing"

func TestParseSnapshotsPreservesMultiWordComments(t *testing.T) {
	input := `Instance    Snapshot    Parent    Comment
vm1         snap1       --        before package update
vm1         snap2       snap1     --
vm1         snap3       snap2     final snapshot`

	got := parseSnapshots(input)
	if len(got) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(got))
	}

	if got[0].Comment != "before package update" {
		t.Fatalf("unexpected comment for snap1: %q", got[0].Comment)
	}
	if got[1].Comment != "" {
		t.Fatalf("expected '--' comment to map to empty string, got %q", got[1].Comment)
	}
	if got[2].Comment != "final snapshot" {
		t.Fatalf("unexpected comment for snap3: %q", got[2].Comment)
	}
}

func TestParseSnapshotLineRequiresMinimumFields(t *testing.T) {
	if _, ok := parseSnapshotLine("invalid"); ok {
		t.Fatalf("expected malformed line to fail parsing")
	}
}
