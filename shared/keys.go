package shared

import "fmt"

const (
	DelayedJobsKey = "delayed_jobs"
)

func AccountBalanceKey(id string) string {
	return fmt.Sprintf("%s:balance", id)
}

func AccountThroughputKey(id string) string {
	return fmt.Sprintf("%s:throughput", id)
}

func AccountActualThroughputKey(id string) string {
	return fmt.Sprintf("%s:actualth", id)
}

func AccountTargetThroughputKey(id string) string {
	return fmt.Sprintf("%s:targetth", id)
}

func AccountHoldKey(id, authID string) string {
	return fmt.Sprintf("%s:hold:%s", id, authID)
}

func AccountCampaignKey(id, authID string) string {
	return fmt.Sprintf("%s:campaign:%s", id, authID)
}

func AccountCampaignsKey(id string) string {
	return fmt.Sprintf("%s:campaigns", id)
}
