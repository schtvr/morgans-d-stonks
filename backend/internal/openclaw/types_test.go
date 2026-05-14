package openclaw

import "testing"

func TestStatusesAreStableForLabAPI(t *testing.T) {
	if StatusQueued != "queued" || StatusRetrying != "retrying" || StatusSkipped != "skipped" {
		t.Fatalf("unexpected status constants")
	}
}
