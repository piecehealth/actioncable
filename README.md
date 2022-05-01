# actioncable

A golang websocket framework, a server implementation of [rails/actioncable](https://github.com/rails/rails/tree/main/actioncable)

[Full-Stack Examples](https://github.com/piecehealth/actioncable-examples)

## Client library
* [Javascript](https://www.npmjs.com/package/@rails/actioncable)
* [Client-Server Interaction](https://guides.rubyonrails.org/action_cable_overview.html#client-server-interaction)

## Basic Usage

### Setup cable

```golang
package main

import (
	"..."
	"github.com/piecehealth/actioncable"
)

var cable *actioncable.Cable

func main() {
  cbCfg := actioncable.NewConfig() // actioncable default config

  // use redis for the PubSub service if your application run on multiple nodes.
	// cbCfg = cbCfg.WithRedisPubSub(&redis.Options{Addr: "localhost:6379"})

  // Config how to authenticate client connection.
  // You can fetch user information by cookies or URL parameters.
  //
  // The first return value is the identifier of the connection. Common identifier
  // is currentUser, but could be anything.
  //
  // If the second return value is false, the server would reject the connection.
  cbCfg = cbCfg.WithAuthenticator(func(h *http.Request) (any, bool) {
		name, err := h.Cookie("userName")

		if err != nil {
			return nil, false
		}

		id, _ := url.QueryUnescape(name.Value)

    if id == "the unwelcome user" {
      return nil, false
    } else {
      return id, true
    }
	})

  // Initialize cable.
  cable = actioncable.NewActionCable(cbCfg)

  // Use GIN as example.
  router := gin.Default()

  // Handle websocket connection/request. The default endpoint is "/cable".
  router.GET("/cable", func(c *gin.Context) {
		cable.Handle(c.Writer, c.Request)
	})
}
```

### Define Channel
```golang
var getRoomId = func(params json.RawMessage) string {
	p := struct {
		Id string `json:"id"`
	}{}

	json.Unmarshal(params, &p)

	return fmt.Sprintf("room_%s", p.Id)
}

var roomChannel = &actioncable.ChannelDescription{
  Name: "RoomChannel",
  // Called once a consumer has become a subscriber of the channel.
  Subscribed: func(c *actioncable.Channel) {
    roomId := getRoomId(c.Params)
		currentUser := fmt.Sprintf("%v", c.ConnIdentifier)

		if roomId == "forbidden_room" {
			// reject Subscription.
      c.Reject()
			return
		}

    // Start streaming from `RoomChannel#room_<id>`
		c.StreamFrom(roomId)
  },
  // handle messages from client socket.
  PerformAction: func(c *actioncable.Channel, jsonData string) {
		d := struct {
			Action  string `json:"action"` // the action would be set by the client library.
			Message string `json:"message"`
		}{}

		json.Unmarshal([]byte(jsonData), &d)

		roomId := getRoomId(c.Params)

		switch d.Action {
		case "send_message":
			currentUser := fmt.Sprintf("%v", c.ConnIdentifier)

			m := map[string]string{
				"send_by": currentUser,
				"message": d.Message,
			}

			// Broadcast the message to all the subscribers.
      c.Broadcast(roomId, m)
		default:
			log.Printf("Unable to process RoomChannel#%s", d.Action)
		}
	},
  // Called once a consumer has cut its cable connection.
  Unsubscribed: func(c *actioncable.Channel) {
    // ...
  }
}

// Register the channel to the cable.
cable.RegisterChannel(roomChannel)
```


## [Full-Stack Example](https://github.com/piecehealth/actioncable-examples)
