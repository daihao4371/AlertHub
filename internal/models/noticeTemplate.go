package models

type NoticeTemplateExample struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	NoticeType           string `json:"noticeType"`
	Description          string `json:"description"`
	Template             string `json:"template"`
	TemplateFiring       string `json:"templateFiring"`
	TemplateRecover      string `json:"templateRecover"`
	EnableFeiShuJsonCard *bool  `json:"enableFeiShuJsonCard"`
	EnableQuickAction    *bool  `json:"enableQuickAction"` // 是否启用快捷操作按钮（仅飞书和钉钉支持）
	UpdateAt             int64  `json:"updateAt"`
	UpdateBy             string `json:"updateBy"`
	UpdateByRealName     string `json:"updateByRealName" gorm:"-"` // Not persisted, for display only
}
