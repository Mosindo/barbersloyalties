package subscriptions

import "testing"

func TestIsActiveStatus(t *testing.T) {
	if !isActiveStatus(StatusActive) {
		t.Fatal("expected active status to allow access")
	}
	if !isActiveStatus(StatusTrialing) {
		t.Fatal("expected trialing status to allow access")
	}
	if isActiveStatus(StatusPastDue) {
		t.Fatal("expected past_due to deny access")
	}
	if isActiveStatus(StatusInactive) {
		t.Fatal("expected inactive to deny access")
	}
}
