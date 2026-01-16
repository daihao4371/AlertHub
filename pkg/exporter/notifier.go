package exporter

import (
	"alertHub/internal/ctx"
	"alertHub/internal/models"
	"alertHub/pkg/sender"
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/zeromicro/go-zero/core/logc"
)

// 常量定义
const (
	reportTitle            = "📊 Exporter 健康巡检报告"
	maxTrendRecords        = 10
	maxFunctionLineCount   = 30
	defaultAvailableRate   = 0.0
	minAvailableRateGood   = 95.0
	minAvailableRateNormal = 80.0
)

// Notifier 通知发送器 - 负责向通知组发送巡检报告
type Notifier struct {
	ctx *ctx.Context
}

// NewNotifier 创建通知发送器实例
func NewNotifier(c *ctx.Context) *Notifier {
	return &Notifier{ctx: c}
}

// SendToNoticeGroups 向通知组发送报告
func (n *Notifier) SendToNoticeGroups(tenantId string, noticeGroups []string, content string) error {
	if len(noticeGroups) == 0 {
		return fmt.Errorf("通知组列表为空")
	}

	results := n.sendToAllGroups(tenantId, noticeGroups, content)
	return n.buildSendResult(results, len(noticeGroups))
}

// sendToAllGroups 向所有通知组发送消息
func (n *Notifier) sendToAllGroups(tenantId string, groups []string, content string) []sendResult {
	results := make([]sendResult, 0, len(groups))

	for _, groupId := range groups {
		result := n.sendToSingleGroup(tenantId, groupId, content)
		results = append(results, result)
	}

	return results
}

// sendResult 发送结果
type sendResult struct {
	groupId string
	success bool
	err     error
}

// sendToSingleGroup 向单个通知组发送消息
func (n *Notifier) sendToSingleGroup(tenantId, groupId, content string) sendResult {
	notice, err := n.ctx.DB.Notice().Get(tenantId, groupId)
	if err != nil {
		logc.Errorf(n.ctx.Ctx, "获取通知对象失败: groupId=%s, err=%v", groupId, err)
		return sendResult{groupId: groupId, success: false, err: err}
	}

	msgBytes, err := n.buildMessage(notice.NoticeType, content)
	if err != nil {
		logc.Errorf(n.ctx.Ctx, "构建消息失败: notice=%s, err=%v", notice.Name, err)
		return sendResult{groupId: groupId, success: false, err: err}
	}

	err = n.sendMessage(tenantId, &notice, msgBytes)
	if err != nil {
		logc.Errorf(n.ctx.Ctx, "发送消息失败: notice=%s, err=%v", notice.Name, err)
		return sendResult{groupId: groupId, success: false, err: err}
	}

	logc.Infof(n.ctx.Ctx, "消息发送成功: notice=%s", notice.Name)
	return sendResult{groupId: groupId, success: true, err: nil}
}

// buildMessage 根据通知类型构建消息
func (n *Notifier) buildMessage(noticeType, content string) ([]byte, error) {
	builder := n.getMessageBuilder(noticeType)
	msgContent := builder.Build(content)
	return sonic.Marshal(msgContent)
}

// getMessageBuilder 获取消息构建器
func (n *Notifier) getMessageBuilder(noticeType string) MessageBuilder {
	builders := map[string]MessageBuilder{
		"DingDing": &DingDingBuilder{notifier: n},
		"FeiShu":   &FeiShuBuilder{notifier: n},
	}

	if builder, exists := builders[noticeType]; exists {
		return builder
	}

	return &DefaultBuilder{}
}

// sendMessage 发送消息到通知渠道
func (n *Notifier) sendMessage(tenantId string, notice *models.AlertNotice, msgBytes []byte) error {
	params := sender.SendParams{
		TenantId:    tenantId,
		EventId:     generateEventId(),
		RuleName:    reportTitle,
		Severity:    "info",
		NoticeType:  notice.NoticeType,
		NoticeId:    notice.Uuid,
		NoticeName:  notice.Name,
		IsRecovered: false,
		Hook:        notice.DefaultHook,
		Email:       notice.Email,
		Content:     string(msgBytes),
		PhoneNumber: notice.PhoneNumber,
		Sign:        notice.DefaultSign,
	}

	return sender.Sender(n.ctx, params)
}

