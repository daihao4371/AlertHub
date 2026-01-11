package services

import (
	"fmt"
	"time"

	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/internal/types"
	"alertHub/pkg/tools"
)

type (
	thirdPartyWebhookService struct {
		ctx *ctx.Context
	}

	InterThirdPartyWebhookService interface {
		Create(req interface{}) (interface{}, interface{})
		Update(req interface{}) (interface{}, interface{})
		Delete(req interface{}) (interface{}, interface{})
		Get(req interface{}) (interface{}, interface{})
		List(req interface{}) (interface{}, interface{})
	}
)

func newInterThirdPartyWebhookService(ctx *ctx.Context) InterThirdPartyWebhookService {
	return &thirdPartyWebhookService{
		ctx: ctx,
	}
}

// Create 创建Webhook配置
func (s thirdPartyWebhookService) Create(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestWebhookCreate)

	// 生成唯一的Webhook ID，确保不重复
	webhookId := tools.GenerateWebhookId()
	for {
		exists, err := s.ctx.DB.ThirdPartyWebhook().ExistsByWebhookId(r.TenantId, webhookId)
		if err != nil {
			return nil, fmt.Errorf("检查Webhook ID唯一性失败: %v", err)
		}
		if !exists {
			break
		}
		// 如果ID已存在，重新生成
		webhookId = tools.GenerateWebhookId()
	}

	// 生成Webhook URL（相对路径格式，前端根据域名拼接完整URL）
	webhookUrl := fmt.Sprintf("/api/webhook/%s", webhookId)

	// 构建Webhook模型
	webhook := &models.ThirdPartyWebhook{
		TenantId:    r.TenantId,
		ID:          tools.RandId(), // 使用现有的ID生成方法
		Name:        r.Name,
		Description: r.Description,
		Source:      r.Source,
		WebhookId:   webhookId,
		WebhookUrl:  webhookUrl,
		DataMapping: r.DataMapping,
		Transform:   r.Transform,
		Status:      string(models.WebhookStatusActive), // 默认启用
		CallCount:   0,
		LastCallAt:  0,
		EnableLog:   r.EnableLog,
		NoticeIds:   r.NoticeIds, // 关联的通知对象ID列表
		CreateAt:    time.Now().Unix(),
		UpdateAt:    time.Now().Unix(),
		CreateBy:    r.CreateBy,
		UpdateBy:    r.CreateBy,
	}

	// 保存到数据库
	err := s.ctx.DB.ThirdPartyWebhook().Create(webhook)
	if err != nil {
		return nil, fmt.Errorf("创建Webhook失败: %v", err)
	}

	// 返回响应
	return types.ResponseWebhook{
		ID:          webhook.ID,
		Name:        webhook.Name,
		Description: webhook.Description,
		Source:      webhook.Source,
		WebhookId:   webhook.WebhookId,
		WebhookUrl:  webhook.WebhookUrl,
		Status:      webhook.Status,
		CallCount:   webhook.CallCount,
		LastCallAt:  webhook.LastCallAt,
		NoticeIds:   webhook.NoticeIds, // 返回通知对象ID列表
		CreateAt:    webhook.CreateAt,
		UpdateAt:    webhook.UpdateAt,
	}, nil
}

// Update 更新Webhook配置
func (s thirdPartyWebhookService) Update(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestWebhookUpdate)

	// 先获取现有配置，确保存在
	existing, err := s.ctx.DB.ThirdPartyWebhook().Get(r.TenantId, r.ID)
	if err != nil {
		return nil, fmt.Errorf("Webhook配置不存在: %v", err)
	}

	// 构建更新模型（保留不可修改的字段）
	webhook := models.ThirdPartyWebhook{
		TenantId:    r.TenantId,
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Source:      r.Source,
		WebhookId:   existing.WebhookId,   // 不可修改
		WebhookUrl:  existing.WebhookUrl,  // 不可修改
		DataMapping: r.DataMapping,
		Transform:   r.Transform,
		Status:      r.Status,
		CallCount:   existing.CallCount,   // 不可修改
		LastCallAt:  existing.LastCallAt,  // 不可修改
		EnableLog:   r.EnableLog,
		NoticeIds:   r.NoticeIds,          // 更新关联的通知对象ID列表
		CreateAt:    existing.CreateAt,    // 不可修改
		UpdateAt:    time.Now().Unix(),
		CreateBy:    existing.CreateBy,    // 不可修改
		UpdateBy:    r.UpdateBy,
	}

	// 更新到数据库
	err = s.ctx.DB.ThirdPartyWebhook().Update(webhook)
	if err != nil {
		return nil, fmt.Errorf("更新Webhook失败: %v", err)
	}

	return nil, nil
}

// Delete 删除Webhook配置
func (s thirdPartyWebhookService) Delete(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestWebhookDelete)

	// 删除Webhook配置
	err := s.ctx.DB.ThirdPartyWebhook().Delete(r.TenantId, r.ID)
	if err != nil {
		return nil, fmt.Errorf("删除Webhook失败: %v", err)
	}

	return nil, nil
}

// Get 获取Webhook配置详情
func (s thirdPartyWebhookService) Get(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestWebhookQuery)

	webhook, err := s.ctx.DB.ThirdPartyWebhook().Get(r.TenantId, r.ID)
	if err != nil {
		return nil, fmt.Errorf("获取Webhook失败: %v", err)
	}

	return types.ResponseWebhook{
		ID:          webhook.ID,
		Name:        webhook.Name,
		Description: webhook.Description,
		Source:      webhook.Source,
		WebhookId:   webhook.WebhookId,
		WebhookUrl:  webhook.WebhookUrl,
		Status:      webhook.Status,
		CallCount:   webhook.CallCount,
		LastCallAt:  webhook.LastCallAt,
		NoticeIds:   webhook.NoticeIds, // 返回通知对象ID列表
		CreateAt:    webhook.CreateAt,
		UpdateAt:    webhook.UpdateAt,
	}, nil
}

// List 查询Webhook配置列表（分页）
func (s thirdPartyWebhookService) List(req interface{}) (interface{}, interface{}) {
	r := req.(*types.RequestWebhookQuery)

	// 查询数据库
	webhooks, total, err := s.ctx.DB.ThirdPartyWebhook().List(
		r.TenantId,
		r.Source,
		r.Status,
		r.Query,
		r.Page,
	)
	if err != nil {
		return nil, fmt.Errorf("查询Webhook列表失败: %v", err)
	}

	// 转换为响应格式
	list := make([]types.ResponseWebhook, 0, len(webhooks))
	for _, webhook := range webhooks {
		list = append(list, types.ResponseWebhook{
			ID:          webhook.ID,
			Name:        webhook.Name,
			Description: webhook.Description,
			Source:      webhook.Source,
			WebhookId:   webhook.WebhookId,
			WebhookUrl:  webhook.WebhookUrl,
			Status:      webhook.Status,
			CallCount:   webhook.CallCount,
			LastCallAt:  webhook.LastCallAt,
			NoticeIds:   webhook.NoticeIds, // 返回通知对象ID列表
			CreateAt:    webhook.CreateAt,
			UpdateAt:    webhook.UpdateAt,
		})
	}

	return types.ResponseWebhookList{
		List:  list,
		Total: total,
		Page:  r.Page,
	}, nil
}
