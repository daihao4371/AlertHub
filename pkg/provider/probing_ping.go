package provider

import (
	"time"

	"github.com/go-ping/ping"
)

type Pinger struct{}

func NewEndpointPinger() EndpointFactoryProvider {
	return Pinger{}
}

func (p Pinger) Pilot(option EndpointOption) (EndpointValue, error) {
	var detail PingerInformation

	pinger, err := ping.NewPinger(option.Endpoint)
	if err != nil {
		// Ping初始化失败时(比如域名解析失败),返回一个表示失败的结果
		// PacketLoss设为100表示完全丢包(拨测失败)
		return convertPingerToEndpointValues(PingerInformation{
			Address:     option.Endpoint,
			PacketsSent: 0,
			PacketsRecv: 0,
			PacketLoss:  100.0, // 100%丢包表示拨测失败
			Addr:        "",
			IPAddr:      "",
			MinRtt:      0,
			MaxRtt:      0,
			AvgRtt:      0,
		}), nil
	}
	pinger.SetPrivileged(true)

	// 请求次数
	pinger.Count = option.ICMP.Count
	// 请求间隔
	pinger.Interval = time.Second * time.Duration(option.ICMP.Interval)
	// 超时时间
	pinger.Timeout = time.Second * time.Duration(option.Timeout)

	pinger.OnFinish = func(stats *ping.Statistics) {
		detail = PingerInformation{
			Address:     stats.Addr,
			PacketsSent: stats.PacketsSent,
			PacketsRecv: stats.PacketsRecv,
			PacketLoss:  stats.PacketLoss,
			Addr:        stats.Addr,
			IPAddr:      stats.IPAddr.String(),
			MinRtt:      float64(stats.MinRtt.Milliseconds()),
			MaxRtt:      float64(stats.MaxRtt.Milliseconds()),
			AvgRtt:      float64(stats.AvgRtt.Milliseconds()),
		}
	}

	err = pinger.Run()
	if err != nil {
		// Ping执行失败时,返回一个表示失败的结果
		return convertPingerToEndpointValues(PingerInformation{
			Address:     option.Endpoint,
			PacketsSent: option.ICMP.Count,
			PacketsRecv: 0,
			PacketLoss:  100.0, // 100%丢包表示拨测失败
			Addr:        "",
			IPAddr:      "",
			MinRtt:      0,
			MaxRtt:      0,
			AvgRtt:      0,
		}), nil
	}

	return convertPingerToEndpointValues(detail), nil
}

func convertPingerToEndpointValues(detail PingerInformation) EndpointValue {
	return EndpointValue{
		"address":     detail.Address,
		"PacketsSent": detail.PacketsSent,
		"PacketsRecv": detail.PacketsRecv,
		"PacketLoss":  detail.PacketLoss,
		"Addr":        detail.Addr,
		"IPAddr":      detail.IPAddr,
		"MinRtt":      detail.MinRtt,
		"MaxRtt":      detail.MaxRtt,
		"AvgRtt":      detail.AvgRtt,
	}
}
