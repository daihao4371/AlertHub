package models

import (
	"encoding/json"
	"testing"
	"time"
)

// TestIntelligentAnalysisModels 测试智能分析相关模型的基本功能
func TestIntelligentAnalysisModels(t *testing.T) {
	// 测试 IntelligentAnalysisRecord 基本功能
	t.Run("IntelligentAnalysisRecord", func(t *testing.T) {
		record := &IntelligentAnalysisRecord{
			AiContentRecord: AiContentRecord{
				RuleId:  "test-rule-123",
				Content: "这是AI分析内容",
			},
			AnalysisId:     "analysis-123",
			AnalysisType:   "auto",
			AnalysisMode:   "deep",
			AnalysisStatus: "completed",
			AnalyzedAt:     time.Now().Unix(),
			AnalysisTime:   5000, // 5秒
			ConfidenceScore: 0.85,
		}

		// 测试表名
		if record.TableName() != "w8t_intelligent_analysis_record" {
			t.Errorf("期望表名为 w8t_intelligent_analysis_record，实际为 %s", record.TableName())
		}

		// 测试JSON序列化
		jsonData, err := json.Marshal(record)
		if err != nil {
			t.Errorf("JSON序列化失败: %v", err)
		}

		// 测试JSON反序列化
		var decoded IntelligentAnalysisRecord
		if err := json.Unmarshal(jsonData, &decoded); err != nil {
			t.Errorf("JSON反序列化失败: %v", err)
		}

		// 验证关键字段
		if decoded.AnalysisId != record.AnalysisId {
			t.Errorf("期望 AnalysisId 为 %s，实际为 %s", record.AnalysisId, decoded.AnalysisId)
		}
	})

	// 测试 AlertEventAnalysis 基本功能
	t.Run("AlertEventAnalysis", func(t *testing.T) {
		analysis := &AlertEventAnalysis{
			TenantId:         "tenant-123",
			EventId:          "event-456",
			Fingerprint:      "fp-789",
			AnalysisEnabled:  true,
			AnalysisStatus:   "completed",
			AnalysisResult:   "分析结果内容",
			AnalysisId:       "analysis-123",
			LastAnalysisTime: time.Now().Unix(),
			AnalysisScore:    0.90,
		}

		// 测试JSON序列化/反序列化
		jsonData, err := json.Marshal(analysis)
		if err != nil {
			t.Errorf("JSON序列化失败: %v", err)
		}

		var decoded AlertEventAnalysis
		if err := json.Unmarshal(jsonData, &decoded); err != nil {
			t.Errorf("JSON反序列化失败: %v", err)
		}

		if decoded.AnalysisScore != analysis.AnalysisScore {
			t.Errorf("期望 AnalysisScore 为 %f，实际为 %f", analysis.AnalysisScore, decoded.AnalysisScore)
		}
	})

	// 测试 UniversalAnalysisContext 基本功能
	t.Run("UniversalAnalysisContext", func(t *testing.T) {
		context := &UniversalAnalysisContext{
			ContextId: "ctx-123",
			TenantId:  "tenant-123",
			CreatedAt: time.Now().Unix(),
			AlertInfo: &AlertBasicInfo{
				RuleId:      "rule-123",
				RuleName:    "CPU使用率过高",
				Severity:    "high",
				Fingerprint: "fp-123",
				Labels:      map[string]interface{}{"job": "node-exporter"},
				TriggerTime: time.Now().Unix(),
				Duration:    300, // 5分钟
			},
			Extensions: make(map[string]interface{}),
		}

		// 测试扩展字段
		context.Extensions["customField"] = "customValue"

		// 测试JSON序列化/反序列化
		jsonData, err := json.Marshal(context)
		if err != nil {
			t.Errorf("JSON序列化失败: %v", err)
		}

		var decoded UniversalAnalysisContext
		if err := json.Unmarshal(jsonData, &decoded); err != nil {
			t.Errorf("JSON反序列化失败: %v", err)
		}

		if decoded.AlertInfo.RuleName != context.AlertInfo.RuleName {
			t.Errorf("期望 RuleName 为 %s，实际为 %s", context.AlertInfo.RuleName, decoded.AlertInfo.RuleName)
		}
	})

	// 测试数据质量相关结构
	t.Run("DataQualityInfo", func(t *testing.T) {
		quality := &DataQualityInfo{
			Completeness:  0.95,
			Accuracy:      0.90,
			Timeliness:    0.85,
			TotalPoints:   1000,
			ValidPoints:   950,
			MissingPoints: 50,
			AnomalyPoints: 20,
			QualityScore:  0.87,
			Issues:        []string{"轻微数据缺失", "少量异常点"},
			Metadata:      map[string]interface{}{"source": "prometheus"},
		}

		// 测试JSON序列化
		jsonData, err := json.Marshal(quality)
		if err != nil {
			t.Errorf("JSON序列化失败: %v", err)
		}

		var decoded DataQualityInfo
		if err := json.Unmarshal(jsonData, &decoded); err != nil {
			t.Errorf("JSON反序列化失败: %v", err)
		}

		if decoded.QualityScore != quality.QualityScore {
			t.Errorf("期望 QualityScore 为 %f，实际为 %f", quality.QualityScore, decoded.QualityScore)
		}
	})

	// 测试特征相关结构
	t.Run("UniversalMetricFeatures", func(t *testing.T) {
		features := &UniversalMetricFeatures{
			StatisticalFeatures: &StatisticalFeatures{
				Count:    1000,
				Mean:     75.5,
				Median:   74.2,
				StdDev:   12.3,
				Min:      45.0,
				Max:      95.0,
				Q1:       65.0,
				Q3:       85.0,
				P95:      92.0,
				P99:      94.5,
			},
			DynamicFeatures: map[string]interface{}{
				"customMetric": 123.45,
			},
		}

		// 测试JSON序列化/反序列化
		jsonData, err := json.Marshal(features)
		if err != nil {
			t.Errorf("JSON序列化失败: %v", err)
		}

		var decoded UniversalMetricFeatures
		if err := json.Unmarshal(jsonData, &decoded); err != nil {
			t.Errorf("JSON反序列化失败: %v", err)
		}

		if decoded.StatisticalFeatures.Mean != features.StatisticalFeatures.Mean {
			t.Errorf("期望 Mean 为 %f，实际为 %f", features.StatisticalFeatures.Mean, decoded.StatisticalFeatures.Mean)
		}
	})
}

