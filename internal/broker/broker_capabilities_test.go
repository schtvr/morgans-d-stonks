package broker

import "testing"

type capabilityStub struct{ caps map[Capability]bool }

func (c capabilityStub) Capabilities() map[Capability]bool { return c.caps }

func TestHasCapability(t *testing.T) {
	t.Run("supports capability", func(t *testing.T) {
		if !HasCapability(capabilityStub{caps: map[Capability]bool{CapabilityQuote: true}}, CapabilityQuote) {
			t.Fatal("expected quote capability")
		}
	})

	t.Run("missing capability", func(t *testing.T) {
		if HasCapability(capabilityStub{caps: map[Capability]bool{}}, CapabilityPlaceOrder) {
			t.Fatal("did not expect place order capability")
		}
	})

	t.Run("no capability provider", func(t *testing.T) {
		if HasCapability(struct{}{}, CapabilityQuote) {
			t.Fatal("did not expect capability from non-provider")
		}
	})
}
