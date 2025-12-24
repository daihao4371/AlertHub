package types

// RequestDashboardStatistics 首页统计请求
type RequestDashboardStatistics struct {
	TenantId      string `form:"tenantId"`
	FaultCenterId string `form:"faultCenterId"`
}

// ResponseDashboardStatistics 首页统计响应
type ResponseDashboardStatistics struct {
	// 今日数据
	TodayMainAlerts  int64   `json:"todayMainAlerts"`  // 今日主告警
	TodayNewAlerts   int64   `json:"todayNewAlerts"`   // 今日新有告警
	
	// 过去7天数据
	Past7DaysAllEvents   int64   `json:"past7DaysAllEvents"`   // 过去7天所有事件
	Past7DaysMainAlerts  int64   `json:"past7DaysMainAlerts"`  // 过去7天主告警
	Past7DaysMTTA        float64 `json:"past7DaysMTTA"`        // 过去7天MTTA（分钟）
	Past7DaysMTTR        float64 `json:"past7DaysMTTR"`        // 过去7天MTTR（分钟）
	
	// 环比数据
	TodayMainAlertsCompareRatio       float64 `json:"todayMainAlertsCompareRatio"`       // 今日主告警环比
	TodayNewAlertsCompareRatio        float64 `json:"todayNewAlertsCompareRatio"`        // 今日新告警环比
	Past7DaysAllEventsCompareRatio    float64 `json:"past7DaysAllEventsCompareRatio"`    // 过去7天所有事件环比
	Past7DaysMainAlertsCompareRatio   float64 `json:"past7DaysMainAlertsCompareRatio"`   // 过去7天主告警环比
	Past7DaysMTTACompareRatio         float64 `json:"past7DaysMTTACompareRatio"`         // 过去7天MTTA环比
	Past7DaysMTTRCompareRatio         float64 `json:"past7DaysMTTRCompareRatio"`         // 过去7天MTTR环比
}