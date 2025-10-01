package handler

import (
	"github.com/gopkg-dev/cursor2api/config"
	"github.com/gopkg-dev/cursor2api/models"
	"github.com/gopkg-dev/cursor2api/service"
	"github.com/gopkg-dev/cursor2api/utils"
)

// APIHandler API 处理器
type APIHandler struct {
	cursorService *service.CursorService
	manager       *models.AntiBotManager
	converter     *utils.MessageConverter
	config        *config.Config
}

// NewAPIHandler 创建 API 处理器
func NewAPIHandler(cursorService *service.CursorService, manager *models.AntiBotManager, cfg *config.Config) *APIHandler {
	return &APIHandler{
		cursorService: cursorService,
		manager:       manager,
		converter:     utils.NewMessageConverter(cfg.Cursor.SystemPrompt),
		config:        cfg,
	}
}
