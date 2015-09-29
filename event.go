package redisroutes

import (
	"fmt"
	"time"
)

type Event struct {
	URI   string
	Value interface{}
	Time  time.Time
}

func (e Event) String() string {
	return fmt.Sprintf("%v|%s|%s", e.Time, e.URI, e.Value)
}