// generateEventId 生成事件 ID
func generateEventId() string {
	return "exporter-report-" + time.Now().Format("20060102150405")
}

// buildSendResult 构建发送结果
func (n *Notifier) buildSendResult(results []sendResult, total int) error {
	successCount := 0
	failedGroups := []string{}

	for _, result := range results {
		if result.success {
			successCount++
		} else {
			failedGroups = append(failedGroups, result.groupId)
		}
	}

	if len(failedGroups) > 0 {
		return fmt.Errorf("发送完成: 成功 %d/%d, 失败的通知组: %v", successCount, total, failedGroups)
	}

	logc.Infof(n.ctx.Ctx, "巡检报告发送完成: 成功 %d/%d", successCount, total)
	return nil
}

// ========== 消息构建器接口 ==========

// MessageBuilder 消息构建器接口
type MessageBuilder interface {
	Build(content string) map[string]interface{}
}

// DingDingBuilder 钉钉消息构建器
type DingDingBuilder struct {
	notifier *Notifier
}

// Build 构建钉钉消息
func (b *DingDingBuilder) Build(content string) map[string]interface{} {
	optimizedContent := b.optimizeContent(content)

	// Add inspection keyword for health check reports to ensure DingDing robot acceptance
	// This ensures the message contains required keywords without affecting other alert functionality
	if strings.Contains(optimizedContent, "健康巡检报告") || strings.Contains(optimizedContent, "巡检时间") {
		optimizedContent = "告警 " + optimizedContent
	}

	return map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"title": reportTitle,
			"text":  optimizedContent,
		},
	}
}

// optimizeContent 优化钉钉内容格式
func (b *DingDingBuilder) optimizeContent(content string) string {
	lines := strings.Split(content, "\n")
	result := []string{}

	for _, line := range lines {
		// 跳过主标题和空行（在内容开始前）
		if len(result) == 0 && b.shouldSkipLine(line) {
			continue
		}

		// 转换标题格式
		convertedLine := b.convertTitle(line)
		result = b.appendWithSpacing(result, convertedLine, line != convertedLine)
	}

	return strings.Join(result, "\n")
}

// shouldSkipLine 判断是否应跳过该行（仅在开始前）
func (b *DingDingBuilder) shouldSkipLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "" || trimmed == "## "+reportTitle
}

// convertTitle 转换 Markdown 标题为钉钉粗体格式
func (b *DingDingBuilder) convertTitle(line string) string {
	titlePrefixes := []string{"#### ", "### "}

	for _, prefix := range titlePrefixes {
		if strings.HasPrefix(line, prefix) {
			return "**" + strings.TrimPrefix(line, prefix) + "**"
		}
	}

	return line
}

// appendWithSpacing 添加行到结果，标题前添加空行
func (b *DingDingBuilder) appendWithSpacing(result []string, line string, isTitle bool) []string {
	// 标题前添加空行（如果结果非空且上一行不是空行）
	if isTitle && len(result) > 0 && result[len(result)-1] != "" {
		result = append(result, "")
	}
	return append(result, line)
}

// FeiShuBuilder 飞书消息构建器
type FeiShuBuilder struct {
	notifier *Notifier
}

// Build 构建飞书消息
func (b *FeiShuBuilder) Build(content string) map[string]interface{} {
	parser := NewContentParser(content)
	elements, hasDown := parser.Parse()

	cardTemplate := "blue"
	if hasDown {
		cardTemplate = "red"
	}

	return map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"config": map[string]interface{}{
				"wide_screen_mode": true,
				"enable_forward":   true,
			},
			"header": map[string]interface{}{
				"template": cardTemplate,
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": reportTitle,
				},
			},
			"elements": elements,
		},
	}
}

// DefaultBuilder 默认消息构建器
type DefaultBuilder struct{}

// Build 构建默认消息
func (b *DefaultBuilder) Build(content string) map[string]interface{} {
	return map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": fmt.Sprintf("# %s\n\n%s", reportTitle, content),
		},
	}
}

