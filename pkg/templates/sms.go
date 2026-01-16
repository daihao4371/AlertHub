package templates

import "alertHub/internal/models"

// smsTemplate 短信模板处理函数
// 使用 ParserTemplate 解析通知模板中的 template 字段，生成短信内容
func smsTemplate(alert models.AlertCurEvent, noticeTmpl models.NoticeTemplateExample) string {
	return ParserTemplate("Event", alert, noticeTmpl.Template)
}
