#!/bin/bash

# 并发测试脚本 - 同时发送多个请求,观察响应时间

URL="http://localhost:3001/v1/chat/completions"

echo "🧪 测试并发请求..."
echo "发送 3 个并发请求,观察是否串行执行"
echo "========================================"

# 记录开始时间
start=$(date +%s)

# 并发发送 3 个请求
for i in 1 2 3; do
  {
    echo "🚀 请求 #$i 开始: $(date +%T)"

    response_time=$(curl -o /dev/null -s -w "%{time_total}\n" -X POST "$URL" \
      -H "Content-Type: application/json" \
      -d "{
        \"model\": \"anthropic/claude-4.5-sonnet\",
        \"messages\": [{\"role\": \"user\", \"content\": \"你好,请说5个字\"}],
        \"stream\": false
      }")

    echo "✅ 请求 #$i 完成: $(date +%T) (耗时: ${response_time}s)"
  } &
done

# 等待所有后台任务完成
wait

end=$(date +%s)
total=$((end - start))

echo "========================================"
echo "✅ 所有请求完成,总耗时: ${total}s"
echo ""
echo "📊 结论:"
echo "  - 如果总耗时 ≈ 单个请求耗时 → 并发执行 ✅"
echo "  - 如果总耗时 ≈ 单个请求耗时 × 3 → 串行执行 ❌"