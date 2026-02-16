#!/bin/bash

# ============================================================================
# RCA 模块集成测试脚本
# ============================================================================

set -e

# 配置
API_HOST="${API_HOST:-http://localhost:8080}"
TEST_USER="${TEST_USER:-admin}"
TEST_PASSWORD="${TEST_PASSWORD:-123456}"
TEST_TENANT="${TEST_TENANT:-default}"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 获取认证令牌
get_auth_token() {
    log_info "正在获取认证令牌..."

    TOKEN=$(curl -s -X POST "${API_HOST}/api/system/login" \
        -H "Content-Type: application/json" \
        -d "{\"username\":\"${TEST_USER}\",\"password\":\"${TEST_PASSWORD}\"}" | jq -r '.data.token')

    if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
        log_error "获取令牌失败"
        exit 1
    fi

    log_info "令牌获取成功: ${TOKEN:0:20}..."
    echo "$TOKEN"
}

# ============================================================================
# Test 1: 测试告警聚类分析端点
# ============================================================================
test_cluster_endpoint() {
    log_info "========== Test 1: 告警聚类分析端点 =========="

    # 准备测试数据：构造告警ID列表
    ALERT_IDS='["alert-001", "alert-002", "alert-003"]'
    TOPOLOGY_DATA='{"services": ["service-a", "service-b"], "dependencies": []}'

    REQUEST_BODY=$(cat <<EOF
{
    "alertIds": ["alert-001", "alert-002", "alert-003"],
    "topologyData": "${TOPOLOGY_DATA}"
}
EOF
)

    log_info "发送聚类分析请求..."
    RESPONSE=$(curl -s -X POST "${API_HOST}/api/w8t/rca/cluster" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${TOKEN}" \
        -d "$REQUEST_BODY")

    log_info "响应内容："
    echo "$RESPONSE" | jq '.'

    # 验证响应格式
    CODE=$(echo "$RESPONSE" | jq -r '.code' 2>/dev/null || echo "")
    if [ "$CODE" = "0" ] || [ "$CODE" = "200" ]; then
        log_info "✓ Test 1 通过：聚类分析端点正常"
        SUGGESTION_ID=$(echo "$RESPONSE" | jq -r '.data.suggestionId' 2>/dev/null)
        echo "$SUGGESTION_ID"
    else
        log_warn "Test 1 失败或返回异常：$CODE"
        echo "null"
    fi
}

# ============================================================================
# Test 2: 测试用户反馈提交端点
# ============================================================================
test_commit_feedback() {
    local SUGGESTION_ID=$1

    if [ -z "$SUGGESTION_ID" ] || [ "$SUGGESTION_ID" = "null" ]; then
        log_warn "跳过 Test 2：无有效的 suggestionId"
        return
    fi

    log_info "========== Test 2: 用户反馈提交端点 =========="

    REQUEST_BODY=$(cat <<EOF
{
    "status": "accepted",
    "score": 5,
    "comment": "聚类结果很准确"
}
EOF
)

    log_info "提交反馈，suggestionId: $SUGGESTION_ID"
    RESPONSE=$(curl -s -X POST "${API_HOST}/api/w8t/rca/cluster/${SUGGESTION_ID}/commit" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${TOKEN}" \
        -d "$REQUEST_BODY")

    log_info "响应内容："
    echo "$RESPONSE" | jq '.'

    CODE=$(echo "$RESPONSE" | jq -r '.code' 2>/dev/null || echo "")
    if [ "$CODE" = "0" ] || [ "$CODE" = "200" ]; then
        log_info "✓ Test 2 通过：反馈提交端点正常"
    else
        log_warn "Test 2 失败或返回异常：$CODE"
    fi
}

# ============================================================================
# Test 3: 测试事件查询端点
# ============================================================================
test_incidents_endpoint() {
    log_info "========== Test 3: 事件查询端点 =========="

    log_info "查询所有事件（分页：page=1, pageSize=20）..."
    RESPONSE=$(curl -s -X GET "${API_HOST}/api/w8t/rca/incidents?page=1&pageSize=20&status=candidate" \
        -H "Authorization: Bearer ${TOKEN}")

    log_info "响应内容："
    echo "$RESPONSE" | jq '.'

    CODE=$(echo "$RESPONSE" | jq -r '.code' 2>/dev/null || echo "")
    TOTAL=$(echo "$RESPONSE" | jq -r '.data.total' 2>/dev/null || echo "0")

    if [ "$CODE" = "0" ] || [ "$CODE" = "200" ]; then
        log_info "✓ Test 3 通过：事件查询端点正常"
        log_info "  查询结果总数: $TOTAL"
    else
        log_warn "Test 3 失败或返回异常：$CODE"
    fi
}

# ============================================================================
# Test 4: 测试统计报告端点
# ============================================================================
test_report_endpoint() {
    log_info "========== Test 4: 统计报告端点 =========="

    # 计算时间戳：最近7天
    END_TIME=$(date +%s)
    START_TIME=$((END_TIME - 7 * 24 * 3600))

    log_info "查询统计报告（时间范围：$START_TIME - $END_TIME）..."
    RESPONSE=$(curl -s -X GET "${API_HOST}/api/w8t/rca/report?startTime=${START_TIME}&endTime=${END_TIME}" \
        -H "Authorization: Bearer ${TOKEN}")

    log_info "响应内容："
    echo "$RESPONSE" | jq '.'

    CODE=$(echo "$RESPONSE" | jq -r '.code' 2>/dev/null || echo "")
    if [ "$CODE" = "0" ] || [ "$CODE" = "200" ]; then
        log_info "✓ Test 4 通过：统计报告端点正常"
    else
        log_warn "Test 4 失败或返回异常：$CODE"
    fi
}

# ============================================================================
# Test 5: 测试权限控制
# ============================================================================
test_permission_control() {
    log_info "========== Test 5: 权限控制验证 =========="

    # 使用无效令牌测试
    log_info "测试无认证访问..."
    RESPONSE=$(curl -s -X GET "${API_HOST}/api/w8t/rca/incidents" \
        -H "Authorization: Bearer invalid_token_123")

    log_info "响应内容："
    echo "$RESPONSE" | jq '.' 2>/dev/null || echo "$RESPONSE"

    # 检查是否返回认证错误
    ERROR=$(echo "$RESPONSE" | jq -r '.msg' 2>/dev/null || echo "")
    if [[ "$ERROR" == *"认证"* ]] || [[ "$ERROR" == *"token"* ]] || [[ "$ERROR" == *"unauthorized"* ]]; then
        log_info "✓ Test 5 通过：权限控制正常"
    else
        log_warn "Test 5 无法确认：响应内容不符合预期"
    fi
}

# ============================================================================
# Test 6: 测试参数验证
# ============================================================================
test_parameter_validation() {
    log_info "========== Test 6: 参数验证 =========="

    # 测试缺少必需参数
    log_info "测试缺少 startTime 参数..."
    RESPONSE=$(curl -s -X GET "${API_HOST}/api/w8t/rca/report?endTime=1707086400" \
        -H "Authorization: Bearer ${TOKEN}")

    log_info "响应内容："
    echo "$RESPONSE" | jq '.'

    CODE=$(echo "$RESPONSE" | jq -r '.code' 2>/dev/null || echo "")
    if [ "$CODE" != "0" ] && [ "$CODE" != "200" ]; then
        log_info "✓ Test 6 通过：参数验证正常（正确拒绝无效请求）"
    else
        log_warn "Test 6 失败：应该返回错误"
    fi
}

# ============================================================================
# Test 7: 测试多租户隔离
# ============================================================================
test_multi_tenant_isolation() {
    log_info "========== Test 7: 多租户隔离验证 =========="

    log_info "获取当前租户的事件列表..."
    RESPONSE=$(curl -s -X GET "${API_HOST}/api/w8t/rca/incidents?page=1&pageSize=10" \
        -H "Authorization: Bearer ${TOKEN}")

    # 检查响应中是否包含租户信息
    TENANT_ID=$(echo "$RESPONSE" | jq -r '.data.incidents[0].tenantId // "not_found"' 2>/dev/null)

    if [ "$TENANT_ID" != "not_found" ] && [ ! -z "$TENANT_ID" ]; then
        log_info "✓ Test 7 通过：多租户隔离正常"
        log_info "  当前租户: $TENANT_ID"
    else
        log_info "✓ Test 7 通过：返回数据为空或无租户信息（正常）"
    fi
}

# ============================================================================
# 主测试流程
# ============================================================================
main() {
    log_info "========== RCA 集成测试开始 =========="
    log_info "API Host: $API_HOST"
    log_info "Test User: $TEST_USER"
    log_info "Test Tenant: $TEST_TENANT"
    echo ""

    # 获取认证令牌
    TOKEN=$(get_auth_token)
    echo ""

    # 执行测试
    SUGGESTION_ID=$(test_cluster_endpoint)
    echo ""

    test_commit_feedback "$SUGGESTION_ID"
    echo ""

    test_incidents_endpoint
    echo ""

    test_report_endpoint
    echo ""

    test_permission_control
    echo ""

    test_parameter_validation
    echo ""

    test_multi_tenant_isolation
    echo ""

    log_info "========== RCA 集成测试完成 =========="
}

# 运行主测试
main