// ========== 内容解析器 ==========

// ContentParser 内容解析器
type ContentParser struct {
	lines   []string
	index   int
	hasDown bool
}

// NewContentParser 创建内容解析器
func NewContentParser(content string) *ContentParser {
	return &ContentParser{
		lines:   strings.Split(content, "\n"),
		index:   0,
		hasDown: false,
	}
}

// Parse 解析内容
func (p *ContentParser) Parse() ([]map[string]interface{}, bool) {
	elements := []map[string]interface{}{}

	for p.index < len(p.lines) {
		line := strings.TrimSpace(p.lines[p.index])

		// 跳过标题和空行
		if p.shouldSkip(line) {
			p.index++
			continue
		}

		// 解析不同类型的内容
		section := p.parseSection(line)
		if section != nil {
			elements = append(elements, section...)
			continue
		}

		p.index++
	}

	// 添加底部信息
	elements = append(elements, p.buildFooter()...)

	return elements, p.hasDown
}

// shouldSkip 判断是否应跳过该行
func (p *ContentParser) shouldSkip(line string) bool {
	return line == "" ||
		line == "## "+reportTitle ||
		strings.HasPrefix(line, "**巡检时间**:")
}

// parseSection 解析内容段落
func (p *ContentParser) parseSection(line string) []map[string]interface{} {
	sectionParsers := []struct {
		matcher func(string) bool
		parser  func() []map[string]interface{}
	}{
		{func(l string) bool { return strings.Contains(l, "总体统计") }, p.parseStatisticsSection},
		{func(l string) bool { return strings.Contains(l, "异常 Exporter 列表") }, p.parseDownListSection},
		{func(l string) bool { return strings.Contains(l, "所有 Exporter 运行正常") }, p.parseNormalSection},
		{func(l string) bool { return strings.Contains(l, "异常详情") }, p.parseDetailedSection},
		{func(l string) bool { return strings.Contains(l, "近 7 日趋势") }, p.parseTrendsSection},
	}

	for _, sp := range sectionParsers {
		if sp.matcher(line) {
			return sp.parser()
		}
	}

	return nil
}

// parseStatisticsSection 解析统计信息段落
func (p *ContentParser) parseStatisticsSection() []map[string]interface{} {
	p.index++
	stats := p.extractStatistics()

	if stats == nil {
		return nil
	}

	if stats.DownCount > 0 {
		p.hasDown = true
	}

	return buildStatisticsCard(stats)
}

// extractStatistics 提取统计数据
func (p *ContentParser) extractStatistics() *Statistics {
	stats := &Statistics{}

	for p.index < len(p.lines) {
		line := strings.TrimSpace(p.lines[p.index])

		// 遇到新段落，停止解析
		if strings.HasPrefix(line, "###") {
			break
		}

		// 解析表格行
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "---") {
			p.parseStatisticsRow(line, stats)
		}

		// 解析状态行
		if strings.Contains(line, "**状态**:") {
			stats.Status = extractStatus(line)
		}

		p.index++

		// 遇到空行可能是段落结束
		if line == "" && p.index < len(p.lines) {
			nextLine := strings.TrimSpace(p.lines[p.index])
			if strings.HasPrefix(nextLine, "###") {
				break
			}
		}
	}

	return stats
}

// parseStatisticsRow 解析统计表格行
func (p *ContentParser) parseStatisticsRow(line string, stats *Statistics) {
	parts := strings.Split(line, "|")
	if len(parts) < 3 {
		return
	}

	key := strings.TrimSpace(parts[1])
	rawValue := strings.TrimSpace(parts[2])
	value := cleanValue(rawValue)

	// 使用查找表解析字段
	fieldParsers := map[string]func(string){
		"总数":  func(v string) { stats.TotalCount, _ = parseInteger(v) },
		"正常":  func(v string) { stats.UpCount, _ = parseInteger(v) },
		"异常":  func(v string) { stats.DownCount, _ = parseInteger(v) },
		"未知":  func(v string) { stats.UnknownCount, _ = parseInteger(v) },
		"可用率": func(v string) { stats.AvailabilityRate, _ = parsePercentage(v) },
	}

	for keyword, parser := range fieldParsers {
		if strings.Contains(key, keyword) {
			parser(value)
			return
		}
	}
}

