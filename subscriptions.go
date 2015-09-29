package redisroutes

import (
	"github.com/garyburd/redigo/redis"
	"github.com/soveran/redisurl"
)

// Subscription Handling
type SubscriptionHandler struct {
	index map[string]subscription
	addr  string
}

// Maps a url to redis Subscribe channel
type subscription struct {
	psc         *redis.PubSubConn
	eventStream string
	msgStream   *chan string
}

func (sh *SubscriptionHandler) CreateSub(eventStream string, msgStream *chan string) {
	conn, err := redisurl.ConnectToURL(sh.addr)
	if err != nil {
		panic(err)
	}

	psc := redis.PubSubConn{Conn: conn}
	err = psc.Subscribe(eventStream)
	if err != nil {
		panic(err)
	}

	sh.index[eventStream] = subscription{&psc, eventStream, msgStream}
}

// Register an array of subscriptions
func (sh *SubscriptionHandler) CreateSubs(eventStreams []string, msgStream *chan string) {
	for _, eventStream := range eventStreams {
		sh.CreateSub(eventStream, msgStream)
	}
}

// Begin listening on all subscriptions
// Each subscription will concurrently handle events and output to
// their associated msgStream
func (sh *SubscriptionHandler) Listen() {
	for _, sub := range sh.index {
		go sub.listen()
	}
}

// listen for published events and send to message channel
func (s subscription) listen() {
	for {
		switch v := s.psc.Receive().(type) {
		case redis.Message:
			*s.msgStream <- string(v.Data)
		case error:
			return
		}
	}
}
