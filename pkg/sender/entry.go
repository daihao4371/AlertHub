package sender

import (
	"alertHub/internal/ctx"
	"fmt"
	"time"

	"github.com/bytedance/sonic"

	"alertHub/internal/models"

	"github.com/zeromicro/go-zero/core/logc"
)

type (
	// SendParams 定义发送参数
	SendParams struct {
		// 基础
		TenantId string
		EventId  string
		RuleName string
		Severity string
		// 通知
		NoticeType string
		NoticeId   string
		NoticeName string
		// 恢复通知
		IsRecovered bool
		// hook 地址
		Hook string
		// 邮件
		Email models.Email
		// 消息
		Content string
		// 电话号码
		PhoneNumber []string
		// 签名
		Sign string `json:"sign,omitempty"`
	}

	// SendInter 发送通知的接口
	SendInter interface {
		Send(params SendParams) error
		Test(params SendParams) error
	}
)

const RobotTestContent = "🎉 AlertHub 告警通道配置成功！\n\n" +
	"这是一条测试消息，用于验证告警通道是否可正常送达 ✅\n" +
	"如您看到此消息，说明配置已生效，\n" +
	"后续重要告警将第一时间通知您 🚀"

// Sender 发送通知的主函数
func Sender(ctx *ctx.Context, sendParams SendParams) error {
	// 根据通知类型获取对应的发送器
	sender, err := senderFactory(sendParams.NoticeType)
	if err != nil {
		return fmt.Errorf("Send alarm failed, %s", err.Error())
	}

	// 发送通知
	if err := sender.Send(sendParams); err != nil {
		addRecord(ctx, sendParams, 1, sendParams.Content, err.Error())
		return fmt.Errorf("Send alarm failed to %s, err: %s", sendParams.NoticeType, err.Error())
	}

	// 记录成功发送的日志
	addRecord(ctx, sendParams, 0, sendParams.Content, "success")
	logc.Info(ctx.Ctx, fmt.Sprintf("Send alarm to %s success", sendParams.NoticeType))
	return nil
}

// Tester 发送测试消息
func Tester(ctx *ctx.Context, sendParams SendParams) error {
	sender, err := senderFactory(sendParams.NoticeType)
	if err != nil {
		return fmt.Errorf("Send alarm failed, %s", err.Error())
	}

	// 发送通知
	if err := sender.Test(sendParams); err != nil {
		return fmt.Errorf("Test alarm failed to %s, err: %s", sendParams.NoticeType, err.Error())
	}

	return nil
}

// senderFactory 创建发送器的工厂函数
func senderFactory(noticeType string) (SendInter, error) {
	switch noticeType {
	case "Email":
		return NewEmailSender(), nil
	case "FeiShu":
		return NewFeiShuSender(), nil
	case "DingDing":
		return NewDingSender(), nil
	case "WeChat":
		return NewWeChatSender(), nil
	case "CustomHook":
		return NewWebHookSender(), nil
	case "PhoneCall":
		return NewPhoneCallSender(), nil
	case "Slack":
		return NewSlackSender(), nil
	case "SMS":
		return NewSmsSender(), nil
	default:
		return nil, fmt.Errorf("无效的通知类型: %s", noticeType)
	}
}

// addRecord 记录通知发送结果
// 注意：保存记录失败不会影响通知发送的成功，仅记录警告日志
func addRecord(ctx *ctx.Context, sendParams SendParams, status int, msg, errMsg string) {
	record := models.NoticeRecord{
		EventId:  sendParams.EventId,
		Date:     time.Now().Format("2006-01-02"),
		CreateAt: time.Now().Unix(),
		TenantId: sendParams.TenantId,
		RuleName: sendParams.RuleName,
		NType:    sendParams.NoticeType,
		NObj:     sendParams.NoticeName + " (" + sendParams.NoticeId + ")",
		Severity: sendParams.Severity,
		Status:   status,
		AlarmMsg: msg,
		ErrMsg:   errMsg,
	}

	err := ctx.DB.Notice().AddRecord(record)
	if err != nil {
		logc.Error(ctx.Ctx, fmt.Sprintf("添加通知记录失败, err: %s", err.Error()))
	}
}

// GetSendMsg 发送内容
func (s *SendParams) GetSendMsg() map[string]any {
	msg := make(map[string]any)
	if s == nil || s.Content == "" {
		return msg
	}
	err := sonic.Unmarshal([]byte(s.Content), &msg)
	if err != nil {
		logc.Errorf(ctx.Ctx, fmt.Sprintf("发送的内容解析失败, err: %s", err.Error()))
		return msg
	}
	return msg
}
