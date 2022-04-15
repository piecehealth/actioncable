package actioncable

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis/v8"
)

// A pubsub implementation with Redis backend.
type RedisPubSub struct {
	Client *redis.Client
	sm     *SubscriberMap
	pubsub *redis.PubSub
	done   chan struct{}
}

var _ PubSub = (*RedisPubSub)(nil)

type broadcastingMessage struct {
	ChannelName  string `json:"channel_name"`
	Broadcasting string `json:"broadcasting"`
	Message      string `json:"message"`
}

const redisChannelName = "_action_cable_internal"

func (r *RedisPubSub) Run() error {
	ctx := context.TODO()

	if r.done == nil {
		r.done = make(chan struct{})
	}

	if r.pubsub == nil {
		r.pubsub = r.Client.Subscribe(ctx, redisChannelName)
	}

	r.sm.Run()

	go func() {
		for {
			select {
			case msg := <-r.pubsub.Channel():
				var m broadcastingMessage
				if err := json.Unmarshal([]byte(msg.Payload), &m); err != nil {
					logger.Error(fmt.Sprintf("Unmarshal redis broadcasting message failed: %+v", err))

					continue
				}

				r.sm.Broadcast(m.ChannelName, m.Broadcasting, []byte(m.Message))
			case <-r.done:
				return
			}
		}
	}()

	return nil
}

func (r *RedisPubSub) SetBroadcastConcurrentNum(n int) {
	r.sm.SetBroadcastConcurrentNum(n)
}

func (r *RedisPubSub) Stop() error {
	r.sm.Stop()
	close(r.done)

	return r.Client.Close()
}

func (r *RedisPubSub) Subscribe(c *Channel, broadcasting string) error {
	return r.sm.Subscribe(c, broadcasting)
}

func (r *RedisPubSub) Broadcast(channelName, broadcasting string, message []byte) error {
	msg := broadcastingMessage{
		ChannelName:  channelName,
		Broadcasting: broadcasting,
		Message:      string(message),
	}
	b, _ := json.Marshal(msg)

	return r.Client.Publish(context.TODO(), redisChannelName, b).Err()
}

func (r *RedisPubSub) Unsubscribe(c *Channel, broadcasting string) error {
	return r.sm.Unsubscribe(c, broadcasting)
}