// TestAlertCurEventIntelligentAnalysis 测试 AlertCurEvent 的智能分析扩展
func TestAlertCurEventIntelligentAnalysis(t *testing.T) {
	// 创建一个包含智能分析的告警事件
	alertEvent := &AlertCurEvent{
		TenantId:     "tenant-123",
		EventId:      "event-456",
		RuleId:       "rule-789",
		RuleName:     "内存使用率告警",
		Fingerprint:  "fp-abc123",
		Severity:     "high",
		Labels:       map[string]interface{}{"service": "web-server", "instance": "server-01"},
		Status:       StateAlerting,
		IntelligentAnalysis: &AlertEventAnalysis{
			TenantId:         "tenant-123",
			EventId:          "event-456",
			Fingerprint:      "fp-abc123",
			AnalysisEnabled:  true,
			AnalysisStatus:   "completed",
			AnalysisResult:   "检测到内存使用率持续上升，可能存在内存泄漏。建议检查应用程序堆内存使用情况。",
			AnalysisId:       "analysis-789",
			LastAnalysisTime: time.Now().Unix(),
			AnalysisScore:    0.88,
		},
	}

	// 测试JSON序列化
	jsonData, err := json.Marshal(alertEvent)
	if err != nil {
		t.Errorf("AlertCurEvent JSON序列化失败: %v", err)
	}

	// 测试JSON反序列化
	var decoded AlertCurEvent
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Errorf("AlertCurEvent JSON反序列化失败: %v", err)
	}

	// 验证智能分析字段
	if decoded.IntelligentAnalysis == nil {
		t.Error("期望 IntelligentAnalysis 不为 nil")
		return
	}

	if decoded.IntelligentAnalysis.AnalysisEnabled != true {
		t.Error("期望 AnalysisEnabled 为 true")
	}

	if decoded.IntelligentAnalysis.AnalysisStatus != "completed" {
		t.Errorf("期望 AnalysisStatus 为 completed，实际为 %s", decoded.IntelligentAnalysis.AnalysisStatus)
	}

	// 测试现有字段是否仍然正常
	if decoded.RuleName != alertEvent.RuleName {
		t.Errorf("期望 RuleName 为 %s，实际为 %s", alertEvent.RuleName, decoded.RuleName)
	}

	if decoded.Status != alertEvent.Status {
		t.Errorf("期望 Status 为 %s，实际为 %s", alertEvent.Status, decoded.Status)
	}
}

// TestBackwardCompatibility 测试向后兼容性
func TestBackwardCompatibility(t *testing.T) {
	// 测试没有智能分析字段的原始告警事件
	originalAlert := &AlertCurEvent{
		TenantId:    "tenant-123",
		EventId:     "event-456",
		RuleId:      "rule-789",
		RuleName:    "原始告警规则",
		Fingerprint: "fp-original",
		Severity:    "medium",
		Status:      StatePreAlert,
		// 注意：没有设置 IntelligentAnalysis 字段
	}

	// 测试JSON序列化（智能分析字段应该被忽略或为null）
	jsonData, err := json.Marshal(originalAlert)
	if err != nil {
		t.Errorf("原始告警事件JSON序列化失败: %v", err)
	}

	// 验证序列化后的JSON不会因为新字段而出错
	var decoded AlertCurEvent
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Errorf("原始告警事件JSON反序列化失败: %v", err)
	}

	// 验证现有字段正常
	if decoded.RuleName != originalAlert.RuleName {
		t.Errorf("期望 RuleName 为 %s，实际为 %s", originalAlert.RuleName, decoded.RuleName)
	}

	// 验证新字段为nil（向后兼容）
	if decoded.IntelligentAnalysis != nil {
		t.Error("期望 IntelligentAnalysis 为 nil 以保持向后兼容性")
	}
}

// BenchmarkModelSerialization 性能基准测试
func BenchmarkModelSerialization(b *testing.B) {
	// 创建一个复杂的分析上下文用于性能测试
	context := &UniversalAnalysisContext{
		ContextId: "benchmark-ctx",
		TenantId:  "benchmark-tenant",
		CreatedAt: time.Now().Unix(),
		AlertInfo: &AlertBasicInfo{
			RuleId:      "benchmark-rule",
			RuleName:    "性能测试规则",
			Severity:    "high",
			Fingerprint: "benchmark-fp",
		},
		MetricFeatures: &UniversalMetricFeatures{
			StatisticalFeatures: &StatisticalFeatures{
				Count: 10000,
				Mean:  75.5,
			},
		},
		Extensions: map[string]interface{}{
			"key1": "value1",
			"key2": 12345,
			"key3": []string{"a", "b", "c"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 测试JSON序列化性能
		jsonData, err := json.Marshal(context)
		if err != nil {
			b.Errorf("JSON序列化失败: %v", err)
		}

		// 测试JSON反序列化性能
		var decoded UniversalAnalysisContext
		if err := json.Unmarshal(jsonData, &decoded); err != nil {
			b.Errorf("JSON反序列化失败: %v", err)
		}
	}
}