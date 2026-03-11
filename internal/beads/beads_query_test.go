package beads

import "testing"

func TestBuildEphemeralQueryExpr_QuotesStringFilters(t *testing.T) {
	opts := ListOptions{
		Label:    "gt:merge-request",
		Status:   "open",
		Priority: -1,
		Parent:   "hq-tp9qk",
		Assignee: "circletest/polecats/bullet",
	}

	got := buildEphemeralQueryExpr(opts)
	want := `ephemeral=true AND label="gt:merge-request" AND status="open" AND parent="hq-tp9qk" AND assignee="circletest/polecats/bullet"`
	if got != want {
		t.Fatalf("buildEphemeralQueryExpr() = %q, want %q", got, want)
	}
}

func TestBuildEphemeralQueryExpr_ConvertsTypeToGtLabel(t *testing.T) {
	opts := ListOptions{Type: "task", Priority: -1}

	got := buildEphemeralQueryExpr(opts)
	want := `ephemeral=true AND label="gt:task"`
	if got != want {
		t.Fatalf("buildEphemeralQueryExpr() = %q, want %q", got, want)
	}
}
