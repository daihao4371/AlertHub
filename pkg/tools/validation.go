package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/zeromicro/go-zero/core/logc"
)

// ValidatePhoneNumbers 验证手机号格式 (中国手机号: 1[3-9]开头的11位数字)
// 返回有效的手机号列表
func ValidatePhoneNumbers(numbers []string) []string {
	var validNumbers []string
	phoneRegex := regexp.MustCompile(`^1[3-9]\d{9}$`)

	for _, number := range numbers {
		// 清理号码格式
		cleanNumber := strings.ReplaceAll(number, " ", "")
		cleanNumber = strings.ReplaceAll(cleanNumber, "-", "")

		if phoneRegex.MatchString(cleanNumber) {
			validNumbers = append(validNumbers, cleanNumber)
		} else {
			logc.Error(nil, fmt.Sprintf("无效的手机号: %s", number))
		}
	}

	return validNumbers
}