// parseDownListSection 解析异常列表段落
func (p *ContentParser) parseDownListSection() []map[string]interface{} {
	p.index++
	downList := p.extractDownList()

	if len(downList) == 0 {
		return nil
	}

	// 提取错误详情并关联到 downList
	p.extractErrorDetails(downList)

	p.hasDown = true
	return buildDownListCard(downList)
}

// extractDownList 提取异常列表数据
func (p *ContentParser) extractDownList() []DownItem {
	// 跳过表头
	p.skipTableHeader()

	items := []DownItem{}
	for p.index < len(p.lines) {
		line := strings.TrimSpace(p.lines[p.index])

		// 遇到新段落，停止解析
		if line == "" || strings.HasPrefix(line, "###") || strings.HasPrefix(line, "####") {
			break
		}

		// 解析数据行
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "---") {
			if item := parseDownItem(line); item != nil {
				items = append(items, *item)
			}
		}

		p.index++
	}

	return items
}

// extractErrorDetails 提取并关联错误详情到异常列表
func (p *ContentParser) extractErrorDetails(downList []DownItem) {
	// 查找 "🔍 错误详情" 段落
	errorSectionFound := false
	for p.index < len(p.lines) {
		line := strings.TrimSpace(p.lines[p.index])

		// 找到错误详情段落
		if strings.Contains(line, "🔍 错误详情") {
			errorSectionFound = true
			p.index++
			break
		}

		// 遇到新的主段落，停止查找
		if strings.HasPrefix(line, "###") {
			return
		}

		p.index++
	}

	if !errorSectionFound {
		return
	}

	// 解析错误详情
	currentInstance := ""
	currentJob := ""
	errorBuffer := ""
	inCodeBlock := false

	for p.index < len(p.lines) {
		line := strings.TrimSpace(p.lines[p.index])

		// 遇到新的主段落，停止解析
		if strings.HasPrefix(line, "###") {
			// 保存最后一个错误
			if currentInstance != "" && errorBuffer != "" {
				matchAndSetError(downList, currentInstance, currentJob, errorBuffer)
			}
			break
		}

		// 检测实例行：**实例名** (`Job`):
		if strings.HasPrefix(line, "**") && strings.Contains(line, "(`") && strings.HasSuffix(line, "):") {
			// 保存上一个错误
			if currentInstance != "" && errorBuffer != "" {
				matchAndSetError(downList, currentInstance, currentJob, errorBuffer)
			}

			// 解析新的实例信息
			currentInstance, currentJob = parseErrorInstanceLine(line)
			errorBuffer = ""
			inCodeBlock = false
		} else if line == "```" {
			// 代码块开始/结束
			inCodeBlock = !inCodeBlock
		} else if inCodeBlock && line != "" {
			// 收集错误信息
			if errorBuffer != "" {
				errorBuffer += " "
			}
			errorBuffer += line
		}

		p.index++
	}

	// 保存最后一个错误
	if currentInstance != "" && errorBuffer != "" {
		matchAndSetError(downList, currentInstance, currentJob, errorBuffer)
	}
}

// parseErrorInstanceLine 解析错误详情的实例行
// 格式：**10.10.217.225:9100** (`node_exporter`):
func parseErrorInstanceLine(line string) (string, string) {
	// 移除开头的 **
	line = strings.TrimPrefix(line, "**")

	// 找到第一个 ** 和 (`
	instanceEnd := strings.Index(line, "**")
	if instanceEnd == -1 {
		return "", ""
	}

	instance := strings.TrimSpace(line[:instanceEnd])

	// 提取 Job
	jobStart := strings.Index(line, "(`")
	jobEnd := strings.Index(line, "`)")
	if jobStart == -1 || jobEnd == -1 {
		return instance, ""
	}

	job := strings.TrimSpace(line[jobStart+2 : jobEnd])
	return instance, job
}

