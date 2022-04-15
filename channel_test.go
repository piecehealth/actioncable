package actioncable

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"
)

func TestStreamFromAndStopStreamFor(t *testing.T) {
	conn, ws := newTestConnection("test")
	cable := conn.cable

	cable.RegisterChannel(&ChannelDescripion{
		Name: "RoomChannel",
		Subscribed: func(c *Channel) {
			getRoomId := func(msg json.RawMessage) string {
				params := struct {
					Id int `json:"id"`
				}{}

				json.Unmarshal(msg, &params)

				return "room_" + strconv.Itoa(params.Id)
			}

			c.StreamFrom(getRoomId(c.Params))
		},
		PerformAction: func(c *Channel, data string) {
			d := struct {
				Action string `json:"action"`
			}{}
			json.Unmarshal([]byte(data), &d)

			if d.Action == "run" {
				c.StopAllStreams()
			}
		},
	})

	conn.Setup()
	defer conn.Close("test complete")

	data := `{"command":"subscribe", "identifier":"{\"channel\":\"RoomChannel\",\"id\":1}"}`
	ws.write([]byte(data))

	msg := ws.messageBox[len(ws.messageBox)-1]
	confirmMessage, ok := msg.(map[string]string)

	if !ok || confirmMessage["type"] != "confirm_subscription" {
		t.Error("Unexpected confirm message type")
	}

	sm, ok := cable.PubSub.(*SubscriberMap)

	if !ok {
		t.Error("Unexpected cable.PubSub type.")
	}

	if sm.subscribers["RoomChannel"]["room_1"] == nil {
		t.Error("Didn't subscribe from RoomChannel#room_1")
	}

	data = `{"command":"message", "identifier":"{\"channel\":\"RoomChannel\",\"id\":1}", "data":"{\"action\":\"run\"}"}`
	ws.write([]byte(data))

	if sm.subscribers["RoomChannel"]["room_1"] != nil {
		t.Error("Didn't unsubscribe from RoomChannel#room_1")
	}
}

func TestReject(t *testing.T) {
	conn, ws := newTestConnection("test")
	cable := conn.cable

	cable.RegisterChannel(&ChannelDescripion{
		Name: "RoomChannel",
		Subscribed: func(c *Channel) {
			params := struct {
				Name string `json:"name"`
			}{}

			json.Unmarshal(c.Params, &params)

			if params.Name == "private" {
				c.Reject()
			}
		},
	})

	conn.Setup()
	defer conn.Close("test complete")

	data := `{"command":"subscribe", "identifier":"{\"channel\":\"RoomChannel\",\"name\":\"private\"}"}`
	ws.write([]byte(data))

	msg := ws.messageBox[len(ws.messageBox)-1]
	m, ok := msg.(map[string]string)

	if !ok || m["type"] != "reject_subscription" {
		t.Error("Unexpected reject message type")
	}

	data = `{"command":"subscribe", "identifier":"{\"channel\":\"RoomChannel\",\"name\":\"normal\"}"}`
	ws.write([]byte(data))

	msg = ws.messageBox[len(ws.messageBox)-1]
	m, ok = msg.(map[string]string)

	if !ok || m["type"] != "confirm_subscription" {
		t.Error("Unexpected confirm message type")
	}
}

func TestBroadcast(t *testing.T) {
	conn1, ws1 := newTestConnection("user1")
	conn2, ws2 := newTestConnection("user2")
	cable := conn1.cable

	conn2.cable = cable

	cable.PubSub.Run()
	defer cable.PubSub.Stop()

	conn1.Setup()
	defer conn1.Close("test complete")

	conn2.Setup()
	defer conn2.Close("test complete")

	getRoomId := func(msg json.RawMessage) string {
		params := struct {
			Id int `json:"id"`
		}{}

		json.Unmarshal(msg, &params)

		return "room_" + strconv.Itoa(params.Id)
	}

	cable.RegisterChannel(&ChannelDescripion{
		Name: "RoomChannel",
		Subscribed: func(c *Channel) {
			c.StreamFrom(getRoomId(c.Params))
		},
		PerformAction: func(c *Channel, data string) {
			d := struct {
				Action  string `json:"action"`
				Message string `json:"message"`
			}{}

			if err := json.Unmarshal([]byte(data), &d); err != nil {
				t.Errorf("Can't unmarshal %s: %+v", data, err)
			}

			if d.Action == "send_message" {
				msg := map[string]string{
					"sendBy":  fmt.Sprintf("%v", c.ConnIdentifier),
					"message": d.Message,
				}

				c.Broadcast(getRoomId(c.Params), msg)
			}
		},
	})

	data := `{"command":"subscribe", "identifier":"{\"channel\":\"RoomChannel\",\"id\":1}"}`
	ws1.write([]byte(data))
	ws2.write([]byte(data))

	data = `{"command":"message", "identifier":"{\"channel\":\"RoomChannel\",\"id\":1}", "data":"{\"action\":\"send_message\", \"message\":\"Hello Actioncable!\"}"}`
	ws1.write([]byte(data))

	msg1 := ws1.messageBox[len(ws1.messageBox)-1]
	msg2 := ws2.messageBox[len(ws2.messageBox)-1]

	if cm, ok := msg1.(channelMessage); !ok {
		t.Errorf("Unexpected message: %+v", msg1)
	} else {
		if m, ok := cm.Message.(map[string]any); !ok || m["message"] != "Hello Actioncable!" || m["sendBy"] != "user1" {
			t.Errorf("Unexpected message: %+v", m)
		}
	}

	if cm, ok := msg2.(channelMessage); !ok {
		t.Errorf("Unexpected message: %+v", msg2)
	} else {
		if m, ok := cm.Message.(map[string]any); !ok || m["message"] != "Hello Actioncable!" || m["sendBy"] != "user1" {
			t.Errorf("Unexpected message: %+v", m)
		}
	}

	cable.Broadcast("RoomChannel", "room_1", map[string]string{"hello": "actioncable"})
	time.Sleep(5 * time.Millisecond)

	msg1 = ws1.messageBox[len(ws1.messageBox)-1]
	msg2 = ws2.messageBox[len(ws2.messageBox)-1]

	if cm, ok := msg1.(channelMessage); !ok {
		t.Errorf("Unexpected message: %+v", msg1)
	} else {
		if m, ok := cm.Message.(map[string]any); !ok || m["hello"] != "actioncable" {
			t.Errorf("Unexpected message: %+v", m)
		}
	}

	if cm, ok := msg2.(channelMessage); !ok {
		t.Errorf("Unexpected message: %+v", msg2)
	} else {
		if m, ok := cm.Message.(map[string]any); !ok || m["hello"] != "actioncable" {
			t.Errorf("Unexpected message: %+v", m)
		}
	}
}
