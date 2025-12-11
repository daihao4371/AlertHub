package provider

import (
	"fmt"
	"time"
	"alertHub/pkg/tools"
)

type Ssler struct{}

func NewEndpointSSLer() EndpointFactoryProvider {
	return Ssler{}
}

func (p Ssler) Pilot(option EndpointOption) (EndpointValue, error) {
	var detail SslInformation

	startTime := time.Now()
	// 发起 HTTPS 请求
	resp, err := tools.Get(nil, "https://"+option.Endpoint, option.Timeout)
	if err != nil {
		// HTTPS请求失败时(比如连接失败、证书错误),返回一个表示失败的结果
		// TimeRemaining设为-1表示拨测失败
		return convertSslerToEndpointValues(SslInformation{
			Address:             option.Endpoint,
			StartTime:           "",
			ExpireTime:          "",
			StartTimeFormatted:  "",
			ExpireTimeFormatted: "",
			TimeRemaining:       -1, // -1表示拨测失败
			TimeRemainingText:   "拨测失败",
			ResponseTime:        0,
		}), nil
	}
	defer resp.Body.Close()

	// 证书为空, 跳过检测
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		// 证书为空时,返回一个表示失败的结果
		return convertSslerToEndpointValues(SslInformation{
			Address:             option.Endpoint,
			StartTime:           "",
			ExpireTime:          "",
			StartTimeFormatted:  "",
			ExpireTimeFormatted: "",
			TimeRemaining:       -1, // -1表示证书为空
			TimeRemainingText:   "证书为空",
			ResponseTime:        float64(time.Since(startTime).Milliseconds()),
		}), nil
	}

	// 获取证书信息
	cert := resp.TLS.PeerCertificates[0]
	notBefore := cert.NotBefore // 证书开始时间
	notAfter := cert.NotAfter   // 证书过期时间
	currentTime := time.Now()

	// 计算剩余有效期(单位:天)
	timeRemaining := int64(notAfter.Sub(currentTime).Hours() / 24)

	// 格式化日期为中文友好格式: "2025年6月16日 星期一 17:41:05"
	startTimeFormatted := formatChineseDate(notBefore)
	expireTimeFormatted := formatChineseDate(notAfter)

	detail = SslInformation{
		Address:             option.Endpoint,
		StartTime:           notBefore.Format("2006-01-02"), // 基础格式(兼容性保留)
		ExpireTime:          notAfter.Format("2006-01-02"),  // 基础格式(兼容性保留)
		StartTimeFormatted:  startTimeFormatted,             // 中文友好格式
		ExpireTimeFormatted: expireTimeFormatted,            // 中文友好格式
		TimeRemaining:       float64(timeRemaining),
		TimeRemainingText:   fmt.Sprintf("%d天", timeRemaining), // 带单位的文本
		ResponseTime:        float64(time.Since(startTime).Milliseconds()),
	}

	return convertSslerToEndpointValues(detail), nil
}

func convertSslerToEndpointValues(detail SslInformation) EndpointValue {
	return EndpointValue{
		"address":             detail.Address,
		"StartTime":           detail.StartTime,
		"ExpireTime":          detail.ExpireTime,
		"StartTimeFormatted":  detail.StartTimeFormatted,  // 中文友好格式的颁发日期
		"ExpireTimeFormatted": detail.ExpireTimeFormatted, // 中文友好格式的截止日期
		"TimeRemaining":       detail.TimeRemaining,
		"TimeRemainingText":   detail.TimeRemainingText, // 添加带单位的文本字段
		"ResponseTime":        detail.ResponseTime,
	}
}

// formatChineseDate 将时间格式化为中文友好格式: "2025年6月16日 星期一 17:41:05"
func formatChineseDate(t time.Time) string {
	// 星期映射
	weekdays := map[time.Weekday]string{
		time.Sunday:    "星期日",
		time.Monday:    "星期一",
		time.Tuesday:   "星期二",
		time.Wednesday: "星期三",
		time.Thursday:  "星期四",
		time.Friday:    "星期五",
		time.Saturday:  "星期六",
	}

	// 格式化为: "2025年6月16日 星期一 17:41:05"
	return fmt.Sprintf("%d年%d月%d日 %s %s",
		t.Year(),
		t.Month(),
		t.Day(),
		weekdays[t.Weekday()],
		t.Format("15:04:05"),
	)
}
