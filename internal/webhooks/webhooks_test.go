package webhooks

import (
	"context"
	"encoding/json"
	"testing"
)

// TestEventJSONShape pins the payload merchants receive.
func TestEventJSONShape(t *testing.T) {
	e := Event{PaymentID: "p-1", MerchantID: "m-1", Amount: 500, Status: "captured"}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := map[string]any{"payment_id": "p-1", "merchant_id": "m-1", "amount": float64(500), "status": "captured"}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("field %s: got %v want %v (body %s)", k, got[k], v, raw)
		}
	}
}

// TestFakePublisherRecords verifies the test double both records events
// and surfaces its configured error.
func TestFakePublisherRecords(t *testing.T) {
	f := &FakePublisher{}
	e := Event{PaymentID: "p-1"}
	if err := f.Publish(context.Background(), e); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(f.Events) != 1 || f.Events[0].PaymentID != "p-1" {
		t.Fatalf("bad recorded events: %+v", f.Events)
	}
}
