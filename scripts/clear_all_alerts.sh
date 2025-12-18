#!/bin/bash

# 清空所有活跃告警和历史告警的脚本
# 用于测试环境重置告警数据

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 从配置文件读取连接信息
MYSQL_HOST="10.10.217.225"
MYSQL_PORT="3306"
MYSQL_USER="root"
MYSQL_PASS="123456"
MYSQL_DB="alerthub"

REDIS_HOST="10.10.217.225"
REDIS_PORT="6379"
REDIS_PASS=""

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}  清空所有告警数据 (测试专用)${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""

# 确认操作
read -p "$(echo -e ${RED}警告: 此操作将删除所有活跃告警和历史告警数据,是否继续? [y/N]: ${NC})" confirm
if [[ ! $confirm =~ ^[Yy]$ ]]; then
    echo -e "${GREEN}操作已取消${NC}"
    exit 0
fi

echo ""
echo -e "${YELLOW}[1/6] 清空 Redis 中的活跃告警缓存...${NC}"

# 获取所有故障中心的事件缓存key
redis_keys=$(redis-cli -h $REDIS_HOST -p $REDIS_PORT keys "w8t:*:faultCenter:*.events" 2>/dev/null)

if [ -z "$redis_keys" ]; then
    echo -e "${GREEN}  - Redis中没有活跃告警缓存${NC}"
else
    count=0
    for key in $redis_keys; do
        redis-cli -h $REDIS_HOST -p $REDIS_PORT del "$key" >/dev/null 2>&1
        ((count++))
    done
    echo -e "${GREEN}  - 已清空 $count 个故障中心的活跃告警缓存${NC}"
fi

echo ""
echo -e "${YELLOW}[2/6] 清空数据库中的历史告警数据...${NC}"

# 检查历史告警表是否存在 (尝试两种表名)
table_exists=$(mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASS -D $MYSQL_DB -sse "SHOW TABLES LIKE 'alert_his_events';" 2>/dev/null)

if [ -z "$table_exists" ]; then
    # 尝试旧表名
    table_exists=$(mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASS -D $MYSQL_DB -sse "SHOW TABLES LIKE 'w8t_alert_his_event';" 2>/dev/null)
    table_name="w8t_alert_his_event"
else
    table_name="alert_his_events"
fi

if [ -z "$table_exists" ]; then
    echo -e "${GREEN}  - 历史告警表不存在,跳过${NC}"
else
    # 获取删除前的记录数
    before_count=$(mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASS -D $MYSQL_DB -sse "SELECT COUNT(*) FROM $table_name;" 2>/dev/null)

    # 清空历史告警表
    mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASS -D $MYSQL_DB -e "DELETE FROM $table_name;" 2>/dev/null

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}  - 已删除 $before_count 条历史告警记录 (表: $table_name)${NC}"
    else
        echo -e "${RED}  - 清空历史告警表失败${NC}"
    fi
fi

echo ""
echo -e "${YELLOW}[3/6] 清空数据库中的静默规则...${NC}"

# 获取静默规则数量
silence_count=$(mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASS -D $MYSQL_DB -sse "SELECT COUNT(*) FROM alert_silences;" 2>/dev/null)

if [ -z "$silence_count" ] || [ "$silence_count" -eq 0 ]; then
    echo -e "${GREEN}  - 数据库中没有静默规则${NC}"
else
    # 清空静默规则表
    mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASS -D $MYSQL_DB -e "DELETE FROM alert_silences;" 2>/dev/null

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}  - 已删除 $silence_count 条静默规则${NC}"
    else
        echo -e "${RED}  - 清空静默规则失败${NC}"
    fi
fi

# 清空Redis中的静默规则缓存
silence_keys=$(redis-cli -h $REDIS_HOST -p $REDIS_PORT keys "w8t:*:silence:*" 2>/dev/null)

if [ -z "$silence_keys" ]; then
    echo -e "${GREEN}  - Redis中没有静默规则缓存${NC}"
else
    count=0
    for key in $silence_keys; do
        redis-cli -h $REDIS_HOST -p $REDIS_PORT del "$key" >/dev/null 2>&1
        ((count++))
    done
    echo -e "${GREEN}  - 已清空 $count 个静默规则缓存${NC}"
fi

echo ""
echo -e "${YELLOW}[4/6] 清空拨测任务的独立缓存 (如果存在)...${NC}"

# 清空拨测任务的独立事件缓存
probing_keys=$(redis-cli -h $REDIS_HOST -p $REDIS_PORT keys "w8t:*:probing:*.event" 2>/dev/null)

if [ -z "$probing_keys" ]; then
    echo -e "${GREEN}  - Redis中没有拨测独立缓存${NC}"
else
    count=0
    for key in $probing_keys; do
        redis-cli -h $REDIS_HOST -p $REDIS_PORT del "$key" >/dev/null 2>&1
        ((count++))
    done
    echo -e "${GREEN}  - 已清空 $count 个拨测任务的独立缓存${NC}"
fi

echo ""
echo -e "${YELLOW}[5/6] 清空通知发送记录表...${NC}"

# 清空通知发送记录表 (notice_records)
notice_count=$(mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASS -D $MYSQL_DB -sse "SELECT COUNT(*) FROM notice_records;" 2>/dev/null)

if [ -z "$notice_count" ] || [ "$notice_count" -eq 0 ]; then
    echo -e "${GREEN}  - 数据库中没有通知发送记录${NC}"
else
    # 清空通知记录表
    mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASS -D $MYSQL_DB -e "DELETE FROM notice_records;" 2>/dev/null

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}  - 已删除 $notice_count 条通知发送记录${NC}"
    else
        echo -e "${RED}  - 清空通知发送记录表失败${NC}"
    fi
fi

echo ""
echo -e "${YELLOW}[6/6] 清空拨测历史数据表...${NC}"

# 清空拨测历史数据表 (w8t_probing_history)
probing_history_count=$(mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASS -D $MYSQL_DB -sse "SELECT COUNT(*) FROM w8t_probing_history;" 2>/dev/null)

if [ -z "$probing_history_count" ] || [ "$probing_history_count" -eq 0 ]; then
    echo -e "${GREEN}  - 数据库中没有拨测历史数据${NC}"
else
    # 清空拨测历史数据表
    mysql -h $MYSQL_HOST -P $MYSQL_PORT -u $MYSQL_USER -p$MYSQL_PASS -D $MYSQL_DB -e "DELETE FROM w8t_probing_history;" 2>/dev/null

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}  - 已删除 $probing_history_count 条拨测历史记录${NC}"
    else
        echo -e "${RED}  - 清空拨测历史数据表失败${NC}"
    fi
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  所有告警数据已清空完成!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${YELLOW}提示:${NC}"
echo -e "  - 活跃告警会在下次规则评估时重新生成"
echo -e "  - 历史告警数据已永久删除,无法恢复"
echo -e "  - 建议在测试环境使用此脚本"
echo -e "  ${RED}- 重要: 建议重启 AlertHub 服务以彻底清除内存中的缓存状态${NC}"
echo ""