// matchAndSetError 匹配并设置错误信息到对应的 DownItem
func matchAndSetError(downList []DownItem, instance, job, errorMsg string) {
	for i := range downList {
		// 清理 instance 中的格式标记进行匹配
		itemInstance := strings.ReplaceAll(downList[i].Instance, "**", "")
		itemInstance = strings.ReplaceAll(itemInstance, "`", "")
		itemInstance = strings.TrimSpace(itemInstance)

		itemJob := strings.ReplaceAll(downList[i].Job, "`", "")
		itemJob = strings.TrimSpace(itemJob)

		if itemInstance == instance && itemJob == job {
			downList[i].Error = errorMsg
			return
		}
	}
}

// skipTableHeader 跳过表格头部
func (p *ContentParser) skipTableHeader() {
	for p.index < len(p.lines) {
		line := strings.TrimSpace(p.lines[p.index])
		if strings.HasPrefix(line, "|") && strings.Contains(line, "实例名称") {
			p.index++
			break
		}
		p.index++
	}
}

// parseNormalSection 解析正常运行提示
func (p *ContentParser) parseNormalSection() []map[string]interface{} {
	p.index++
	return []map[string]interface{}{
		{
			"tag": "div",
			"text": map[string]interface{}{
				"tag":     "lark_md",
				"content": "所有 Exporter 运行正常\n\n🎉 本次巡检未发现任何异常，所有 Exporter 均正常运行。",
			},
		},
	}
}

// parseDetailedSection 解析详细列表段落
func (p *ContentParser) parseDetailedSection() []map[string]interface{} {
	p.index++
	return p.extractDetailedList()
}

// extractDetailedList 提取详细列表数据
func (p *ContentParser) extractDetailedList() []map[string]interface{} {
	elements := []map[string]interface{}{}

	for p.index < len(p.lines) {
		line := strings.TrimSpace(p.lines[p.index])

		// 遇到新段落，停止解析
		if strings.HasPrefix(line, "###") {
			break
		}

		// 处理子标题
		if strings.HasPrefix(line, "####") {
			elements = append(elements, createTextElement(line))
			p.index++
			continue
		}

		// 处理表格行
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "---") {
			if elem := parseDetailedRow(line); elem != nil {
				elements = append(elements, elem)
			}
		}

		p.index++
	}

	return elements
}

// parseTrendsSection 解析趋势段落
func (p *ContentParser) parseTrendsSection() []map[string]interface{} {
	p.index++
	trends := p.extractTrends()

	if len(trends) == 0 {
		return nil
	}

	// 添加标题
	title := createTextElement("** 近 7 日趋势**")
	return append([]map[string]interface{}{title}, trends...)
}

// extractTrends 提取趋势数据
func (p *ContentParser) extractTrends() []map[string]interface{} {
	// 跳过表头
	p.skipTableHeader()

	elements := []map[string]interface{}{}
	count := 0

	for p.index < len(p.lines) && count < maxTrendRecords {
		line := strings.TrimSpace(p.lines[p.index])

		// 遇到新段落，停止解析
		if line == "" {
			p.index++
			continue
		}

		if strings.HasPrefix(line, "###") {
			break
		}

		// 解析数据行
		if strings.HasPrefix(line, "|") && !strings.Contains(line, "---") {
			if elem := parseTrendRow(line); elem != nil {
				elements = append(elements, elem)
				count++
			}
		}

		p.index++
	}

	return elements
}

// buildFooter 构建底部信息
func (p *ContentParser) buildFooter() []map[string]interface{} {
	return []map[string]interface{}{
		{"tag": "hr"},
		{
			"tag": "note",
			"elements": []map[string]interface{}{
				{
					"tag":     "lark_md",
					"content": fmt.Sprintf(" **报告时间**: %s\n\n*本报告由 AlertHub Exporter 健康巡检系统自动生成*", time.Now().Format("2006-01-02 15:04:05")),
				},
			},
		},
	}
}

// ========== 数据结构 ==========

// Statistics 统计信息
type Statistics struct {
	TotalCount       int
	UpCount          int
	DownCount        int
	UnknownCount     int
	AvailabilityRate float64
	Status           string
}

// DownItem 异常项
type DownItem struct {
	Index      string
	Instance   string
	Job        string
	Datasource string
	URL        string
	Time       string
	Error      string // 新增错误详情字段
}

