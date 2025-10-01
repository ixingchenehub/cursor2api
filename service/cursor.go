package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/imroc/req/v3"
	utls "github.com/refraction-networking/utls"

	"github.com/gopkg-dev/cursor2api/models"
	"github.com/gopkg-dev/cursor2api/types"
	"github.com/gopkg-dev/cursor2api/utils"
)

// CursorService Cursor API 服务
type CursorService struct {
	manager   *models.AntiBotManager
	converter *utils.MessageConverter
	client    *req.Client
}

// NewCursorService 创建 Cursor 服务
func NewCursorService(manager *models.AntiBotManager, systemPrompt string) *CursorService {
	return &CursorService{
		manager:   manager,
		converter: utils.NewMessageConverter(systemPrompt),
		client: req.C().
			ImpersonateChrome().
			SetTLSFingerprint(utls.HelloChrome_131).
			EnableInsecureSkipVerify(),
	}
}

// Chat 非流式聊天
func (cs *CursorService) Chat(ctx context.Context, messages []types.ChatMessage, model string, conversationID string) (string, error) {
	xIsHuman, err := cs.manager.GetXIsHuman()
	if err != nil {
		log.Printf("❌ 获取认证参数失败: %v", err)
		return "", fmt.Errorf("获取认证参数失败: %w", err)
	}

	requestBody := cs.converter.BuildCursorRequest(messages, model, conversationID)

	// 记录请求详情
	log.Printf("🔵 [非流式] 请求 Cursor API")
	log.Printf("  └─ Model: %s", model)
	log.Printf("  └─ ConversationID: %s", conversationID)
	log.Printf("  └─ Messages: %d 条", len(messages))
	log.Printf("  └─ Request Body (格式化):")
	log.Printf("     %s", utils.MarshalIndentToString(requestBody))

	resp, err := cs.client.R().
		SetContext(ctx).
		SetHeaders(map[string]string{
			"referer":    "https://cursor.com/cn/learn/context",
			"x-is-human": xIsHuman,
			"x-method":   "POST",
			"x-path":     "/api/chat",
		}).
		SetBodyString(requestBody).
		Post("https://cursor.com/api/chat")

	if err != nil {
		if ctx.Err() != nil {
			log.Printf("⚠️  请求被取消: %v", ctx.Err())
			return "", ctx.Err()
		}
		log.Printf("❌ 请求失败: %v", err)
		return "", fmt.Errorf("请求失败: %w", err)
	}

	log.Printf("✅ 收到响应: HTTP %d", resp.StatusCode)

	if !resp.IsSuccessState() {
		responseBody := resp.String()
		log.Printf("❌ HTTP 错误: %d", resp.StatusCode)
		log.Printf("  └─ Response: %s", responseBody)
		return "", fmt.Errorf("HTTP错误: %d", resp.StatusCode)
	}

	responseBody := resp.String()
	log.Printf("📥 [非流式] 响应内容: %s", responseBody)

	return responseBody, nil
}

// StreamChat 流式聊天
func (cs *CursorService) StreamChat(ctx context.Context, messages []types.ChatMessage, model string, conversationID string) (<-chan string, <-chan error) {
	contentChan := make(chan string, 10)
	errorChan := make(chan error, 1)

	go func() {
		defer close(contentChan)
		defer close(errorChan)

		xIsHuman, err := cs.manager.GetXIsHuman()
		if err != nil {
			log.Printf("❌ 获取认证参数失败: %v", err)
			errorChan <- fmt.Errorf("获取认证参数失败: %w", err)
			return
		}

		requestBody := cs.converter.BuildCursorRequest(messages, model, conversationID)

		// 记录请求详情
		log.Printf("🟢 [流式] 请求 Cursor API")
		log.Printf("  └─ Model: %s", model)
		log.Printf("  └─ ConversationID: %s", conversationID)
		log.Printf("  └─ Messages: %d 条", len(messages))
		log.Printf("  └─ Request Body (格式化):")
		log.Printf("     %s", utils.MarshalIndentToString(requestBody))

		resp, err := cs.client.R().
			SetContext(ctx).
			SetHeaders(map[string]string{
				"referer":    "https://cursor.com/cn/learn/context",
				"x-is-human": xIsHuman,
				"x-method":   "POST",
				"x-path":     "/api/chat",
			}).
			SetBodyString(requestBody).
			DisableAutoReadResponse().
			Post("https://cursor.com/api/chat")

		if err != nil {
			if ctx.Err() != nil {
				log.Printf("⚠️  请求被取消: %v", ctx.Err())
				errorChan <- ctx.Err()
				return
			}
			log.Printf("❌ 请求失败: %v", err)
			errorChan <- fmt.Errorf("请求失败: %w", err)
			return
		}

		log.Printf("✅ 收到响应: HTTP %d", resp.StatusCode)

		if !resp.IsSuccessState() {
			log.Printf("❌ HTTP 错误: %d", resp.StatusCode)
			errorChan <- fmt.Errorf("HTTP错误: %d", resp.StatusCode)
			return
		}

		log.Printf("📥 [流式] 开始接收 SSE 流...")
		chunkCount := 0

		// 创建可中断的 Reader
		bodyReader := &contextReader{
			ctx:    ctx,
			reader: resp.Body,
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		scanner := bufio.NewScanner(bodyReader)
		for scanner.Scan() {
			line := scanner.Text()

			if strings.TrimSpace(line) == "" {
				continue
			}

			if after, ok := strings.CutPrefix(line, "data: "); ok {
				data := after

				if data == "[DONE]" {
					log.Printf("✅ [流式] 接收完成,共 %d 个 chunk", chunkCount)
					break
				}

				var event types.SSEEventData
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					log.Printf("⚠️  解析 SSE 事件失败: %v, data: %s", err, data)
					continue
				}

				if event.Type == "text-delta" && event.Delta != "" {
					chunkCount++
					log.Printf("📦 [流式] Chunk #%d: %s", chunkCount, event.Delta)

					// 发送前检查 context 是否已取消
					select {
					case <-ctx.Done():
						log.Printf("⚠️  发送 chunk 时检测到客户端取消")
						return
					case contentChan <- event.Delta:
						// 发送成功
					}
				}
			}
		}

		// 检查 scanner 错误
		if err := scanner.Err(); err != nil {
			// 如果是因为 context 取消导致的错误,直接返回
			if ctx.Err() != nil {
				log.Printf("⚠️  读取流时客户端取消: %v", ctx.Err())
				return
			}
			log.Printf("❌ 读取响应流失败: %v", err)
			errorChan <- fmt.Errorf("读取响应流失败: %w", err)
			return
		}

		log.Printf("✅ 流式响应完整处理完毕")
	}()

	return contentChan, errorChan
}

// contextReader 包装 io.Reader,使其能响应 context 取消
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

// Read 实现 io.Reader 接口,在每次读取前检查 context 状态
func (cr *contextReader) Read(p []byte) (n int, err error) {
	// 检查 context 是否已取消
	select {
	case <-cr.ctx.Done():
		log.Printf("⚠️  contextReader 检测到客户端取消")
		return 0, cr.ctx.Err()
	default:
	}

	// 执行实际读取
	return cr.reader.Read(p)
}

