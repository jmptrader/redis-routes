package main

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
	return fmt.Sprintf("Key: %s\t| Val: %s\t| Time: %s", e.URI, e.Value, e.Time)
}
