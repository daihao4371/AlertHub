package services

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/sender"
	"alertHub/pkg/tools"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logc"
)

type noticeService struct {
	ctx *ctx.Context
}

type InterNoticeService interface {
	List(req interface{}) (interface{}, interface{})
	Create(req interface{}) (interface{}, interface{})
	Update(req interface{}) (interface{}, interface{})
	Delete(req interface{}) (interface{}, interface{})
	Get(req interface{}) (interface{}, interface{})
	ListRecord(req interface{}) (interface{}, interface{})
	GetRecordMetric(req interface{}) (interface{}, interface{})
	DeleteRecord(req interface{}) (interface{}, interface{})
	Test(req interface{}) (interface{}, interface{})
}

func newInterAlertNoticeService(ctx *ctx.Context) InterNoticeService {
	return &noticeService{
		ctx,
	}
}

func (n noticeService) List(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestNoticeQuery)
	data, err := n.ctx.DB.Notice().List(r.TenantId, r.NoticeTmplId, r.Query)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (n noticeService) Create(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestNoticeCreate)
	ok := n.ctx.DB.Notice().GetQuota(r.TenantId)
	if !ok {
		return models.AlertNotice{}, fmt.Errorf("创建失败, 配额不足")
	}

	err := n.ctx.DB.Notice().Create(models.AlertNotice{
		TenantId:            r.TenantId,
		Uuid:                "n-" + tools.RandId(),
		Name:                r.Name,
		DutyId:              r.DutyId,
		NoticeType:          r.NoticeType,
		NoticeTmplId:        r.NoticeTmplId,
		DefaultHook:         r.DefaultHook,
		DefaultSign:         r.DefaultSign,
		Routes:              r.Routes,
		Email:               r.Email,
		PhoneNumber:         r.PhoneNumber,
		EnterpriseApiConfig: r.EnterpriseApiConfig,
		InternalSmsConfig:   r.InternalSmsConfig,
		UpdateAt:            time.Now().Unix(),
		UpdateBy:            r.UpdateBy,
	})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (n noticeService) Update(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestNoticeUpdate)
	err := n.ctx.DB.Notice().Update(models.AlertNotice{
		TenantId:            r.TenantId,
		Uuid:                r.Uuid,
		Name:                r.Name,
		DutyId:              r.GetDutyId(),
		NoticeType:          r.NoticeType,
		NoticeTmplId:        r.NoticeTmplId,
		DefaultHook:         r.DefaultHook,
		DefaultSign:         r.DefaultSign,
		Routes:              r.Routes,
		Email:               r.Email,
		PhoneNumber:         r.PhoneNumber,
		EnterpriseApiConfig: r.EnterpriseApiConfig,
		InternalSmsConfig:   r.InternalSmsConfig,
		UpdateAt:            time.Now().Unix(),
		UpdateBy:            r.UpdateBy,
	})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (n noticeService) Delete(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestNoticeQuery)
	err := n.ctx.DB.Notice().Delete(r.TenantId, r.Uuid)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (n noticeService) Get(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestNoticeQuery)
	data, err := n.ctx.DB.Notice().Get(r.TenantId, r.Uuid)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (n noticeService) ListRecord(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestNoticeQuery)
	data, err := n.ctx.DB.Notice().ListRecord(r.TenantId, r.EventId, r.Severity, r.Status, r.Query, r.Page)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (n noticeService) DeleteRecord(req interface{}) (interface{}, interface{}) {
	err := n.ctx.DB.Notice().DeleteRecord()
	if err != nil {
		return nil, err
	}

	return nil, nil
}

type ResponseRecordMetric struct {
	Date   []string `json:"date"`
	Series series   `json:"series"`
}

type series struct {
	P0 []int64 `json:"p0"`
	P1 []int64 `json:"p1"`
	P2 []int64 `json:"p2"`
}

