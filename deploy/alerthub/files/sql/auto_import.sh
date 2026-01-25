#!/bin/bash

# SQL 导入开关控制
# 通过环境变量来控制是否导入各个 SQL 文件

# 从环境变量获取开关值，默认都为 1（导入）
IMPORT_NOTICE_TEMPLATE=${IMPORT_NOTICE_TEMPLATE:-1}
IMPORT_RULE_TEMPLATE_GROUPS=${IMPORT_RULE_TEMPLATE_GROUPS:-1}
IMPORT_RULE_TEMPLATES=${IMPORT_RULE_TEMPLATES:-1}
IMPORT_TENANTS=${IMPORT_TENANTS:-1}
IMPORT_TENANTS_LINKED_USERS=${IMPORT_TENANTS_LINKED_USERS:-1}

# 导入函数
import_sql() {
    local switch=$1
    local file=$2
    local desc=$3

    if [ "$switch" = "1" ]; then
        echo "正在导入: $desc ($file)"
        mysql -h ${MYSQL_HOST} -u ${MYSQL_USERNAME} -p${MYSQL_PASSWORD} --default-character-set=utf8mb4 -D ${MYSQL_DATABASE} < /sql/$file
        if [ $? -eq 0 ]; then
            echo "✓ $desc 导入成功"
        else
            echo "✗ $desc 导入失败"
            exit 1
        fi
    else
        echo "⊘ 跳过: $desc (已禁用)"
    fi
}

echo "开始导入 SQL 数据..."
echo "================================"

# 导入各个 SQL 文件
import_sql "$IMPORT_NOTICE_TEMPLATE" "notice_template_examples.sql" "通知模板示例"
import_sql "$IMPORT_RULE_TEMPLATE_GROUPS" "rule_template_groups.sql" "规则模板组"
import_sql "$IMPORT_RULE_TEMPLATES" "rule_templates.sql" "规则模板"
import_sql "$IMPORT_TENANTS" "tenants.sql" "租户"
import_sql "$IMPORT_TENANTS_LINKED_USERS" "tenants_linked_users.sql" "租户关联用户"

echo "================================"
echo "SQL 数据导入完成"
