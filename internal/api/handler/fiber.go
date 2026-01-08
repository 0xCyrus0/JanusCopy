package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"go.uber.org/zap"
)

type HealthHandler struct {
	logger *zap.Logger
}

func NewHealthHandler(log *zap.Logger) *HealthHandler {
	return &HealthHandler{
		logger: log,
	}
}

// Health returns gateway health status
func (h *HealthHandler) Health(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
	})
}

// Status returns gateway status
func (h *HealthHandler) Status(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":    "running",
		"timestamp": time.Now().UTC(),
		"backend":   "http://localhost:3000",
	})
}

// HandleWebSocket handles WebSocket connections
func HandleWebSocket(c *fiber.Ctx) error {
	// Check if the connection is WebSocket
	if websocket.IsWebSocketUpgrade(c) {
		return websocket.New(func(ws *websocket.Conn) {
			for {
				mt, msg, err := ws.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						return
					}
					return
				}

				// Echo message back
				if err := ws.WriteMessage(mt, msg); err != nil {
					return
				}
			}
		})(c)
	}

	return fiber.NewError(fiber.StatusUpgradeRequired, "WebSocket upgrade required")
}
