package actioncable

import (
	"errors"
	"strings"
	"testing"
	"time"
)

type testWsConnection struct {
	readCn      chan []byte
	done        chan struct{}
	clientClose chan struct{}
	messageBox  []any
	isClosed    bool
}

var _ IConn = (*testWsConnection)(nil)

func (c *testWsConnection) WriteJSON(j any) error {
	c.messageBox = append(c.messageBox, j)

	return nil
}

func (c *testWsConnection) ReadMessage() (int, []byte, error) {
	select {
	case m := <-c.readCn:
		return 1, m, nil
	case <-c.clientClose:
		return 1, nil, errors.New("connection is closed.")
	}
}

func (c *testWsConnection) Close() error {
	c.isClosed = true
	close(c.done)
	return nil
}

func (c *testWsConnection) write(b []byte) {
	c.readCn <- b
	time.Sleep(5 * time.Millisecond)
}

func newTestConnection(id any) (*Connection, *testWsConnection) {
	wsConn := &testWsConnection{
		readCn:      make(chan []byte),
		done:        make(chan struct{}),
		clientClose: make(chan struct{}),
		messageBox:  []any{},
	}

	return &Connection{
		identifier: id,
		wsConn:     wsConn,
		cable:      newTestCable(),
		send:       make(chan any),
		done:       make(chan struct{}),
		channels:   map[string]map[string]*Channel{},
	}, wsConn
}

func TestConnectionSetup(t *testing.T) {
	conn, ws := newTestConnection("test")
	conn.Setup()
	defer conn.Close("test complete")

	if !conn.isInitialized {
		t.Error("The connection is not initialized.")
	}

	m, _ := ws.messageBox[0].(map[string]string)
	if m["type"] != "welcome" {
		t.Error("Didn't send welcome message.")
	}
}

func TestClose(t *testing.T) {
	conn, ws := newTestConnection("test")
	conn.Setup()

	close(ws.clientClose) // simulate sending a close message.
	time.Sleep(5 * time.Millisecond)

	if !conn.closed {
		t.Error("The connection is not closed.")
	}
}

func TestExecuteCommandWrongMessage(t *testing.T) {
	conn, ws := newTestConnection("test")
	conn.Setup()
	defer conn.Close("test complete")

	ws.write([]byte(`{"command":"random command", "identifier":"{\"channel\":\"ChatChannel\",\"room\":\"Best Room\"}"}`))

	l := getCurrentTestLogger()
	if !strings.HasPrefix(l.errorMessages[len(l.errorMessages)-1], "Received unrecognized command ") {
		t.Error("didn't report unrecognized command")
	}

	ws.write([]byte(`{"command":"subscribe", "identifier":"{\"channel\":\"ChatChannel\",\"room\":\"Best Room\"}"}`))

	if l.errorMessages[len(l.errorMessages)-1] != "addSubscription failed: Channel not found: ChatChannel" {
		t.Error("addSubscription check failed.")
	}
}

func TestExecuteCommand(t *testing.T) {
	conn, ws := newTestConnection("test")
	conn.Setup()
	defer conn.Close("test complete")

	messages := []string{}

	conn.cable.RegisterChannel(&ChannelDescripion{
		Name:          "ChatChannel",
		PerformAction: func(_ *Channel, msg string) { messages = append(messages, msg) },
	})

	data := `{"command":"subscribe", "identifier":"{\"channel\":\"ChatChannel\",\"room\":\"Best Room\"}"}`
	ws.write([]byte(data))

	if conn.channels["ChatChannel"][`{"channel":"ChatChannel","room":"Best Room"}`] == nil {
		t.Error("didn't subscribe the channel.")
	}

	data = `
		{
			"command":"message",
			"identifier":"{\"channel\":\"ChatChannel\",\"room\":\"Best Room\"}",
			"data": "{\"message\":\"test\",\"action\":\"test\"}"
		}
	`

	ws.write([]byte(data))

	if messages[0] != `{"message":"test","action":"test"}` {
		t.Error("didn't receive the message.")
	}

	data = `{"command":"unsubscribe", "identifier":"{\"channel\":\"ChatChannel\",\"room\":\"Best Room\"}"}`

	ws.write([]byte(data))

	if conn.channels["ChatChannel"][`{"channel":"ChatChannel","room":"Best Room"}`] != nil {
		t.Error("didn't unsubscribe the channel.")
	}
}
