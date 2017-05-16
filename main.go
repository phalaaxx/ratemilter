/* ratemilter is a milter service for postfix */
package main

import (
	"flag"
	//"fmt"
	"github.com/phalaaxx/milter"
	"log"
	"net"
	"net/textproto"
	"net/http"
	"os"
	"time"
)

/* MailboxMap is an in-memory mailbox cache */
var MailboxMap *MailboxMemoryCache

/* RateMilter object */
type RateMilter struct {
	milter.Milter
	from string
}

/* MailFrom is called on envelope from address */
func (b *RateMilter) MailFrom(from string, m *milter.Modifier) (milter.Response, error) {
	// look for authentication token
	if _, ok := m.Macros["{auth_authen}"]; !ok {
		// user is not authenticated, ignore
		return milter.RespAccept, nil
	}
	// save from address
	b.from = from
	return milter.RespContinue, nil
}

/* Header handles processing individual headers */
func (b *RateMilter) Header(header, value string, m *milter.Modifier) (milter.Response, error) {
	return milter.RespContinue, nil
}

/* Headers handles end of headers milter callback */
func (b *RateMilter) Headers(headers textproto.MIMEHeader, m *milter.Modifier) (milter.Response, error) {
	// only process outgoing emails
	QueueID := m.Macros["i"]
	if MailboxMap.IsBlocked(b.from, QueueID, 200, time.Minute*30) {
		// blocked mailbox, quarantine
		m.Quarantine("rate limit")
	}
	return milter.RespContinue, nil
}

func (b *RateMilter) Body(m *milter.Modifier) (milter.Response, error) {
	return milter.RespContinue, nil
}

/* RunServer creates and runs new RateMilter server */
func RunServer(socket net.Listener) {
	// declare milter init function
	init := func() (milter.Milter, uint32, uint32) {
		return &RateMilter{},
			milter.OptQuarantine,
			milter.OptNoConnect | milter.OptNoHelo | milter.OptNoRcptTo | milter.OptNoBody
	}
	// start server
	if err := milter.RunServer(socket, init); err != nil {
		log.Fatal(err)
	}
}

/* CleanUpLoop cleans up expired records from cache */
func CleanUpLoop() {
	for {
		time.Sleep(time.Minute)
		MailboxMap.CleanUp(time.Minute * 30)
	}
}

/* main program */
func main() {
	// parse commandline arguments
	var protocol, address, HttpBind string
	flag.StringVar(&protocol,
		"proto",
		"unix",
		"Protocol family (unix or tcp)")
	flag.StringVar(&address,
		"addr",
		"/var/spool/postfix/milter/rate.sock",
		"Bind to address or unix domain socket")
	flag.StringVar(&HttpBind,
		"http",
		":1704",
		"HTTP server bind address")
	flag.Parse()
	// make sure the specified protocol is either unix or tcp
	if protocol != "unix" && protocol != "tcp" {
		log.Fatal("invalid protocol name")
	}
	// make sure socket does not exist
	if protocol == "unix" {
		// ignore os.Remove errors
		os.Remove(address)
	}
	// bind to listening address
	socket, err := net.Listen(protocol, address)
	if err != nil {
		log.Fatal(err)
	}
	defer socket.Close()
	// remove old unix domain socket if exists
	if protocol == "unix" {
		// set mode 0660 for unix domain sockets
		if err := os.Chmod(address, 0660); err != nil {
			log.Fatal(err)
		}
		// remove socket on exit
		defer os.Remove(address)
	}
	// prepare memory cache
	MailboxMap = NewMailboxMemoryCache()
	// run server
	go RunServer(socket)
	go CleanUpLoop()
	// run http server
	http.HandleFunc("/", viewApiHandler)
	http.ListenAndServe(HttpBind, nil)
}