// ========== 卡片构建器 ==========

// buildStatisticsCard 构建统计信息卡片
func buildStatisticsCard(stats *Statistics) []map[string]interface{} {
	elements := []map[string]interface{}{}

	// 状态行 - 更突出
	statusIcon := getStatusIcon(stats.Status)
	statusColor := "green"
	if stats.DownCount > 0 {
		statusColor = "red"
	} else if stats.UnknownCount > 0 {
		statusColor = "orange"
	}

	elements = append(elements, createTextElement(
		fmt.Sprintf("%s **状态**: <font color='%s'>%s</font>", statusIcon, statusColor, stats.Status),
	))

	// 第一行：总数和可用率
	row1 := []map[string]interface{}{
		createColumn("总数", fmt.Sprintf("**%d**", stats.TotalCount), ""),
		createColumn("可用率", fmt.Sprintf("**%.1f%%**", stats.AvailabilityRate), getRateColor(stats.AvailabilityRate)),
	}
	elements = append(elements, map[string]interface{}{
		"tag":              "column_set",
		"flex_mode":        "none",
		"background_style": "grey",
		"columns":          row1,
	})

	// 第二行：正常和异常
	row2 := []map[string]interface{}{
		createColumn("正常", fmt.Sprintf("**%d**", stats.UpCount), "green"),
		createColumn("异常", fmt.Sprintf("**%d**", stats.DownCount), "red"),
	}
	elements = append(elements, map[string]interface{}{
		"tag":              "column_set",
		"flex_mode":        "none",
		"background_style": "default",
		"columns":          row2,
	})

	// 添加分隔线，增加视觉层次
	elements = append(elements, map[string]interface{}{"tag": "hr"})

	return elements
}

// getStatusIcon 获取状态图标
func getStatusIcon(status string) string {
	if strings.Contains(status, "异常") {
		return "⚠️"
	}
	if strings.Contains(status, "未知") {
		return "❓"
	}
	return "✅"
}

// buildStatisticsColumns 构建统计列
func buildStatisticsColumns(stats *Statistics) []map[string]interface{} {
	return []map[string]interface{}{
		createColumn("📊 总数", fmt.Sprintf("**%d**", stats.TotalCount), ""),
		createColumn("✅ 正常", fmt.Sprintf("**%d**", stats.UpCount), "green"),
		createColumn("❌ 异常", fmt.Sprintf("**%d**", stats.DownCount), "red"),
		createColumn("📈 可用率", fmt.Sprintf("**%.2f%%**", stats.AvailabilityRate), getRateColor(stats.AvailabilityRate)),
	}
}

// createColumn 创建列元素
func createColumn(title, value, color string) map[string]interface{} {
	content := fmt.Sprintf("**%s**\n\n%s", title, value)
	if color != "" {
		content = fmt.Sprintf("**%s**\n\n<font color='%s'>%s</font>", title, color, value)
	}

	return map[string]interface{}{
		"tag":    "column",
		"width":  "weighted",
		"weight": 1,
		"elements": []map[string]interface{}{
			createTextElement(content),
		},
	}
}

// getRateColor 获取可用率颜色
func getRateColor(rate float64) string {
	if rate >= minAvailableRateGood {
		return "blue"
	}
	if rate >= minAvailableRateNormal {
		return "orange"
	}
	return "red"
}

// buildDownListCard 构建异常列表卡片
func buildDownListCard(downList []DownItem) []map[string]interface{} {
	elements := []map[string]interface{}{
		createTextElement(fmt.Sprintf("**⚠️ 异常 Exporter 列表 (%d)**", len(downList))),
	}

	for idx, item := range downList {
		content := buildDownItemContent(item)
		elements = append(elements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"tag":     "lark_md",
				"content": content,
			},
		})

		// 添加分隔线（除最后一项）
		if idx < len(downList)-1 {
			elements = append(elements, map[string]interface{}{"tag": "hr"})
		}
	}

	return elements
}

