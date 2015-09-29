package redisroutes

import (
	"time"

	"github.com/garyburd/redigo/redis"
)

// RedisComponent dataflow structure
type RedisComponent struct {
	conn       redis.Conn
	msgStream  *chan string
	dataStream chan Event
}

// Register will initialize the RedisComponent with a database connection
// and a msgStream to parse for emitting output tuples.
func (rc *RedisComponent) Register(conn redis.Conn, msgStream *chan string) {
	rc.conn = conn
	rc.msgStream = msgStream
	rc.dataStream = make(chan Event)
}

// Process will start the DBComponent listening to the registered
// msgStream. Each msg that is received is understood to be a key
// in the database. The component will lookup this key and emit
// a new event on the dataStream with the URI, value, and timestamp
func (rc RedisComponent) Process() {
	go func() {
		for {
			select {
			case msg := <-*rc.msgStream:
				// Lookup key in database
				val, _ := rc.conn.Do("GET", msg)
				// Build event structure
				e := Event{
					URI:   msg,
					Value: val,
					Time:  time.Now().UTC(),
				}
				// Write event to outputStream
				rc.dataStream <- e
			}
		}
	}()
}
