package provider

import (
	"bytes"
	"net/http"
	"time"
	"alertHub/pkg/tools"
)

const (
	GetHTTPMethod  = "GET"
	PostHTTPMethod = "POST"
)

type HTTPer struct{}

func NewEndpointHTTPer() EndpointFactoryProvider {
	return HTTPer{}
}

func (h HTTPer) Pilot(option EndpointOption) (EndpointValue, error) {
	var (
		res     *http.Response
		err     error
		headers = make(map[string][]string)
	)

	// 开始时间
	start := time.Now()
	switch option.HTTP.Method {
	case GetHTTPMethod:
		res, err = tools.Get(option.HTTP.Header, option.Endpoint, option.Timeout)
		if err != nil {
			// HTTP 请求失败时，返回一个表示失败的 EndpointValue
			// StatusCode 设为 0 表示请求失败（连接失败、DNS解析失败等）
			return convertHTTPerToEndpointValue(HttperInformation{
				Address:    option.Endpoint,
				StatusCode: 0, // 0 表示请求失败
				Latency:    0,
				Headers:    make(map[string][]string),
			}), nil
		}
		headers = res.Header
		defer res.Body.Close()
	case PostHTTPMethod:
		res, err = tools.Post(option.HTTP.Header, option.Endpoint, bytes.NewReader([]byte(option.HTTP.Body)), option.Timeout)
		if err != nil {
			// HTTP 请求失败时，返回一个表示失败的 EndpointValue
			return convertHTTPerToEndpointValue(HttperInformation{
				Address:    option.Endpoint,
				StatusCode: 0, // 0 表示请求失败
				Latency:    0,
				Headers:    make(map[string][]string),
			}), nil
		}
		headers = res.Header
		defer res.Body.Close()
	}
	end := time.Now()
	// 计算请求耗时
	latency := end.Sub(start).Milliseconds()

	return convertHTTPerToEndpointValue(HttperInformation{
		Address:    res.Request.URL.String(),
		StatusCode: float64(res.StatusCode),
		Latency:    float64(latency),
		Headers:    headers,
	}), nil
}

func convertHTTPerToEndpointValue(detail HttperInformation) EndpointValue {
	headers := make(map[string]interface{})
	for k, v := range detail.Headers {
		headers[k] = v[0]
	}

	return EndpointValue{
		"address":    detail.Address,
		"StatusCode": detail.StatusCode,
		"Latency":    detail.Latency,
		"headers":    headers,
	}
}
