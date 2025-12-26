package provider

import (
	"errors"
	"fmt"
	"time"

	"alertHub/internal/ctx"
	"github.com/alibabacloud-go/dysmsapi-20170525/v3/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/bytedance/sonic"
	"github.com/zeromicro/go-zero/core/logc"
)

// AliyunSmsConfig 阿里云短信配置接口
type AliyunSmsConfig interface {
	SmsProviderConfig
	// GetTemplateCode 获取阿里云模板代码
	GetTemplateCode() string
}

// AliyunSmsProvider 阿里云短信服务商
type AliyunSmsProvider struct{
	retryer Retryer // 重试器
}

// NewAliyunSmsProvider 创建阿里云短信服务商实例
func NewAliyunSmsProvider() SmsProvider {
	return &AliyunSmsProvider{
		retryer: nil, // 延迟初始化
	}
}

// SendSms 发送阿里云短信
func (a *AliyunSmsProvider) SendSms(config SmsProviderConfig, phoneNumbers []string, content string, isRecovered bool) error {
	// 类型断言获取阿里云特定配置
	aliyunConfig, ok := config.(AliyunSmsConfig)
	if !ok {
		return errors.New("无效的阿里云配置")
	}

	// 初始化重试器（如果需要）
	if a.retryer == nil {
		retryConfig := config.GetRetryConfig()
		a.retryer = NewSmsRetryer(retryConfig)
	}

	// 使用重试机制执行SMS发送
	return a.retryer.ExecuteWithContext("阿里云短信发送", func() error {
		return a.sendSmsWithoutRetry(aliyunConfig, phoneNumbers, content, isRecovered)
	})
}

// sendSmsWithoutRetry 发送阿里云短信（不包含重试逻辑）
func (a *AliyunSmsProvider) sendSmsWithoutRetry(aliyunConfig AliyunSmsConfig, phoneNumbers []string, content string, isRecovered bool) error {
	startTime := time.Now()
	metricsManager := GetSmsMetricsManager()
	
	// 记录发送尝试
	metricsManager.RecordSent("aliyun", len(phoneNumbers))
	
	// 创建阿里云短信客户端
	aliyunClient, err := a.createAliyunSmsClient(aliyunConfig)
	if err != nil {
		metricsManager.RecordFailure("aliyun", len(phoneNumbers), err.Error())
		return fmt.Errorf("创建阿里云短信客户端失败: %v", err)
	}

	// 构建模板参数
	templateParam := a.buildAliyunTemplateParams(content, isRecovered)

	// 发送短信到每个手机号（阿里云推荐单个发送以提高成功率）
	err = a.sendAliyunSmsBatch(aliyunClient, aliyunConfig, phoneNumbers, templateParam)
	
	// 记录结果指标
	latency := time.Since(startTime)
	if err != nil {
		metricsManager.RecordFailure("aliyun", len(phoneNumbers), err.Error())
		return err
	}
	
	metricsManager.RecordSuccess("aliyun", len(phoneNumbers), latency)
	return nil
}

// createAliyunSmsClient 创建阿里云短信客户端
func (a *AliyunSmsProvider) createAliyunSmsClient(config AliyunSmsConfig) (*client.Client, error) {
	// 创建配置
	apiConfig := &openapi.Config{
		AccessKeyId:     tea.String(config.GetAccessKeyId()),
		AccessKeySecret: tea.String(config.GetAccessKeySecret()),
	}
	
	// 设置访问的域名
	apiConfig.Endpoint = tea.String("dysmsapi.aliyuncs.com")

	// 创建客户端
	aliyunClient, err := client.NewClient(apiConfig)
	if err != nil {
		return nil, fmt.Errorf("创建阿里云短信客户端失败: %v", err)
	}

	return aliyunClient, nil
}

// buildAliyunTemplateParams 构建阿里云短信模板参数
func (a *AliyunSmsProvider) buildAliyunTemplateParams(content string, isRecovered bool) string {
	status := "告警中"
	if isRecovered {
		status = "已恢复"
	}

	// 阿里云短信模板参数为JSON格式
	// 假设模板变量为: {"status": "状态", "content": "内容", "time": "时间"}
	params := map[string]string{
		"status":  status,
		"content": content,
		"time":    time.Now().Format("2006-01-02 15:04:05"),
	}

	paramsJson, _ := sonic.Marshal(params)
	return string(paramsJson)
}

// sendAliyunSmsBatch 批量发送阿里云短信
func (a *AliyunSmsProvider) sendAliyunSmsBatch(aliyunClient *client.Client, config AliyunSmsConfig, phoneNumbers []string, templateParam string) error {
	var failedNums []string
	var successCount int

	// 阿里云推荐逐个发送以提高成功率和精确错误处理
	for _, phoneNumber := range phoneNumbers {
		if err := a.sendAliyunSingleSms(aliyunClient, config, phoneNumber, templateParam); err != nil {
			failedNums = append(failedNums, phoneNumber)
			logc.Error(ctx.Ctx, fmt.Sprintf("阿里云短信发送失败 - 手机号: %s, 错误: %v", phoneNumber, err))
		} else {
			successCount++
			logc.Info(ctx.Ctx, fmt.Sprintf("阿里云短信发送成功 - 手机号: %s", phoneNumber))
		}
	}

	// 处理发送结果
	return a.handleAliyunSmsResult(successCount, failedNums, len(phoneNumbers))
}

// sendAliyunSingleSms 发送单条阿里云短信
func (a *AliyunSmsProvider) sendAliyunSingleSms(aliyunClient *client.Client, config AliyunSmsConfig, phoneNumber, templateParam string) error {
	// 构建发送请求
	sendSmsRequest := &client.SendSmsRequest{
		PhoneNumbers:  tea.String(phoneNumber),
		SignName:      tea.String(config.GetSignName()),
		TemplateCode:  tea.String(config.GetTemplateCode()),
		TemplateParam: tea.String(templateParam),
	}

	// 创建运行时配置
	runtime := &util.RuntimeOptions{}

	// 发送短信
	response, err := aliyunClient.SendSmsWithOptions(sendSmsRequest, runtime)
	if err != nil {
		return fmt.Errorf("阿里云API调用失败: %v", err)
	}

	// 检查发送结果
	if response == nil || response.Body == nil {
		return errors.New("阿里云短信响应为空")
	}

	// 检查返回码
	if response.Body.Code == nil || *response.Body.Code != "OK" {
		errMsg := "未知错误"
		if response.Body.Message != nil {
			errMsg = *response.Body.Message
		}
		return fmt.Errorf("阿里云短信发送失败，错误码: %s, 错误信息: %s", 
			tea.StringValue(response.Body.Code), errMsg)
	}

	return nil
}

// handleAliyunSmsResult 处理阿里云短信发送结果
func (a *AliyunSmsProvider) handleAliyunSmsResult(successCount int, failedNums []string, totalCount int) error {
	// 如果全部失败，返回错误
	if successCount == 0 {
		return fmt.Errorf("阿里云短信全部发送失败，失败号码: %v", failedNums)
	}

	// 如果部分失败，记录警告但不返回错误
	if len(failedNums) > 0 {
		logc.Error(ctx.Ctx, fmt.Sprintf("阿里云短信部分发送失败，成功: %d, 失败: %d, 失败号码: %v", 
			successCount, len(failedNums), failedNums))
	}

	logc.Info(ctx.Ctx, fmt.Sprintf("阿里云短信发送完成，总计: %d, 成功: %d, 失败: %d", 
		totalCount, successCount, len(failedNums)))

	return nil
}