package actioncable

type PubSub interface {
	Run() error
	Stop() error
	SetBroadcastConcurrentNum(int)
	Broadcast(channelName, broadcasting string, message []byte) error
	Subscribe(channel *Channel, broadcasting string) error
	Unsubscribe(channel *Channel, broadcasting string) error
}
