package types

type RequestNoticeTemplateCreate struct {
	Name                 string `json:"name"`
	NoticeType           string `json:"noticeType"`
	Description          string `json:"description"`
	Template             string `json:"template"`
	TemplateFiring       string `json:"templateFiring"`
	TemplateRecover      string `json:"templateRecover"`
	EnableFeiShuJsonCard *bool  `json:"enableFeiShuJsonCard"`
	EnableQuickAction    *bool  `json:"enableQuickAction"` // 是否启用快捷操作按钮
	UpdateBy             string `json:"updateBy"`
}

type RequestNoticeTemplateUpdate struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	NoticeType           string `json:"noticeType"`
	Description          string `json:"description"`
	Template             string `json:"template"`
	TemplateFiring       string `json:"templateFiring"`
	TemplateRecover      string `json:"templateRecover"`
	EnableFeiShuJsonCard *bool  `json:"enableFeiShuJsonCard"`
	EnableQuickAction    *bool  `json:"enableQuickAction"` // 是否启用快捷操作按钮
	UpdateBy             string `json:"updateBy"`
}

type RequestNoticeTemplateQuery struct {
	ID         string `json:"id" form:"id"`
	Name       string `json:"name" form:"name"`
	NoticeType string `json:"noticeType" form:"noticeType"`
	Query      string `json:"query" form:"query"`
}
