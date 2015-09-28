package main

import (
	"fmt"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/soveran/redisurl"
)

// Subscription Handling
type SubscriptionHandler struct {
	index map[string]subscription
}

// Maps a url to redis Subscribe channel
type subscription struct {
	psc         *redis.PubSubConn
	eventStream string
	msgStream   *chan string
}

func (sh *SubscriptionHandler) CreateSub(eventStream string, msgStream *chan string) {
	conn, err := redisurl.ConnectToURL("redis://localhost:6379")
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

// Sandbox
func main() {
	sh := new(SubscriptionHandler)
	sh.index = make(map[string]subscription)
	msgStream := make(chan string)

	// Can be registered individually
	sh.CreateSub("projet2500:0:eventstream", &msgStream)
	sh.CreateSub("projet2500:1:eventstream", &msgStream)

	// Can be registered as a group
	// *But only if each sub shares an outgoing msgStream
	eventStreams := []string{
		"projet2500:2:eventstream",
		"projet2500:3:eventstream",
		"projet2500:4:eventstream",
	}
	sh.CreateSubs(eventStreams, &msgStream)
	sh.Listen()

	db := new(RedisComponent)
	rdb, _ := redisurl.ConnectToURL("redis://localhost:6379")
	db.Register(rdb, &msgStream)
	db.Process()

	for {
		select {
		case msg := <-db.dataStream:
			fmt.Println(msg)
		}
	}
}
