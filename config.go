package actioncable

import (
	"fmt"
	"net/http"

	"github.com/go-redis/redis/v8"
)

type config struct {
	allowedOrigins         []string
	broadcastConcurrentNum int
	readBufferSize         int
	writeBufferSize        int
	maxMessageSize         int64
	enableCompression      bool
	logger                 Logger
	authenticator          func(*http.Request) (identifier any, pass bool)
	rescuer                func(conn *Connection, exception any)
	pubsub                 PubSub
}

// Return default actioncable config.
func NewConfig() *config {
	return &config{
		enableCompression:      true,
		readBufferSize:         4096,
		writeBufferSize:        4096,
		maxMessageSize:         65535,
		broadcastConcurrentNum: 100,
		logger:                 &defaultLogger{info},
		pubsub:                 &SubscriberMap{broadcastConcurrentNum: 100},
		authenticator:          func(*http.Request) (any, bool) { return nil, true },
		rescuer: func(c *Connection, e any) {
			logger.Error(fmt.Sprintf("panic in channel callback: %v", e))
			c.Close("internal server error")
		},
	}
}

// Set the authentication function. The application could set the identifier by reading cookies or URL parameters from the *http.Request.
// * The first return value is the identifier for the client connection.
//   The identifier could be fetched by Connection.Identifier or Channel.ConnectionIdentifier.
// * The second return value determines whether the client connection passes the authentication.
//   If it's false, the server will reject the client connection.
func (c *config) WithAuthenticator(authenticator func(*http.Request) (any, bool)) *config {
	c.authenticator = authenticator
	return c
}

func (c *config) WithRedisPubSub(opts *redis.Options) *config {
	c.pubsub = &RedisPubSub{
		Client: redis.NewClient(opts),
		sm:     &SubscriberMap{},
		done:   make(chan struct{}),
	}

	c.pubsub.SetBroadcastConcurrentNum(c.broadcastConcurrentNum)

	return c
}

// Set the function of how to handle panic.
func (c *config) WithRescuer(r func(c *Connection, e any)) *config {
	c.rescuer = r

	return c
}

// Set the specific Logger implemention.
func (c *config) WithLogger(l Logger) *config {
	c.logger = l
	return c
}

// Set allowed origin hosts.
func (c *config) WithAllowedOrigins(hosts ...string) *config {
	c.allowedOrigins = hosts
	return c
}

func (c *config) WithReadBufferSize(n int) *config {
	c.readBufferSize = n
	return c
}

func (c *config) WithWriteBufferSize(n int) *config {
	c.writeBufferSize = n
	return c
}

func (c *config) WithMaxMessageSize(n int64) *config {
	c.maxMessageSize = n
	return c
}

func (c *config) WithReadWriteBufferSize(n int) *config {
	c.readBufferSize = n
	c.writeBufferSize = n
	return c
}

// NOTE: Used for testing/debugging.
func (c *config) WithDebugLogger() *config {
	c.logger = &defaultLogger{debug}
	return c
}

func (c *config) WithBroadcastConcurrentNum(n int) *config {
	c.broadcastConcurrentNum = n

	if c.pubsub != nil {
		c.pubsub.SetBroadcastConcurrentNum(n)
	}

	return c
}
