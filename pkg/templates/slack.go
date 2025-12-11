package templates

import (
	"alertHub/internal/models"
	"alertHub/pkg/tools"
)

func slackTemplate(alert models.AlertCurEvent, noticeTmpl models.NoticeTemplateExample) string {
	t := models.SlackMsgTemplate{
		Text: ParserTemplate("Event", alert, noticeTmpl.Template),
	}

	return tools.JsonMarshalToString(t)
}
