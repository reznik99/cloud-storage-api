package internal

import (
	"errors"
	"time"

	"github.com/gorilla/websocket"
)

const SOCKET_PING_EVERY = time.Second * 30
const SOCKET_PING_TIMEOUT = time.Second * 10

type SocketMsg struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Command string `json:"command"`
	Data    string `json:"data"`
}

// PingSockets pings all connected clients in Handler, if any fail or timeout, they are closed and deleted.
func (h *Handler) PingSockets() {
	ticker := time.NewTicker(SOCKET_PING_EVERY)
	defer ticker.Stop()
	// Repeat forever
	for {
		// This blocks for SOCKET_PING_EVERY
		<-ticker.C
		// This executes every SOCKET_PING_EVERY
		h.Logger.Infof("Initiating websocket pings")
		h.WebSockets.Range(func(key, value any) bool {
			conn := value.(*websocket.Conn)
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(SOCKET_PING_TIMEOUT)); err != nil {
				h.Logger.Warnf("ping error: %s", err)
				if err := conn.Close(); err != nil {
					h.Logger.Errorf("ping close error: %s", err)
				}
				h.WebSockets.Delete(key)
			}
			return true
		})
	}
}

func (h *Handler) HandleSocket(socketKey string, conn *websocket.Conn) {
	// Cache socket for later use
	h.WebSockets.Store(socketKey, conn)
	// Defer cleanup and closing of socket
	defer func() {
		conn.Close()
		h.WebSockets.Delete(socketKey)
	}()
	// Give client their key
	err := h.SocketWriteJSON(socketKey, &SocketMsg{
		Command: "websocket-key",
		Data:    socketKey,
	})
	if err != nil {
		h.Logger.Errorf("Failed to send websocket key to client: %s", err)
		return
	}
	// Log closure
	defaultCloseHandler := conn.CloseHandler()
	conn.SetCloseHandler(func(code int, text string) error {
		h.Logger.Infof("Closing websocket conn: %s", socketKey)
		return defaultCloseHandler(code, text)
	})

	// Be ready for incoming messages
	for {
		message := &SocketMsg{}
		err := conn.ReadJSON(message)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.Logger.Errorf("Unexpected socket close error: %v", err)
			}
			break
		}
		message.From = socketKey // Override from value
		h.Logger.Info("Recieved websocket message")
		switch message.Command {
		case "answer":
			err = h.SocketWriteJSON(message.To, message)
		case "icecandidate":
			err = h.SocketWriteJSON(message.To, message)
		default:
			h.Logger.Warnf("Unrecognized websocket message command: %s", message.Command)
		}
		if err != nil {
			h.Logger.Warnf("Socket command error: %s", err)
		}
	}
}

func (h *Handler) SocketWriteJSON(socketKey string, data any) error {
	value, ok := h.WebSockets.Load(socketKey)
	if !ok {
		return errors.New("failed to read socket from client map")
	}
	conn := value.(*websocket.Conn)

	return conn.WriteJSON(data)
}
