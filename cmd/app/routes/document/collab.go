package document

import (
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/HappyLadySauce/Knowledge-Core/cmd/app/middleware"
	v1 "github.com/HappyLadySauce/Knowledge-Core/cmd/app/types/v1"
	internaldocument "github.com/HappyLadySauce/Knowledge-Core/internal/document"
	apperrors "github.com/HappyLadySauce/Knowledge-Core/internal/errors"
)

type collabHub struct {
	mu      sync.Mutex
	clients map[int64]map[*websocket.Conn]struct{}
}

type collabMessage struct {
	Type    string                        `json:"type"`
	Version int64                         `json:"version,omitempty"`
	Ops     []v1.DocumentOperationRequest `json:"ops,omitempty"`
	Data    any                           `json:"data,omitempty"`
	Error   string                        `json:"error,omitempty"`
}

func newCollabHub() *collabHub {
	return &collabHub{clients: make(map[int64]map[*websocket.Conn]struct{})}
}

func (h *collabHub) add(documentID int64, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[documentID] == nil {
		h.clients[documentID] = make(map[*websocket.Conn]struct{})
	}
	h.clients[documentID][conn] = struct{}{}
}

func (h *collabHub) remove(documentID int64, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients[documentID], conn)
	if len(h.clients[documentID]) == 0 {
		delete(h.clients, documentID)
	}
}

func (h *collabHub) broadcast(documentID int64, msg collabMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients[documentID] {
		_ = conn.WriteJSON(msg)
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Collab opens the block-level realtime collaboration channel.
// Collab 打开块级实时协作通道。
// @Summary Open document collaboration websocket
// @Description Open a block-level realtime collaboration websocket. Admin only.
// @Tags Admin Documents
// @Produce json
// @Security BearerAuth
// @Param id path int true "Document ID"
// @Success 101 {string} string "Switching Protocols"
// @Failure 400 {object} common.SwaggerErrorResponse
// @Failure 401 {object} common.SwaggerErrorResponse
// @Failure 403 {object} common.SwaggerErrorResponse
// @Router /api/v1/admin/documents/{id}/collab [get]
func (h *Controller) Collab(c *gin.Context) {
	actor, ok := middleware.UserFromContext(c)
	if !ok {
		commonUnauthorized(c)
		return
	}
	id, ok := documentIDParam(c)
	if !ok {
		return
	}
	if !originAllowed(c.Request, h.allowedOrigins) {
		c.JSON(http.StatusForbidden, gin.H{"code": http.StatusForbidden, "message": apperrors.MessageForbidden, "data": nil})
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	h.hub.add(id, conn)
	defer h.hub.remove(id, conn)

	detail, err := h.service.GetAdmin(c.Request.Context(), actor, id)
	if err != nil {
		_ = conn.WriteJSON(collabMessage{Type: "error", Error: apperrors.From(err).Message})
		return
	}
	_ = conn.WriteJSON(collabMessage{
		Type:    "snapshot",
		Version: detail.CurrentVersion,
		Data:    toDocumentDetailResponse(detail),
	})
	h.hub.broadcast(id, collabMessage{
		Type: "presence",
		Data: map[string]any{"user_id": actor.ID, "event": "join"},
	})

	for {
		var msg collabMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		switch msg.Type {
		case "hello":
			detail, err := h.service.GetAdmin(c.Request.Context(), actor, id)
			if err != nil {
				_ = conn.WriteJSON(collabMessage{Type: "error", Error: apperrors.From(err).Message})
				continue
			}
			_ = conn.WriteJSON(collabMessage{Type: "snapshot", Version: detail.CurrentVersion, Data: toDocumentDetailResponse(detail)})
		case "op":
			result, err := h.service.ApplyOpsAdmin(c.Request.Context(), actor, id, internaldocument.ApplyOpsCommand{
				Ops: toOperations(msg.Ops),
			})
			response := toApplyOpsResponse(result)
			if err != nil {
				if apperrors.Is(err, apperrors.Conflict) && len(result.Conflicts) > 0 {
					_ = conn.WriteJSON(collabMessage{Type: "conflict", Version: response.CurrentVersion, Data: response})
					continue
				}
				_ = conn.WriteJSON(collabMessage{Type: "error", Error: apperrors.From(err).Message})
				continue
			}
			h.hub.broadcast(id, collabMessage{Type: "ack", Version: response.CurrentVersion, Data: response})
		default:
			_ = conn.WriteJSON(collabMessage{Type: "error", Error: apperrors.MessageInvalidRequest})
		}
	}
}

func commonUnauthorized(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, gin.H{"code": http.StatusUnauthorized, "message": apperrors.MessageUnauthorized, "data": nil})
}

func originAllowed(r *http.Request, allowedOrigins []string) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	for _, allowed := range allowedOrigins {
		if originMatches(parsed, strings.TrimSpace(allowed)) {
			return true
		}
	}
	return false
}

func originMatches(origin *url.URL, allowed string) bool {
	if allowed == "" {
		return false
	}
	if !strings.Contains(allowed, "*") {
		return strings.EqualFold(origin.String(), allowed)
	}
	if !strings.HasSuffix(allowed, ":*") {
		return false
	}
	base := strings.TrimSuffix(allowed, ":*")
	allowedURL, err := url.Parse(base)
	if err != nil {
		return false
	}
	if !strings.EqualFold(origin.Scheme, allowedURL.Scheme) {
		return false
	}
	return strings.EqualFold(origin.Hostname(), allowedURL.Hostname())
}
