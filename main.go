/* ratemilter is a milter service for postfix */
package main

import (
	"flag"
	"fmt"
	"github.com/phalaaxx/milter"
	"log"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"os/signal"
	"time"
)

/* MailboxMap is an in-memory mailbox cache */
var MailboxMap *MailboxMemoryCache
var LocalCdb string

/* RateMilter object */
type RateMilter struct {
	milter.Milter
	From string
	Addr net.IP
}

/* Connect handles new smtp connection */
func (b *RateMilter) Connect(_, _ string, _ uint16, addr net.IP, m *milter.Modifier) (milter.Response, error) {
	// save remote address
	b.Addr = addr
	return milter.RespContinue, nil
}

/* MailFrom is called on envelope from address */
func (b *RateMilter) MailFrom(from string, m *milter.Modifier) (milter.Response, error) {
	// ignore messages with empty envelope from
	if len(from) == 0 {
		return milter.RespAccept, nil
	}
	// look for authentication token
	if _, ok := m.Macros["{auth_authen}"]; !ok {
		// make sure mailbox is not local
		if b.Addr.String() != "127.0.0.1" || !VerifyLocal(from) {
			return milter.RespAccept, nil
		}
	}
	// save from address
	b.From = from
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
	if MailboxMap.IsBlocked(b.From, QueueID, 200, time.Minute*30) {
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
	init := func() (milter.Milter, milter.OptAction, milter.OptProtocol) {
		return &RateMilter{},
			milter.OptQuarantine,
			milter.OptNoHelo | milter.OptNoRcptTo | milter.OptNoBody
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

/* SigintHandler handles an INT signal sent to ratemilter */
func SigintHandler(c chan os.Signal, sock net.Listener) {
	// wait for sigint
	<-c
	// save data to persistent storage
	if err := SaveMemoryCache(MailboxMap); err != nil {
		fmt.Println("SaveMemoryMap(): %v\n", err)
	}
	// gracefully stop the web server
	sock.Close()
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
	flag.StringVar(&LocalCdb,
		"cdb",
		"/etc/postfix/cdb/virtual-mailbox-maps.cdb",
		"A cdb database containing list of all local mailboxes")
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
	// load data from persistent storage
	if err := LoadMemoryCache(MailboxMap); err != nil {
		fmt.Printf("LoadMemoryCache(): %v\n", err)
	}
	// run server
	go RunServer(socket)
	go CleanUpLoop()
	// start listener socket
	sock, err := net.Listen("tcp", HttpBind)
	if err != nil {
		log.Fatal(err)
	}
	// catch sigint
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go SigintHandler(c, sock)
	// run http server
	http.HandleFunc("/", viewApiHandler)
	http.Serve(sock, nil)
}
