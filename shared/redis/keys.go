package redis

import "fmt"

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

// TODO potentially change so that it's a list on each campagin rather than global
// However if you do that it becomes slower on the worker as youll need to do a scan
// IDK more thought needed
//func AccountCampaignHistory(accountKey, campaignKey string) string {
//	//return fmt.Sprintf("%s:campaign:%s:history", accountKey, campaignKey)
//	return "historicalrecords"
//}

func BadHistoryKey() string {
	return "historicalrecords"
}

func WebsiteKey(websiteKey string) string {
	return fmt.Sprintf("website:%s", websiteKey)
}
