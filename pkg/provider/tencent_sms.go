package provider

import (
	"errors"
	"fmt"
	"time"

	"alertHub/internal/ctx"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
	"github.com/zeromicro/go-zero/core/logc"
)

// TencentSmsConfig 腾讯云短信配置接口
type TencentSmsConfig interface {
	SmsProviderConfig
	// GetSdkAppId 获取腾讯云应用ID
	GetSdkAppId() string
	// GetTemplateId 获取腾讯云模板ID
	GetTemplateId() string
}

// TencentSmsProvider 腾讯云短信服务商
type TencentSmsProvider struct{
	retryer Retryer // 重试器
}

// NewTencentSmsProvider 创建腾讯云短信服务商实例
func NewTencentSmsProvider() SmsProvider {
	return &TencentSmsProvider{
		retryer: nil, // 延迟初始化
	}
}

// SendSms 发送腾讯云短信
func (t *TencentSmsProvider) SendSms(config SmsProviderConfig, phoneNumbers []string, content string, isRecovered bool) error {
	// 类型断言获取腾讯云特定配置
	tencentConfig, ok := config.(TencentSmsConfig)
	if !ok {
		return errors.New("无效的腾讯云配置")
	}

	// 初始化重试器（如果需要）
	if t.retryer == nil {
		retryConfig := config.GetRetryConfig()
		t.retryer = NewSmsRetryer(retryConfig)
	}

	// 使用重试机制执行SMS发送
	return t.retryer.ExecuteWithContext("腾讯云短信发送", func() error {
		return t.sendSmsWithoutRetry(tencentConfig, phoneNumbers, content, isRecovered)
	})
}

// sendSmsWithoutRetry 发送腾讯云短信（不包含重试逻辑）
func (t *TencentSmsProvider) sendSmsWithoutRetry(tencentConfig TencentSmsConfig, phoneNumbers []string, content string, isRecovered bool) error {
	startTime := time.Now()
	metricsManager := GetSmsMetricsManager()
	
	// 记录发送尝试
	metricsManager.RecordSent("tencent", len(phoneNumbers))
	
	// 创建腾讯云客户端
	tencentClient, err := t.createTencentSmsClient(tencentConfig)
	if err != nil {
		metricsManager.RecordFailure("tencent", len(phoneNumbers), err.Error())
		return fmt.Errorf("创建腾讯云短信客户端失败: %v", err)
	}

	// 准备模板参数 - 腾讯云短信使用模板参数而非直接文本
	templateParams := t.buildTencentTemplateParams(content, isRecovered)

	// 构建发送请求
	request := t.buildTencentSmsRequest(tencentConfig, phoneNumbers, templateParams)

	// 发送短信
	response, err := tencentClient.SendSms(request)
	if err != nil {
		metricsManager.RecordFailure("tencent", len(phoneNumbers), err.Error())
		return fmt.Errorf("腾讯云短信发送失败: %v", err)
	}

	// 处理发送结果
	err = t.handleTencentSmsResponse(response, phoneNumbers)
	
	// 记录结果指标
	latency := time.Since(startTime)
	if err != nil {
		metricsManager.RecordFailure("tencent", len(phoneNumbers), err.Error())
		return err
	}
	
	metricsManager.RecordSuccess("tencent", len(phoneNumbers), latency)
	return nil
}

// createTencentSmsClient 创建腾讯云短信客户端
func (t *TencentSmsProvider) createTencentSmsClient(config TencentSmsConfig) (*sms.Client, error) {
	// 创建认证信息
	credential := common.NewCredential(config.GetAccessKeyId(), config.GetAccessKeySecret())

	// 创建客户端配置
	clientProfile := profile.NewClientProfile()
	clientProfile.HttpProfile.Endpoint = "sms.tencentcloudapi.com"

	// 创建客户端
	tencentClient, err := sms.NewClient(credential, "ap-beijing", clientProfile)
	if err != nil {
		return nil, fmt.Errorf("创建客户端失败: %v", err)
	}

	return tencentClient, nil
}

// buildTencentTemplateParams 构建腾讯云短信模板参数
func (t *TencentSmsProvider) buildTencentTemplateParams(content string, isRecovered bool) []*string {
	status := "告警中"
	if isRecovered {
		status = "已恢复"
	}

	// 腾讯云短信模板参数（根据实际模板设计）
	// 假设模板为：【{1}】{2} - {3}，时间：{4}
	params := []*string{
		common.StringPtr(status),                                    // {1} 状态
		common.StringPtr(content),                                   // {2} 内容
		common.StringPtr(time.Now().Format("2006-01-02 15:04:05")), // {3} 时间
	}

	return params
}

// buildTencentSmsRequest 构建腾讯云短信发送请求
func (t *TencentSmsProvider) buildTencentSmsRequest(config TencentSmsConfig, phoneNumbers []string, templateParams []*string) *sms.SendSmsRequest {
	request := sms.NewSendSmsRequest()

	// 设置应用ID
	request.SmsSdkAppId = common.StringPtr(config.GetSdkAppId())

	// 设置短信签名
	request.SignName = common.StringPtr(config.GetSignName())

	// 设置模板ID
	request.TemplateId = common.StringPtr(config.GetTemplateId())

	// 设置模板参数
	request.TemplateParamSet = templateParams

	// 设置手机号列表（需要添加国际区号）
	var phoneNumberSet []*string
	for _, number := range phoneNumbers {
		// 中国手机号需要添加+86前缀
		phoneNumberSet = append(phoneNumberSet, common.StringPtr("+86"+number))
	}
	request.PhoneNumberSet = phoneNumberSet

	return request
}

// handleTencentSmsResponse 处理腾讯云短信发送结果
func (t *TencentSmsProvider) handleTencentSmsResponse(response *sms.SendSmsResponse, phoneNumbers []string) error {
	if response == nil || response.Response == nil {
		return errors.New("腾讯云短信响应为空")
	}

	sendStatusSet := response.Response.SendStatusSet
	if len(sendStatusSet) == 0 {
		return errors.New("腾讯云短信发送状态为空")
	}

	var failedNums []string
	var successCount int

	// 检查每个手机号的发送状态
	for i, status := range sendStatusSet {
		if status.Code != nil && *status.Code == "Ok" {
			successCount++
			logc.Info(ctx.Ctx, fmt.Sprintf("腾讯云短信发送成功 - 手机号: %s, SerialNo: %s", 
				phoneNumbers[i], *status.SerialNo))
		} else {
			failedNums = append(failedNums, phoneNumbers[i])
			errMsg := "未知错误"
			if status.Message != nil {
				errMsg = *status.Message
			}
			logc.Error(ctx.Ctx, fmt.Sprintf("腾讯云短信发送失败 - 手机号: %s, 错误: %s", 
				phoneNumbers[i], errMsg))
		}
	}

	// 如果全部失败，返回错误
	if successCount == 0 {
		return fmt.Errorf("腾讯云短信全部发送失败，失败号码: %v", failedNums)
	}

	// 如果部分失败，记录警告但不返回错误
	if len(failedNums) > 0 {
		logc.Error(ctx.Ctx, fmt.Sprintf("腾讯云短信部分发送失败，成功: %d, 失败: %d, 失败号码: %v", 
			successCount, len(failedNums), failedNums))
	}

	return nil
}