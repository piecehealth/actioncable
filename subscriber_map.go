package actioncable

import (
	"fmt"
	"sync"
)

type SubscriberMap struct {
	broadcastConcurrentNum int
	done                   chan struct{}
	sending                chan *envelope
	// Key hierarchy ChannelName -> Broadcasting
	subscribers map[string]map[string]map[*Channel]struct{}
	mu          sync.Mutex
}

type envelope struct {
	receiver *Channel
	message  []byte
}

var _ PubSub = (*SubscriberMap)(nil)

func (sm *SubscriberMap) Run() error {
	if sm.broadcastConcurrentNum == 0 {
		sm.broadcastConcurrentNum = 100
	}

	if sm.done == nil {
		sm.done = make(chan struct{})
	}

	if sm.sending == nil {
		sm.sending = make(chan *envelope)
	}

	for i := 0; i < sm.broadcastConcurrentNum; i++ {
		go func() {
			for {
				select {
				case e := <-sm.sending:
					e.receiver.onBroadcast(e.receiver, e.message)
				case <-sm.done:
					return
				}
			}
		}()
	}

	return nil
}

func (sm *SubscriberMap) SetBroadcastConcurrentNum(n int) {
	sm.broadcastConcurrentNum = n
}

func (sm *SubscriberMap) Stop() error {
	close(sm.done)
	return nil
}

func (sm *SubscriberMap) Subscribe(c *Channel, broadcasting string) (err error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.subscribers == nil {
		sm.subscribers = map[string]map[string]map[*Channel]struct{}{}
	}

	if sm.subscribers[c.Name] == nil {
		sm.subscribers[c.Name] = map[string]map[*Channel]struct{}{}
	}

	if sm.subscribers[c.Name][broadcasting] == nil {
		sm.subscribers[c.Name][broadcasting] = map[*Channel]struct{}{}
	}

	sm.subscribers[c.Name][broadcasting][c] = struct{}{}

	return
}

func (sm *SubscriberMap) Broadcast(channelName, broadcasting string, message []byte) (err error) {
	_, ok := sm.subscribers[channelName]

	if !ok {
		logger.Error("Can't find any subscribers for Channel " + channelName)

		return
	}

	subscribers, ok := sm.subscribers[channelName][broadcasting]

	if !ok {
		return
	}

	logger.Debug(fmt.Sprintf("Broadcasting to %s: %s", broadcasting, message))

	go func() {
		for c := range subscribers {
			logger.Debug(fmt.Sprintf("%s transmitting %s (via streamed from %s)", c.Name, message, broadcasting))

			sm.sending <- &envelope{receiver: c, message: message}
		}
	}()

	return
}

func (sm *SubscriberMap) Unsubscribe(c *Channel, broadcasting string) (err error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	_, ok := sm.subscribers[c.Name]

	if !ok {
		return
	}

	if _, ok := sm.subscribers[c.Name][broadcasting]; ok {
		delete(sm.subscribers[c.Name][broadcasting], c)

		if len(sm.subscribers[c.Name][broadcasting]) == 0 {
			delete(sm.subscribers[c.Name], broadcasting)
		}

		if len(sm.subscribers[c.Name]) == 0 {
			delete(sm.subscribers, c.Name)
		}
	}

	return
}