// buildDownItemContent 构建异常项内容（每个字段独占一行，美观易读）
func buildDownItemContent(item DownItem) string {
	// 移除截断，完整显示实例名
	instance := item.Instance
	// 清理 markdown 格式
	instance = strings.ReplaceAll(instance, "**", "")
	instance = strings.ReplaceAll(instance, "`", "")

	job := strings.ReplaceAll(item.Job, "`", "")
	datasource := strings.TrimSpace(item.Datasource)

	// 每个字段独占一行
	content := fmt.Sprintf("**%s. 实例**: %s", item.Index, instance)
	content += fmt.Sprintf("\n**Job**: %s", job)
	content += fmt.Sprintf("\n**数据源**: %s", datasource)
	content += fmt.Sprintf("\n**采集时间**: %s", item.Time)

	// 错误详情（如果有）
	if item.Error != "" {
		errorMsg := cleanValue(item.Error)
		// 限制错误信息长度为 150 字符
		if len(errorMsg) > 150 {
			errorMsg = errorMsg[:147] + "..."
		}
		content += fmt.Sprintf("\n**错误详情**: %s", errorMsg)
	}

	return content
}

// createTextElement 创建文本元素
func createTextElement(content string) map[string]interface{} {
	return map[string]interface{}{
		"tag": "div",
		"text": map[string]interface{}{
			"tag":     "lark_md",
			"content": content,
		},
	}
}

// ========== 解析辅助函数 ==========

// cleanValue 清理值中的格式标记
func cleanValue(value string) string {
	value = removeHTMLTags(value)
	value = strings.ReplaceAll(value, "**", "")
	value = strings.ReplaceAll(value, "`", "")
	return strings.TrimSpace(value)
}

// removeHTMLTags 移除 HTML 标签
func removeHTMLTags(s string) string {
	result := strings.Builder{}
	inTag := false

	for _, ch := range s {
		switch ch {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				result.WriteRune(ch)
			}
		}
	}

	return result.String()
}

// extractStatus 提取状态文本
func extractStatus(line string) string {
	if strings.Contains(line, "全部正常") {
		return "全部正常"
	}
	if strings.Contains(line, "异常") {
		return "有异常"
	}
	return "有未知状态"
}

// parseInteger 解析整数
func parseInteger(value string) (int, error) {
	var result int
	_, err := fmt.Sscanf(value, "%d", &result)
	return result, err
}

// parsePercentage 解析百分比
func parsePercentage(value string) (float64, error) {
	var result float64
	_, err := fmt.Sscanf(value, "%f%%", &result)
	return result, err
}

// parseDownItem 解析异常项
func parseDownItem(line string) *DownItem {
	parts := strings.Split(line, "|")
	if len(parts) < 7 {
		return nil
	}

	return &DownItem{
		Index:      strings.TrimSpace(parts[1]),
		Instance:   strings.TrimSpace(parts[2]),
		Job:        strings.TrimSpace(parts[3]),
		Datasource: strings.TrimSpace(parts[4]),
		URL:        strings.TrimSpace(parts[5]),
		Time:       strings.TrimSpace(parts[6]),
	}
}

// parseDetailedRow 解析详细行
func parseDetailedRow(line string) map[string]interface{} {
	parts := strings.Split(line, "|")
	if len(parts) < 4 {
		return nil
	}

	content := fmt.Sprintf("%s. %s (`%s`) - %s",
		strings.TrimSpace(parts[1]),
		strings.TrimSpace(parts[2]),
		strings.TrimSpace(parts[3]),
		strings.TrimSpace(parts[4]))

	return createTextElement(content)
}

// parseTrendRow 解析趋势行
func parseTrendRow(line string) map[string]interface{} {
	parts := strings.Split(line, "|")
	if len(parts) < 6 {
		return nil
	}

	content := fmt.Sprintf(
		"**%s** | %s | 总数: %s | 正常: <font color='green'>%s</font> | 异常: <font color='red'>%s</font> | 可用率: <font color='blue'>%s</font>",
		strings.TrimSpace(parts[1]),
		strings.TrimSpace(parts[2]),
		strings.TrimSpace(parts[3]),
		strings.TrimSpace(parts[4]),
		strings.TrimSpace(parts[5]),
		strings.TrimSpace(parts[6]),
	)

	return createTextElement(content)
}
