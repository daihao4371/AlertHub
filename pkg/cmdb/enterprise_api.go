package cmdb

import (
	"alertHub/pkg/tools"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
)

// EnterpriseApiTokenResponse 企业内部API Token响应结构
type EnterpriseApiTokenResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		AccessToken string `json:"access_token"`
	} `json:"data"`
}

// EnterpriseApiAuthHeaders 企业内部API认证请求头
type EnterpriseApiAuthHeaders struct {
	Signature       string `json:"signature"`
	ClientId        string `json:"clientId"`
	Timestamp       string `json:"timestamp"`
	RequestId       string `json:"requestId"`
	SignatureMethod string `json:"signatureMethod"`
	AccessToken     string `json:"accessToken"`
}

// ExtractBaseUrlFromApiUrl 从完整的企业内部API URL中提取基础URL
// 例如: http://open-gateway.prd.bjm6v.belle.lan/dmc-service/dmc/api/msg/enterpriseRobot/receiverSingle
// 返回: http://open-gateway.prd.bjm6v.belle.lan
func ExtractBaseUrlFromApiUrl(apiUrl string) (string, error) {
	parsedURL, err := url.Parse(apiUrl)
	if err != nil {
		return "", fmt.Errorf("解析URL失败: %w", err)
	}

	// 构建基础URL: scheme + host
	baseURL := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)
	return baseURL, nil
}

// GetEnterpriseApiToken 获取企业内部API认证Token
// apiUrl: 企业内部API完整URL（用于提取基础URL）
// clientId: 客户端ID
// clientSecret: 客户端密钥
// 返回accessToken和错误信息
func GetEnterpriseApiToken(apiUrl, clientId, clientSecret string) (string, error) {
	// 从完整URL中提取基础URL
	baseUrl, err := ExtractBaseUrlFromApiUrl(apiUrl)
	if err != nil {
		return "", fmt.Errorf("提取基础URL失败: %w", err)
	}

	// 构建Token获取URL
	tokenURL := fmt.Sprintf("%s/cas/oauth/token?scope=all&grant_type=client_credentials&client_id=%s&client_secret=%s",
		baseUrl, clientId, clientSecret)

	// 发送GET请求获取Token
	res, err := tools.Get(nil, tokenURL, 10)
	if err != nil {
		return "", fmt.Errorf("获取Token请求失败: %w", err)
	}
	defer res.Body.Close()

	// 读取响应体
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("读取Token响应失败: %w", err)
	}

	// 解析响应
	var tokenResp EnterpriseApiTokenResponse
	if err := sonic.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return "", fmt.Errorf("解析Token响应失败: %w, 响应内容: %s", err, string(bodyBytes))
	}

	// 检查响应码
	if tokenResp.Code != 200 {
		return "", fmt.Errorf("获取Token失败: Code=%d, Msg=%s", tokenResp.Code, tokenResp.Msg)
	}

	if tokenResp.Data.AccessToken == "" {
		return "", fmt.Errorf("Token响应中access_token为空")
	}

	return tokenResp.Data.AccessToken, nil
}

// GenerateEnterpriseApiSignature 生成企业内部API签名
// clientId: 客户端ID
// timestamp: 时间戳
// requestId: 请求ID
// accessToken: 访问令牌
// clientSecret: 客户端密钥
// 返回MD5签名的十六进制大写字符串
func GenerateEnterpriseApiSignature(clientId, timestamp, requestId, accessToken, clientSecret string) string {
	// 拼接签名字符串：clientId + timestamp + requestId + accessToken + clientSecret
	signatureStr := clientId + timestamp + requestId + accessToken + clientSecret

	// 计算MD5哈希
	hash := md5.Sum([]byte(signatureStr))

	// 转换为十六进制大写字符串（与Python脚本保持一致：hexdigest().upper()）
	return strings.ToUpper(hex.EncodeToString(hash[:]))
}

// BuildEnterpriseApiAuthHeaders 构建企业内部API认证请求头
// apiUrl: 企业内部API完整URL（用于提取基础URL获取Token）
// clientId: 客户端ID
// clientSecret: 客户端密钥
// accessToken: 访问令牌（如果为空，会自动获取Token）
// 返回认证请求头和错误信息
func BuildEnterpriseApiAuthHeaders(apiUrl, clientId, clientSecret, accessToken string) (*EnterpriseApiAuthHeaders, string, error) {
	// 如果accessToken为空，需要先获取
	var finalAccessToken string
	var err error
	if accessToken == "" {
		finalAccessToken, err = GetEnterpriseApiToken(apiUrl, clientId, clientSecret)
		if err != nil {
			return nil, "", fmt.Errorf("获取Token失败: %w", err)
		}
	} else {
		finalAccessToken = accessToken
	}

	// 生成时间戳和请求ID
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	requestId := timestamp

	// 生成签名
	signature := GenerateEnterpriseApiSignature(clientId, timestamp, requestId, finalAccessToken, clientSecret)

	headers := &EnterpriseApiAuthHeaders{
		Signature:       signature,
		ClientId:        clientId,
		Timestamp:       timestamp,
		RequestId:       requestId,
		SignatureMethod: "MD5",
		AccessToken:     finalAccessToken,
	}

	return headers, finalAccessToken, nil
}

// GenerateBatchNo 生成批次号（UUID，去除横线）
func GenerateBatchNo() string {
	// 生成UUID并去除横线，与Python脚本保持一致
	uuidStr := uuid.New().String()
	return strings.ReplaceAll(uuidStr, "-", "")
}

// EnterpriseApiRequestPayload 企业内部API请求体
type EnterpriseApiRequestPayload struct {
	BatchNo         string `json:"batchNo"`         // 唯一标识（UUID，去除横线）
	MsgType         int    `json:"msgType"`         // 7=markdown, 1=文本
	SecretKey       string `json:"secretKey"`       // 密钥
	BusinessCode    string `json:"businessCode"`    // 业务代码
	RobotCode       string `json:"robotCode"`       // 钉钉机器人Code
	ReceiverAccount string `json:"receiverAccount"` // 接收者账号（多个用逗号分隔）
	ReceiverType    int    `json:"receiverType"`    // 5=钉钉用户ID
	Content         struct {
		Title string `json:"title"` // 告警标题
		Text  string `json:"text"`  // 告警内容
	} `json:"content"`
}

// BuildEnterpriseApiRequestPayload 构建企业内部API请求体
// secretKey: 密钥
// businessCode: 业务代码
// robotCode: 钉钉机器人Code
// receiverAccounts: 接收者账号列表（钉钉ID）
// title: 告警标题
// text: 告警内容
// 返回请求体
func BuildEnterpriseApiRequestPayload(secretKey, businessCode, robotCode string, receiverAccounts []string, title, text string) *EnterpriseApiRequestPayload {
	// 将接收者账号列表转换为逗号分隔的字符串
	receiverAccountStr := ""
	if len(receiverAccounts) > 0 {
		receiverAccountStr = receiverAccounts[0]
		for i := 1; i < len(receiverAccounts); i++ {
			receiverAccountStr += "," + receiverAccounts[i]
		}
	}

	payload := &EnterpriseApiRequestPayload{
		BatchNo:         GenerateBatchNo(),
		MsgType:         7, // 7=markdown
		SecretKey:       secretKey,
		BusinessCode:    businessCode,
		RobotCode:       robotCode,
		ReceiverAccount: receiverAccountStr,
		ReceiverType:    2, // 5=钉钉用户ID
	}
	payload.Content.Title = title
	payload.Content.Text = text

	return payload
}
