package loyalty

import "fmt"

func ApplyStampCard(stampsCount, availableRewards, totalPaidVisits, threshold, rewardValue int) (int, int, int, bool, error) {
	if threshold <= 0 {
		return 0, 0, 0, false, fmt.Errorf("stamp threshold must be positive")
	}
	if rewardValue <= 0 {
		return 0, 0, 0, false, fmt.Errorf("reward value must be positive")
	}

	nextTotalPaidVisits := totalPaidVisits + 1
	nextStamps := stampsCount + 1
	nextAvailableRewards := availableRewards
	rewardUnlocked := false

	if nextStamps >= threshold {
		nextAvailableRewards += rewardValue
		nextStamps = 0
		rewardUnlocked = true
	}

	return nextStamps, nextAvailableRewards, nextTotalPaidVisits, rewardUnlocked, nil
}
