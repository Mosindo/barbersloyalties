package loyalty

import "testing"

func TestApplyStampCardUnlocksRewardAtThreshold(t *testing.T) {
	nextStamps, nextRewards, nextTotal, unlocked, err := ApplyStampCard(2, 0, 2, 3, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !unlocked {
		t.Fatal("expected reward to be unlocked")
	}
	if nextStamps != 0 {
		t.Fatalf("expected stamps reset to 0, got %d", nextStamps)
	}
	if nextRewards != 1 {
		t.Fatalf("expected available rewards 1, got %d", nextRewards)
	}
	if nextTotal != 3 {
		t.Fatalf("expected total visits 3, got %d", nextTotal)
	}
}

func TestApplyStampCardAccumulatesWhenBelowThreshold(t *testing.T) {
	nextStamps, nextRewards, nextTotal, unlocked, err := ApplyStampCard(1, 2, 5, 4, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if unlocked {
		t.Fatal("did not expect reward unlock")
	}
	if nextStamps != 2 {
		t.Fatalf("expected stamps 2, got %d", nextStamps)
	}
	if nextRewards != 2 {
		t.Fatalf("expected available rewards unchanged, got %d", nextRewards)
	}
	if nextTotal != 6 {
		t.Fatalf("expected total visits 6, got %d", nextTotal)
	}
}

func TestApplyStampCardRejectsInvalidConfig(t *testing.T) {
	_, _, _, _, err := ApplyStampCard(0, 0, 0, 0, 1)
	if err == nil {
		t.Fatal("expected error for invalid threshold")
	}
}
