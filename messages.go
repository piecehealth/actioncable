package actioncable

import (
	"time"
)

var welcomeMessage = map[string]string{"type": "welcome"}

type disconnectMessage struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Reconnect bool   `json:"reconnect"`
}

type pingMessage struct {
	Type    string `json:"type"`
	Message int64  `json:"message"`
}

type command struct {
	Identifier string `json:"identifier"`
	Command    string `json:"command"`
	Data       string `json:"data"`
}

type channelMessage struct {
	Identifier string `json:"identifier"`
	Message    any    `json:"message"`
}

func newPingMessage() *pingMessage {
	return &pingMessage{
		Type:    "ping",
		Message: time.Now().Unix(),
	}
}
