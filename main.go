package main

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gopkg-dev/cursor2api/config"
	"github.com/gopkg-dev/cursor2api/handler"
	"github.com/gopkg-dev/cursor2api/logger"
	"github.com/gopkg-dev/cursor2api/middleware"
	"github.com/gopkg-dev/cursor2api/models"
	"github.com/gopkg-dev/cursor2api/service"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化日志系统
	logger.Init(cfg.Logger.Level, cfg.Logger.Verbose)

	logger.Info("========================================")
	logger.Info(" Cursor2API - Go Implementation")
	logger.Info(" OpenAI-compatible Cursor API Service")
	logger.Info("========================================")

	// 创建 AntiBot 管理器
	logger.Info("🔧 初始化 AntiBot 管理器...")
	manager := models.NewAntiBotManager(
		cfg.Cursor.JSURL,
		cfg.Cursor.ProcessURL,
		cfg.Cursor.RefreshInterval,
		cfg.Cursor.IdleTimeout,
	)

	// 启动管理器
	if err := manager.Start(); err != nil {
		logger.Fatal("❌ 启动管理器失败: %v", err)
	}
	logger.Info("✅ AntiBot 管理器启动成功")

	// 创建服务和处理器
	cursorService := service.NewCursorService(manager, cfg.Cursor.SystemPrompt)
	apiHandler := handler.NewAPIHandler(cursorService, manager, cfg)

	// 设置路由
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", apiHandler.HandleChatCompletions)
	mux.HandleFunc("/v1/models", apiHandler.HandleModels)
	mux.HandleFunc("/health", apiHandler.HandleHealth)

	// 应用中间件
	handlerWithMiddleware := middleware.CORS(mux)

	// 创建服务器
	server := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: handlerWithMiddleware,
	}

	// 监听系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("\n🛑 收到关闭信号，正在清理资源...")
		manager.Stop()
		os.Exit(0)
	}()

	// 打印服务信息
	logger.Info("========================================")
	logger.Info("✨ 服务已启动，监听端口: %s", cfg.Server.Port)
	logger.Info("📊 Health check: http://localhost:%s/health", cfg.Server.Port)
	logger.Info("🤖 API endpoint: http://localhost:%s/v1/chat/completions", cfg.Server.Port)
	logger.Info("📋 Model list: http://localhost:%s/v1/models", cfg.Server.Port)
	logger.Info("========================================")

	// 启动 HTTP 服务器
	if err := server.ListenAndServe(); err != nil {
		logger.Fatal("❌ 服务器启动失败: %v", err)
	}
}
