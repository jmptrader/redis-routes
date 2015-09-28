package redisroutes

import (
	"fmt"
	"regexp"
	"sync"
)

// Objects implementing the Handler interface can be
// registered to serve a particular URI in the Redis
// subscription server
type Handler interface {
	Serve(*Event)
}

type Event struct {
	URI string
}

type HandlerFunc func(*Event)

func (f HandlerFunc) Serve(e *Event) {
	f(e)
}

// Helper handlers

// Error outputs a specified error
// The error message should be plain text
func Error(error string) {
	fmt.Printf("Error: %s\n", error)
}

// NotFound replies to the event with an error message indicating route
// not able to be located
func NotFound(e *Event) {
	msg := fmt.Sprintf("Route (%s) not found\n", e.URI)
	Error(msg)
}

// NotFoundHandler returns a simple handler
func NotFoundHandler() Handler { return HandlerFunc(NotFound) }

// ServeMux is a Redis channel event multiplexer.
// It matches the URI of each incoming event against a list of registered
// patterns and calls the handler for the pattern that matches the URI.
//
// Patterns can be any arbitrary string
type ServeMux struct {
	mu sync.RWMutex
	m  map[string]muxEntry
}

type muxEntry struct {
	mu      sync.RWMutex
	h       Handler
	pattern string
}

// NewServeMux allocates and returns a new ServeMux
func NewServeMux() *ServeMux { return &ServeMux{m: make(map[string]muxEntry)} }

// DefaultServeMux is the default ServeMux used by Serve
var DefaultServeMux = NewServeMux()

// Does the URI match pattern?
func uriMatch(pattern, uri string) bool {
	// Fail empty paths
	if len(pattern) == 0 {
		return false
	}

	// Regex match the pattern to the URI
	match, err := regexp.MatchString(pattern, uri)
	if err != nil {
		// Error?
		return false
	}
	return match
}

// Find a handler on a handler map given a path string
// First registered match will win
func (mux *ServeMux) match(uri string) (h Handler, pattern string) {
	var n = 0
	for k, v := range mux.m {
		if !uriMatch(k, uri) {
			continue
		}
		return v.h, v.pattern
	}
	return
}

// Handler returns the handler to use for the given request
func (mux *ServeMux) Handler(e *Event) (h Handler, pattern string) {
	// Apply URI sanitization
	// ...
	return mux.handler(e.URI)
}

// handler is the main implementation of handler
func (mux *ServeMux) handler(uri string) (h Handler, pattern string) {
	mux.mu.RLock()
	defer mux.mu.RUnlock()

	h, pattern = mux.match(uri)

	if h == nil {
		h, pattern = NotFoundHandler(), ""
	}
	return
}

// Serve dispatches the event to the handler whose
// pattern matches the event URI
func (mux *ServeMux) Serve(e *Event) {
	h, _ := mux.Handler(e)
	h.Serve(e)
}

// Handle registers the handler for the given pattern
// If a handler already exists for pattern, Handle panics.
func (mux *ServeMux) Handle(pattern string, handler Handler) {
	mux.mu.Lock()

	if pattern == "" {
		panic("invalid pattern " + pattern)
	}
	if handler == nil {
		panic("nil handler")
	}
	if mux.m[pattern].explicit {
		panic("multiple registrations for ", pattern)
	}

	mux.m[pattern] = muxEntry{explicit: true, h: handler, pattern: pattern}
}

// HandleFunc registers the handler function for the given pattern
func (mux *ServeMux) HandleFunc(pattern string, handler func(*Event)) {
	mux.Handle(pattern, HandlerFunc(handler))
}

// Handle registers the handler for the given pattern
// in the DefaultServeMux
// The documentation for ServeMux explains how patterns are matched
func Handle(pattern string, handler Handler) { DefaultServeMux.Handle(pattern, handler) }

// HandleFunc registers the handler function for the given pattern
// in the DefaultServeMux
// The documentation for ServeMux explains how patterns are matched
func HandleFunc(pattern string, handler func(*Event)) {
	DefaultServeMux.HandleFunc(pattern, handler)
}

// Server defines parameters for connecting to a Redis instance
// and serving event channgels
type Server struct {
	Addr    string  // Address to connect to redis instance, "localhost:6379" if empty
	Handler Handler // Handler to invoke DefaultServeMux if nil
}

// serverHandler delegates to either the server's handler or
// DefaultServeMux
type serverHandler struct {
	srv *Server
}

func (sh serverHandler) Serve(e *Event) {
	handler := sh.srv.Handler
	if handler == nil {
		handler = DefaultServeMux
	}
	handler.Serve(e)
}

// SubscribeAndServe will subscribe to all provided channels
// on srv.Addr and then calls Serve to handle events on subscribed
// Redis channels. If srv.Addr is blank then "localhost" is used
func (srv *Server) SubscribeAndServe() error {
	addr := srv.Addr
	if addr == "" {
		addr = "redis://localhost:6379"
	}
	conn, err := redisurl.ConnectToURL(addr)
	if err != nil {
		panic(err)
	}
	return srv.Serve()
}

// Serve will kick off listener for each subscription and then
// spin and wait for msg events on the event channel. Each event
// that arrivees on the channel will have handler called.
func (srv *Server) Serve() error {
	for {

	}
}

// Subscription Handling
type SubscriptionHandler struct {
	index map[string]subscription
}

// Maps a url to redis Subscribe channel
type subscription struct {
	psc         *redis.PubSubClient
	eventStream string
	msgStream   chan string
}

func (sh *SubscriptionHandler) CreateSub(eventStream string, msgStream *chan string) {
	conn, err := redisurl.ConnectToURL("redis://localhost:6379")
	if err != nil {
		panic(err)
	}

	psc := redis.PubSubConn{Conn: conn}
	ch, err := psc.Subscribe(eventStream)
	if err != nil {
		panic(err)
	}

	sh.index[eventStream] = subscription{&psc, eventStream, msgStream}
}

// Register an array of subscriptions
func (sh *SubscriptionHandler) CreateSubs(eventStreams []string, msgStream *chan string) {
	for eventStream := range eventStreams {
		sh.CreateSub(eventStream, msgStream)
	}
}

// Begin listening on all subscriptions
// Each subscription will concurrently handle events and output to
// their associated msgStream
func (sh *SubscriptionHandler) Listen() {
	for name, sub := range sh.index {
		go sub.listen()
	}
}

// listen for published events and send to message channel
func (s subscription) listen() {
	for {
		switch v := s.psc.Receive().(type) {
		case redis.Message:
			s.msgStream <- string(v.Data)
		case error:
			return
		}
	}
}

// Sandbox
func sandbox() {
	sh := new(SubscriptionHandler)
	sh.index = make(map[string]subscription)
	msgStream := make(chan string)

	// Can be registered individually
	sh.CreateSub("projet2500:0:eventStream", &msgStream)
	sh.CreateSub("projet2500:1:eventStream", &msgStream)

	// Can be registered as a group
	// *But only if each sub shares an outgoing msgStream
	eventStream := []string{
		"projet2500:2:eventStream",
		"projet2500:3:eventStream",
		"projet2500:4:eventStream",
	}
	sh.CreateSubs(eventStreams, &msgStream)
	sh.Listen()
}
