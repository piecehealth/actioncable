package actioncable

func newTestCable() *Cable {
	cfg := NewConfig().WithLogger(
		&testLogger{
			debugMessages: []string{},
			infoMessages:  []string{},
			errorMessages: []string{},
		},
	)

	logger = cfg.logger

	return &Cable{
		Config:              NewConfig(),
		PubSub:              cfg.pubsub,
		connections:         map[*Connection]struct{}{},
		channelDescriptions: map[string]*ChannelDescripion{},
	}
}
