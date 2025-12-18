package models

type DingMsg struct {
	Msgtype    string      `json:"msgtype"`
	Markdown   *Markdown   `json:"markdown,omitempty"`   // 使用指针类型以便 omitempty 生效
	At         *At         `json:"at,omitempty"`         // 使用指针类型以便 omitempty 生效
	ActionCard *ActionCard `json:"actionCard,omitempty"` // ActionCard模式
}

type Markdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type At struct {
	AtMobiles []string `json:"atMobiles"`
	AtUserIds []string `json:"atUserIds"`
	IsAtAll   bool     `json:"isAtAll"`
}

// ActionCard 钉钉ActionCard消息
// 官方文档: https://open.dingtalk.com/document/robots/custom-robot-access
type ActionCard struct {
	Title          string          `json:"title"`          // 首屏会话透出的展示内容
	Text           string          `json:"text"`           // markdown格式的消息
	BtnOrientation string          `json:"btnOrientation"` // 0:横向 1:纵向
	Btns           []ActionCardBtn `json:"btns"`           // 按钮列表
}

// ActionCardBtn ActionCard按钮
type ActionCardBtn struct {
	Title     string `json:"title"`     // 按钮标题
	ActionURL string `json:"actionURL"` // 点击按钮触发的URL
}
