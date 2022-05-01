package actioncable

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const (
	beatInterval = 3 * time.Second
)

// The APIs actioncable would use from *websocket.Conn
type IConn interface {
	// WriteJSON writes the JSON encoding of v as a message.
	WriteJSON(any) error
	// ReadMessage is a helper method for getting a reader using NextReader and reading from that reader to a buffer.
	ReadMessage() (int, []byte, error)
	// Close closes the underlying network connection without sending or waiting for a close message.
	Close() error
}

// For every WebSocket connection the Action Cable server accepts, a Connection object will be instantiated.
type Connection struct {
	// the value returned from config.authenticator(*http.Request)
	identifier    any
	wsConn        IConn
	closed        bool
	isInitialized bool
	cable         *Cable
	send          chan any
	done          chan struct{}
	// key hierarchy: ChannelName -> SubscriptionIdentifier
	channels        map[string]map[string]*Channel
	internalChannel *Channel
	mu              sync.Mutex
}

// Setup connection.
func (conn *Connection) Setup() {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.isInitialized {
		return
	}

	go conn.serveReading()
	go conn.serveWriting()
	conn.send <- welcomeMessage
	conn.subscribeToInternalChannel()
	conn.setupHeartbeatTimer()

	conn.isInitialized = true
}

// Close connection and clean up.
func (conn *Connection) Close(reason string) {
	logger.Debug("Close connection due to " + reason)
	conn.mu.Lock()

	if conn.closed {
		logger.Debug("The connection has already been closed.")
		return
	}
	conn.closed = true
	conn.mu.Unlock()

	delete(conn.cable.connections, conn)

	for channelName := range conn.channels {
		for _, ch := range conn.channels[channelName] {
			ch.unsubscribe()
		}
	}

	if conn.internalChannel != nil {
		conn.internalChannel.unsubscribe()
	}

	close(conn.done)
	closeConnection(conn.wsConn, reason, false)
}

func (conn *Connection) writeJsonMessage(msg interface{}) error {
	return conn.wsConn.WriteJSON(msg)
}

func (conn *Connection) executeCommand(cmd *command) error {
	defer func() {
		if r := recover(); r != nil {
			conn.cable.Config.rescuer(conn, r)
		}
	}()

	logger.Debug(fmt.Sprintf("Receive command %+v", cmd))

	c := struct {
		ChannelName string `json:"channel"`
	}{}

	if err := json.Unmarshal([]byte(cmd.Identifier), &c); err != nil {
		logger.Error(fmt.Sprintf("Can't decode identifier %s, %v", cmd.Identifier, err))

		return err
	}

	if c.ChannelName == "" {
		return fmt.Errorf("didn't find channel: %s", cmd.Identifier)
	}

	switch cmd.Command {
	case "subscribe":
		conn.addSubscription(c.ChannelName, cmd.Identifier, json.RawMessage(cmd.Identifier))
	case "unsubscribe":
		conn.removeSubscription(c.ChannelName, cmd.Identifier)
	case "message":
		conn.performAction(c.ChannelName, cmd.Identifier, cmd.Data)
	default:
		logger.Error(fmt.Sprintf("Received unrecognized command %+v", cmd))

		return fmt.Errorf("received unrecognized command %+v", cmd)
	}

	return nil
}

func (conn *Connection) addSubscription(channelName, subId string, params json.RawMessage) {
	logger.Debug(fmt.Sprintf("addSubscription %s, %s, %s", channelName, subId, params))

	cd, ok := conn.cable.channelDescriptions[channelName]

	if !ok {
		logger.Error("addSubscription failed: Channel not found: " + channelName)

		return
	}

	c := newChannel(conn, subId, params, cd, func(ch *Channel, msg []byte) {
		var message any
		if err := json.Unmarshal(msg, &message); err != nil {
			logger.Error(fmt.Sprintf("Unmarshal message failed: %+v ", msg))

			return
		}

		ch.Transmit(message)
	})
	c.subscribe()
}

func (conn *Connection) performAction(channelName, subId, data string) {
	logger.Debug(fmt.Sprintf("performAction %s, %s, %s", channelName, subId, data))

	var c *Channel

	if _, ok := conn.channels[channelName]; ok {
		c = conn.channels[channelName][subId]
	}

	if c == nil {
		logger.Error("performAction failed: Channel not found: " + channelName)

		return
	}

	c.performAction(data)
}

func (conn *Connection) removeSubscription(channelName, subId string) {
	var c *Channel

	if _, ok := conn.channels[channelName]; ok {
		c = conn.channels[channelName][subId]
	}

	if c != nil {
		logger.Debug(fmt.Sprintf("Unsubscribing from channel: %+v", c.Identifier))
		c.unsubscribe()
	}
}

func (conn *Connection) serveReading() {
	for {
		message, err := conn.read()

		if err != nil {
			// TODO: distinguish unexpected read fail.
			conn.Close("close by client.")
			return
		}

		cmd := &command{}

		if err := json.Unmarshal(message, cmd); err != nil {
			logger.Error(fmt.Sprintf("Can't unmarshal message %s due to %s.", message, err))
			continue
		}

		conn.executeCommand(cmd)
	}
}

func (conn *Connection) serveWriting() {
	for {
		select {
		case <-conn.done:
			return
		case msg := <-conn.send:
			if err := conn.writeJsonMessage(msg); err != nil {
				logger.Error(fmt.Sprintf("Write message failed: %v", err))
			}
		}
	}
}

func (conn *Connection) subscribeToInternalChannel() {
	if conn.identifier == nil || conn.internalChannel != nil {
		return
	}

	name := fmt.Sprintf("action_cable/%v", conn.identifier)

	cd := &ChannelDescription{Name: name}
	ch := newChannel(conn, name, nil, cd, func(ch *Channel, data []byte) {
		var msg struct {
			Type string `json:"type"`
		}

		if err := json.Unmarshal(data, &msg); err != nil {
			logger.Error(fmt.Sprintf("Unmarshal internal message failed: %v", err))
		}

		if msg.Type == "disconnect" {
			logger.Info(fmt.Sprintf("Removing connection (%v)", conn.identifier))
			conn.Close("close by remote.")
		}
	})

	if err := conn.cable.PubSub.Subscribe(ch, ch.Name); err != nil {
		logger.Error(fmt.Sprintf("Subscribe the internal channel failed: %v", err))

		return
	}

	conn.internalChannel = ch
	logger.Info(fmt.Sprintf("Registered connection (%v)", conn.identifier))
}

func (conn *Connection) setupHeartbeatTimer() {
	if conn.isInitialized {
		return
	}

	go func() {
		for {
			time.Sleep(beatInterval)

			if conn.closed {
				return
			}

			conn.send <- newPingMessage()
		}
	}()
}

func (conn *Connection) read() ([]byte, error) {
	_, message, err := conn.wsConn.ReadMessage()
	return message, err
}

func closeConnection(wsConn IConn, reason string, reconnect bool) error {
	defer wsConn.Close()

	return wsConn.WriteJSON(&disconnectMessage{Type: "disconnect", Reason: reason, Reconnect: reconnect})
}

func rejectUnauthorizedConnection(wsConn IConn) error {
	logger.Info("An unauthorized connection attempt was rejected.")

	return closeConnection(wsConn, "unauthorized", false)
}
