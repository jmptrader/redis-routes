package sandbox

func sandbox() {

	// EventStreams is a string array containing the
	// names of channels in Redis that you want to be subscribed
	// to for events.
	//
	// Can be registered as a group if each subscription is to share
	// a single message stream channel. (DEFAULT)
	// In this case we are deferring the creation of msgStreams
	// to the SubscribeAndServe function
	eventStreams := []string{
		"projet2500:0:eventstream",
		"projet2500:1:eventstream",
		"projet2500:2:eventstream",
		"projet2500:3:eventstream",
		"projet2500:4:eventstream",
	}

	// NewRouter will iterate over list of registered URI routes
	// to associate each route in routes array. Routes will have
	// a HandlerFunc field of type redisroutes.HandlerFunc each
	// HandlerFunc will write out to a stream the string value
	// representation
	router := NewRouter(routes)

	// SubscribeAndServe will fire off all async listeners for registered
	// channels. It will then spin up the DBComponent associated with the
	// Server and begin waiting for events on the eventStream. Another
	// handler component will be retrieving values off the dataStream
	// and emitting them as string values to the TCPSink
	redisroutes.SubscribeAndServe("redis://localhost:6379", eventStreams, router)

}
