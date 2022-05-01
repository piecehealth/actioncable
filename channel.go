package actioncable

import (
	"encoding/json"
	"fmt"
	"sync"
)

type ChannelSubscribedCallback func(*Channel)
type ChannelUnsubscribedCallback func(*Channel)
type ChannelPerformActionCallback func(*Channel, string)

type ChannelDescription struct {
	Name          string
	Subscribed    ChannelSubscribedCallback
	Unsubscribed  ChannelUnsubscribedCallback
	PerformAction ChannelPerformActionCallback
}

// The channel provides the basic structure of grouping behavior into logical units when communicating over the WebSocket connection.
// You can think of a channel like a form of controller, but one that's capable of pushing content to the subscriber in addition to simply
// responding to the subscriber's direct requests.
// -- Quote from rails/actioncable
//
// E.g. (client javascript code)
//
// import { createConsumer } from "@rails/actioncable"
// let consumer = createConsumer()
// consumer.subscriptions.create({ channel: "RoomChannel", id: 1 }
//
// Then the channel name will be "RoomChannel"
// The params will be map[string]any{ channel: "RoomChannel", id: 1 }
// The Identifier will be `{"channel":"RoomChannel","id":1}`
type Channel struct {
	// Channel name
	Name string
	// Channel parameters
	Params json.RawMessage
	// Mark a key as being a connection identifier index that can then be used to find the specific connection again later.
	// It's the value returned from config.authenticator(*http.Request)
	// Common identifiers are currentUser or currentAccount, but could be anything.
	ConnIdentifier any
	// Identifier for current channel. E.g. `{"channel":"RoomChannel","id":1}`
	Identifier             string
	conn                   *Connection
	pubsub                 PubSub
	isSubscriptionRejected bool
	isConfirmationSent     bool
	descrption             *ChannelDescription
	streams                map[string]struct{}
	onBroadcast            func(*Channel, []byte)
	mu                     sync.Mutex
}

// Start streaming from the named broadcasting pubsub queue.
func (c *Channel) StreamFrom(broadcasting string) {
	if c.isSubscriptionRejected {
		return
	}

	go func() {
		if err := c.pubsub.Subscribe(c, broadcasting); err != nil {
			logger.Error(fmt.Sprintf("Subscribe %s failed due to: %s", broadcasting, err.Error()))

			return
		}

		logger.Debug(fmt.Sprintf("%s is streaming from %s", c.Name, broadcasting))

		c.mu.Lock()
		c.streams[broadcasting] = struct{}{}
		c.mu.Unlock()

		c.transmitSubscriptionConfirmation()
	}()
}

// Broadcast message directly to a named broadcasting. The message will later be JSON encoded.
func (c *Channel) Broadcast(broadcasting string, message any) error {
	msg, err := json.Marshal(message)

	if err != nil {
		logger.Error(fmt.Sprintf("Can't marshal message: %+v", message))

		return err
	}

	return c.pubsub.Broadcast(c.Name, broadcasting, msg)
}

// Unsubscribes all streams associated with this channel from the pubsub queue.
func (c *Channel) StopAllStreams() {
	for b := range c.streams {
		c.StopStreamFrom(b)
	}
}

// Unsubscribes streams from the named broadcasting.
func (c *Channel) StopStreamFrom(broadcasting string) {
	c.mu.Lock()
	delete(c.streams, broadcasting)
	c.mu.Unlock()

	go c.pubsub.Unsubscribe(c, broadcasting)
}

// Transmit a hash of message to the subscriber. The hash will automatically be wrapped in a JSON envelope with
// the proper channel identifier marked as the recipient.
func (c *Channel) Transmit(message any) {
	m := channelMessage{
		Identifier: c.Identifier,
		Message:    message,
	}

	c.conn.send <- m
}

// Reject a subscription. Could be called in the Subscribled callback.
func (c *Channel) Reject() {
	c.mu.Lock()
	c.isSubscriptionRejected = true
	c.mu.Unlock()
}

func (c *Channel) subscribe() {
	conn := c.conn
	channelName := c.descrption.Name

	c.descrption.Subscribed(c)

	if c.isSubscriptionRejected {
		c.rejectSubscription()
		return
	}

	if !c.isConfirmationSent {
		c.transmitSubscriptionConfirmation()
	}

	c.conn.mu.Lock()
	defer c.conn.mu.Unlock()

	if conn.channels[channelName] == nil {
		conn.channels[channelName] = map[string]*Channel{}
	}

	conn.channels[channelName][c.Identifier] = c
}

func (c *Channel) unsubscribe() {
	if c.conn.internalChannel != c {
		c.conn.mu.Lock()
		delete(c.conn.channels[c.Name], c.Identifier)
		if len(c.conn.channels[c.Name]) == 0 {
			delete(c.conn.channels, c.Name)
		}
		c.conn.mu.Unlock()
	}

	for broadcasting := range c.streams {
		c.pubsub.Unsubscribe(c, broadcasting)
	}
	c.descrption.Unsubscribed(c)
}

func (c *Channel) performAction(data string) {
	if c.isSubscriptionRejected {
		return
	}
	c.descrption.PerformAction(c, data)
}

func (c *Channel) rejectSubscription() {
	c.unsubscribe()
	c.transmitSubscriptionRejection()
}

func (c *Channel) transmitSubscriptionConfirmation() {
	if c.isConfirmationSent {
		return
	}

	c.mu.Lock()

	if c.isConfirmationSent {
		return
	}

	c.isConfirmationSent = true
	logger.Debug(c.descrption.Name + " is transmitting the subscription confirmation")
	c.mu.Unlock()

	message := map[string]string{
		"identifier": c.Identifier,
		"type":       "confirm_subscription",
	}
	c.conn.send <- message
}

func (c *Channel) transmitSubscriptionRejection() {
	logger.Debug(c.descrption.Name + " is transmitting the subscription rejection")

	message := map[string]string{
		"identifier": c.Identifier,
		"type":       "reject_subscription",
	}
	c.conn.send <- message
}

func newChannel(conn *Connection, identifier string, params json.RawMessage, cd *ChannelDescription, onBroadcast func(ch *Channel, msg []byte)) *Channel {
	if cd.Subscribed == nil {
		cd.Subscribed = func(*Channel) {}
	}
	if cd.Unsubscribed == nil {
		cd.Unsubscribed = func(*Channel) {}
	}
	if cd.PerformAction == nil {
		cd.PerformAction = func(*Channel, string) {}
	}

	return &Channel{
		Name:           cd.Name,
		conn:           conn,
		ConnIdentifier: conn.identifier,
		Identifier:     identifier,
		Params:         params,
		pubsub:         conn.cable.PubSub,
		onBroadcast:    onBroadcast,
		descrption:     cd,
		streams:        map[string]struct{}{},
	}
}
