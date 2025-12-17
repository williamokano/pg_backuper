package rotation

import (
	"time"
)

// TierName represents the different retention tiers
type TierName string

const (
	TierHourly    TierName = "hourly"
	TierDaily     TierName = "daily"
	TierWeekly    TierName = "weekly"
	TierMonthly   TierName = "monthly"
	TierQuarterly TierName = "quarterly"
	TierYearly    TierName = "yearly"
)

// CategorizeTier determines which tier a backup belongs to based on its age
func CategorizeTier(backupTime time.Time, now time.Time) TierName {
	age := now.Sub(backupTime)

	switch {
	case age <= 24*time.Hour:
		return TierHourly
	case age <= 7*24*time.Hour:
		return TierDaily
	case age <= 30*24*time.Hour:
		return TierWeekly
	case age <= 90*24*time.Hour:
		return TierMonthly
	case age <= 365*24*time.Hour:
		return TierQuarterly
	default:
		return TierYearly
	}
}
