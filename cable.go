package actioncable

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
)

const (
	Version = "0.1.0"
)

type Cable struct {
	Config              *config
	PubSub              PubSub
	connections         map[*Connection]struct{}
	channelDescriptions map[string]*ChannelDescription
}

var logger Logger

func NewActionCable(cfg *config) *Cable {
	cb := &Cable{
		Config:              cfg,
		PubSub:              cfg.pubsub,
		connections:         map[*Connection]struct{}{},
		channelDescriptions: map[string]*ChannelDescription{},
	}
	logger = cfg.logger

	if err := cb.PubSub.Run(); err != nil {
		panic(err)
	}

	return cb
}

func (cb *Cable) Handle(w http.ResponseWriter, r *http.Request) error {
	cfg := cb.Config

	upgrader := &websocket.Upgrader{
		CheckOrigin:       checkOrigin(cfg.allowedOrigins),
		Subprotocols:      []string{"actioncable-v1-json"},
		EnableCompression: cfg.enableCompression,
		ReadBufferSize:    cfg.readBufferSize,
		WriteBufferSize:   cfg.writeBufferSize,
	}

	wsConn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		logger.Info(fmt.Sprintf("Websocket connection upgrade error: %#v.", err))

		return err
	}

	wsConn.SetReadLimit(cfg.maxMessageSize)

	if cfg.enableCompression {
		wsConn.EnableWriteCompression(true)
	}

	logger.Info("Successfully upgraded to WebSocket.")

	id, pass := cfg.authenticator(r)

	if !pass {
		return rejectUnauthorizedConnection(wsConn)
	}

	conn := &Connection{
		identifier: id,
		wsConn:     wsConn,
		cable:      cb,
		send:       make(chan any),
		done:       make(chan struct{}),
		channels:   map[string]map[string]*Channel{},
	}

	conn.Setup()
	cb.connections[conn] = struct{}{}

	return nil
}

func (cb *Cable) RegisterChannel(cd *ChannelDescription) {
	if cd.Name == "" {
		panic(fmt.Sprintf("channel name can't be nil: %+v", cd))
	}

	if _, ok := cb.channelDescriptions[cd.Name]; ok {
		panic(fmt.Sprintf("the Channel %s has already been registered.", cd.Name))
	}

	cb.channelDescriptions[cd.Name] = cd
}

// Disconnect a remote connection by the connection identifier.
func (cb *Cable) DisconnectRemoteConnection(identifier any) {
	name := fmt.Sprintf("action_cable/%v", identifier)
	msg, _ := json.Marshal(map[string]string{"type": "disconnect"})

	cb.PubSub.Broadcast(name, name, msg)
}

func (cb *Cable) Broadcast(channel, broadcasting string, message any) error {
	msg, err := json.Marshal(message)

	if err != nil {
		return err
	}

	return cb.PubSub.Broadcast(channel, broadcasting, msg)
}

func (cb *Cable) Stop() {
	cb.PubSub.Stop()
	for conn := range cb.connections {
		conn.Close("server is shutdown.")
	}
}

// Stolen from github.com/anycable/anycable-go
func checkOrigin(hosts []string) func(r *http.Request) bool {
	if len(hosts) == 0 {
		return func(r *http.Request) bool { return true }
	}

	return func(r *http.Request) bool {
		origin := strings.ToLower(r.Header.Get("Origin"))
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}

		for _, host := range hosts {
			if host[0] == '*' && strings.HasSuffix(u.Host, host[1:]) {
				return true
			}
			if u.Host == host {
				return true
			}
		}
		return false
	}
}
