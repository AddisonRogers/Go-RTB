package shared

import "fmt"

const (
	DelayedJobsKey = "delayed_jobs"
)

func CampaignBalanceKey(accountKey string, campaignKey string) string {
	return fmt.Sprintf("%s:campaign:%s:balance", accountKey, campaignKey)
}

func CampaignActualThroughputKey(accountKey string, campaignKey string) string {
	return fmt.Sprintf("%s:campaign:%s:actualth", accountKey, campaignKey)
}

func CampaignTargetThroughputKey(accountKey string, campaignKey string) string {
	return fmt.Sprintf("%s:campaign:%s:targetth", accountKey, campaignKey)
}

func CampaignHoldKey(accountKey, campaignKey, authID string) string {
	return fmt.Sprintf("%s:campaign:%s:hold:%s", accountKey, campaignKey, authID)
}

func AccountCampaignKey(accountKey, campaignKey string) string {
	return fmt.Sprintf("%s:campaign:%s", accountKey, campaignKey)
}
