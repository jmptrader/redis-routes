package redisroutes

import (
	"fmt"
	"io"
	"regexp"
	"sync"

	"github.com/soveran/redisurl"
)

// Objects implementing the Handler interface can be
// registered to serve a particular URI in the Redis
// subscription server
type Handler interface {
	Serve(w io.Writer, e *Event)
}

type HandlerFunc func(io.Writer, *Event)

func (f HandlerFunc) Serve(w io.Writer, e *Event) {
	f(w, e)
}

// Helper handlers

// Error outputs a specified error
// The error message should be plain text
func Error(w io.Writer, error string) {
	fmt.Fprintln(w, error)
}

// NotFound replies to the event with an error message indicating route
// not able to be located
func NotFound(w io.Writer, e *Event) {
	msg := fmt.Sprintf("URI not registered (%s)", e.URI)
	Error(w, msg)
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
	mu       sync.RWMutex
	h        Handler
	pattern  string
	explicit bool
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
func (mux *ServeMux) Serve(w io.Writer, e *Event) {
	h, _ := mux.Handler(e)
	h.Serve(w, e)
}

// Handle registers the handler for the given pattern
// If a handler already exists for pattern, Handle panics.
func (mux *ServeMux) Handle(pattern string, handler Handler) {
	mux.mu.Lock()
	defer mux.mu.Unlock()

	if pattern == "" {
		panic("invalid pattern " + pattern)
	}
	if handler == nil {
		panic("nil handler")
	}
	if mux.m[pattern].explicit {
		panic(fmt.Sprintf("multiple registrations for %s\n", pattern))
	}

	mux.m[pattern] = muxEntry{explicit: true, h: handler, pattern: pattern}
}

// HandleFunc registers the handler function for the given pattern
func (mux *ServeMux) HandleFunc(pattern string, handler func(io.Writer, *Event)) {
	mux.Handle(pattern, HandlerFunc(handler))
}

// Handle registers the handler for the given pattern
// in the DefaultServeMux
// The documentation for ServeMux explains how patterns are matched
func Handle(pattern string, handler Handler) { DefaultServeMux.Handle(pattern, handler) }

// HandleFunc registers the handler function for the given pattern
// in the DefaultServeMux
// The documentation for ServeMux explains how patterns are matched
func HandleFunc(pattern string, handler func(io.Writer, *Event)) {
	DefaultServeMux.HandleFunc(pattern, handler)
}

// Server defines parameters for connecting to a Redis instance
// and serving event channgels
type Server struct {
	Addr       string  // Address to connect to redis instance, "localhost:6379" if empty
	Handler    Handler // Handler to invoke DefaultServeMux if nil
	SubHandler SubscriptionHandler
}

// serverHandler delegates to either the server's handler or
// DefaultServeMux
type serverHandler struct {
	srv *Server
}

func (sh serverHandler) Serve(w io.Writer, e *Event) {
	handler := sh.srv.Handler
	if handler == nil {
		handler = DefaultServeMux
	}
	handler.Serve(w, e)
}

// SubscribeAndServe will subscribe to all provided channels
// on srv.Addr and then calls Serve to handle events on subscribed
// Redis channels. If srv.Addr is blank then "localhost" is used
func (srv *Server) SubscribeAndServe(addr string, subscriptions []string, wr io.Writer) error {
	if addr == "" {
		addr = "redis://localhost:6379"
	}
	srv.Addr = addr

	srv.SubHandler = SubscriptionHandler{
		index: make(map[string]subscription),
		addr:  addr,
	}

	conn, err := redisurl.ConnectToURL(addr)
	if err != nil {
		panic(err)
	}

	// Register subscriptions and start listening
	msgStream := make(chan string)
	srv.SubHandler.CreateSubs(subscriptions, &msgStream)
	srv.SubHandler.Listen()

	// Spin up DBComponent and set to listen on same msgStream
	db := new(RedisComponent)
	db.Register(conn, &msgStream)
	db.Process()

	// Begin serving messages output to dataStream
	return srv.Serve(wr, db.dataStream)
}

func SubscribeAndServe(addr string, subscriptions []string, wr io.Writer) error {
	srv := new(Server)
	return srv.SubscribeAndServe(addr, subscriptions, wr)
}

// Convenience wrapper allowing caller to omit the Redis DB address
func LocalSubscribeAndServe(subscriptions []string, wr io.Writer) error {
	return SubscribeAndServe("", subscriptions, wr)
}

// Serve will kick off listener for each subscription and then
// spin and wait for msg events on the event channel. Each event
// that arrivees on the channel will have handler called.
func (srv *Server) Serve(w io.Writer, dataStream chan Event) error {
	// Create serverhandler
	for {
		select {
		case event := <-dataStream:
			go serverHandler{srv}.Serve(w, &event)
		}

	}
}
