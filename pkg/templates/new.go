package templates

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
)

type Template struct {
	CardContentMsg string
}

func NewTemplate(ctx *ctx.Context, alert models.AlertCurEvent, notice models.AlertNotice) Template {
	noticeTmpl := ctx.DB.NoticeTmpl().Get(notice.NoticeTmplId)
	switch notice.NoticeType {
	case "FeiShu":
		return Template{CardContentMsg: feishuTemplate(alert, noticeTmpl)}
	case "DingDing":
		// 传递通知对象以便获取关键词配置（如果后续添加）
		return Template{CardContentMsg: dingdingTemplate(alert, noticeTmpl, notice)}
	case "Email":
		return Template{CardContentMsg: emailTemplate(alert, noticeTmpl)}
	case "WeChat":
		return Template{CardContentMsg: wechatTemplate(alert, noticeTmpl)}
	case "PhoneCall":
		return Template{CardContentMsg: phoneCallTemplate(alert, noticeTmpl)}
	case "Slack":
		return Template{slackTemplate(alert, noticeTmpl)}
	}

	return Template{}
}
