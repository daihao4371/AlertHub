#!/bin/bash

# AlertHub告警清理脚本
# 用途: 删除指定指纹的告警和相关缓存

# 配置信息
REDIS_HOST="10.10.217.225"
REDIS_PORT="6380"
REDIS_PASSWORD=""

# 要删除的告警信息
FINGERPRINT="9cb2ca9441c7c19ec4e7a734be36067f"
RULE_NAME="域名过期"

echo "=== AlertHub 告警清理脚本 ==="
echo "指纹: $FINGERPRINT"
echo "规则名称: $RULE_NAME"
echo ""

# 连接Redis的命令前缀
if [ -n "$REDIS_PASSWORD" ]; then
    REDIS_CMD="redis-cli -h $REDIS_HOST -p $REDIS_PORT -a $REDIS_PASSWORD"
else
    REDIS_CMD="redis-cli -h $REDIS_HOST -p $REDIS_PORT"
fi

echo "1. 查找包含该告警的Redis keys..."
ALERT_KEYS=$($REDIS_CMD KEYS "w8t:*:faultCenter:*.events")

echo "找到的告警事件keys:"
echo "$ALERT_KEYS"
echo ""

echo "2. 查找静默规则keys..."
MUTE_KEYS=$($REDIS_CMD KEYS "w8t:*:faultCenter:*.mutes")

echo "找到的静默规则keys:"
echo "$MUTE_KEYS"
echo ""

# 提示用户确认
read -p "是否继续删除? (y/n): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "已取消操作"
    exit 1
fi

echo "3. 开始清理..."

# 对于每个告警事件key,需要删除包含该指纹的hash字段
for key in $ALERT_KEYS; do
    echo "检查 key: $key"
    # 获取hash中所有字段
    FIELDS=$($REDIS_CMD HKEYS "$key")

    for field in $FIELDS; do
        # 获取字段值并检查是否包含该指纹
        VALUE=$($REDIS_CMD HGET "$key" "$field")
        if echo "$VALUE" | grep -q "$FINGERPRINT"; then
            echo "  找到匹配的告警,删除字段: $field"
            $REDIS_CMD HDEL "$key" "$field"
        fi
    done
done

# 删除静默规则(如果存在)
echo "$MUTE_KEYS" | while IFS= read -r mute_key; do
    [ -z "$mute_key" ] && continue
    echo "检查静默规则 key: $mute_key"

    # 先检查key的类型
    KEY_TYPE=$($REDIS_CMD TYPE "$mute_key")

    if [ "$KEY_TYPE" = "list" ]; then
        # List类型处理
        LENGTH=$($REDIS_CMD LLEN "$mute_key")
        if [ "$LENGTH" -gt 0 ]; then
            i=0
            while [ $i -lt "$LENGTH" ]; do
                ITEM=$($REDIS_CMD LINDEX "$mute_key" $i)
                if echo "$ITEM" | grep -q "$FINGERPRINT"; then
                    echo "  找到匹配的静默规则,删除..."
                    $REDIS_CMD LSET "$mute_key" $i "DELETE_ME"
                fi
                i=$((i + 1))
            done
            # 删除所有标记的项
            $REDIS_CMD LREM "$mute_key" 0 "DELETE_ME" > /dev/null
        fi
    elif [ "$KEY_TYPE" = "hash" ]; then
        # Hash类型处理
        echo "  静默规则是hash类型,检查字段..."
        MUTE_FIELDS=$($REDIS_CMD HKEYS "$mute_key")

        for mute_field in $MUTE_FIELDS; do
            MUTE_VALUE=$($REDIS_CMD HGET "$mute_key" "$mute_field")
            if echo "$MUTE_VALUE" | grep -q "$FINGERPRINT"; then
                echo "  找到匹配的静默规则,删除字段: $mute_field"
                $REDIS_CMD HDEL "$mute_key" "$mute_field"
            fi
        done
    else
        echo "  跳过: 不支持的key类型 ($KEY_TYPE)"
    fi
done

echo ""
echo "4. 清理完成!"
echo ""
echo "建议: 同时在数据库中删除相关记录"
echo "SQL示例:"
echo "DELETE FROM w8t_alert_cur_event WHERE fingerprint = '$FINGERPRINT';"
echo "DELETE FROM w8t_alert_silences WHERE comment LIKE '%$FINGERPRINT%';"