func (n noticeService) GetRecordMetric(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestNoticeQuery)
	curTime := time.Now()
	var layout = "2006-01-02"
	timeList := []string{
		curTime.Add(-144 * time.Hour).Format(layout),
		curTime.Add(-120 * time.Hour).Format(layout),
		curTime.Add(-96 * time.Hour).Format(layout),
		curTime.Add(-72 * time.Hour).Format(layout),
		curTime.Add(-48 * time.Hour).Format(layout),
		curTime.Add(-24 * time.Hour).Format(layout),
		curTime.Format(layout),
	}

	var severitys = []string{"P0", "P1", "P2"}
	var P0, P1, P2 []int64
	for _, t := range timeList {
		for _, s := range severitys {
			count, err := n.ctx.DB.Notice().CountRecord(models.CountRecord{
				Date:     t,
				TenantId: r.TenantId,
				Severity: s,
			})
			if err != nil {
				logc.Error(n.ctx.Ctx, err.Error())
			}
			switch s {
			case "P0":
				P0 = append(P0, count)
			case "P1":
				P1 = append(P1, count)
			case "P2":
				P2 = append(P2, count)
			}

		}
	}

	return ResponseRecordMetric{
		Date: timeList,
		Series: series{
			P0: P0,
			P1: P1,
			P2: P2,
		},
	}, nil
}

func (n noticeService) Test(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestNoticeTest)
	var errList []struct {
		Hook  string
		Error string
	}

	// 对于SMS类型，检查是否配置了短信方式
	if r.NoticeType == "SMS" {
		if r.DefaultHook == "" && r.InternalSmsConfig == nil {
			// 检查路由中是否有配置
			hasConfig := false
			for _, route := range r.Routes {
				if route.Hook != "" || route.InternalSmsConfig != nil {
					hasConfig = true
					break
				}
			}
			if !hasConfig {
				return nil, errors.New("SMS类型通知未配置任何短信方式，请配置DefaultHook（外部短信服务）或InternalSmsConfig（内部短信网关）")
			}
		}
	}

	// 仅当配置了默认Hook或InternalSmsConfig时才测试默认配置
	if r.DefaultHook != "" || r.InternalSmsConfig != nil {
		err := sender.Tester(n.ctx, sender.SendParams{
			NoticeType:          r.NoticeType,
			NoticeName:          r.Name, // 传递通知对象名称，用于识别关键词
			Hook:                r.DefaultHook,
			Email:               r.Email,
			Sign:                r.DefaultSign,
			PhoneNumber:         r.PhoneNumber,
			EnterpriseApiConfig: r.EnterpriseApiConfig,
			InternalSmsConfig:   r.InternalSmsConfig, // 传递内部短信网关配置
		})
		if err != nil {
			errList = append(errList, struct {
				Hook  string
				Error string
			}{Hook: "config-masked", Error: err.Error()})
		}
	}

	for _, route := range r.Routes {
		// 路由策略中的EnterpriseApiConfig优先，如果没有则使用默认配置
		routeEnterpriseApiConfig := route.EnterpriseApiConfig
		if routeEnterpriseApiConfig == nil {
			routeEnterpriseApiConfig = r.EnterpriseApiConfig
		}
		// 路由策略中的InternalSmsConfig优先，如果没有则使用默认配置
		routeInternalSmsConfig := route.InternalSmsConfig
		if routeInternalSmsConfig == nil {
			routeInternalSmsConfig = r.InternalSmsConfig
		}

		// 仅当路由配置了Hook或InternalSmsConfig时才测试
		if route.Hook == "" && routeInternalSmsConfig == nil {
			continue
		}

		err := sender.Tester(n.ctx, sender.SendParams{
			NoticeType: r.NoticeType,
			NoticeName: r.Name, // 传递通知对象名称，用于识别关键词
			Hook:       route.Hook,
			Email: models.Email{
				To: route.To,
				CC: route.CC,
			},
			Sign:                route.Sign,
			PhoneNumber:         route.To, // 短信发送时使用To字段作为电话号码
			EnterpriseApiConfig: routeEnterpriseApiConfig,
			InternalSmsConfig:   routeInternalSmsConfig, // 传递内部短信网关配置
		})
		if err != nil {
			errList = append(errList, struct {
				Hook  string
				Error string
			}{Hook: "route-config-masked", Error: err.Error()})
		}
	}

	if len(errList) != 0 {
		return nil, errors.New(tools.JsonMarshalToString(errList))
	}

	return nil, nil
}
