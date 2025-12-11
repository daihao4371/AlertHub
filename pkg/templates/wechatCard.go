package templates

import (
	models2 "alertHub/internal/models"
	"alertHub/pkg/tools"
)

func wechatTemplate(alert models2.AlertCurEvent, noticeTmpl models2.NoticeTemplateExample) string {
	Title := ParserTemplate("Title", alert, noticeTmpl.Template)
	Footer := ParserTemplate("Footer", alert, noticeTmpl.Template)

	t := models2.WeChatMsgTemplate{
		MsgType: "markdown",
		MarkDown: models2.WeChatMarkDown{
			Content: "**" + Title + "**" +
				"\n" + "\n" +
				ParserTemplate("Event", alert, noticeTmpl.Template) +
				"\n" +
				Footer,
		},
	}

	return tools.JsonMarshalToString(t)
}
