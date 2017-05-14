/* bogomilter is a milter service for postfix */
package main

import (
	"flag"
	//"fmt"
	"github.com/phalaaxx/cdb"
	"github.com/phalaaxx/milter"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

/* global variables */
var LocalCdb string

/* MailboxMap is an in-memory mailbox cache */
var MailboxMap *MailboxMemoryCache

/* BogoMilter object */
type BogoMilter struct {
	milter.Milter
}

/* MailFrom is called on envelope from address */
func (b *BogoMilter) MailFrom(from string, m *milter.Modifier) (milter.Response, error) {
	// only process outgoing emails
	if VerifyLocal(from) {
		if MailboxMap.IsBlocked(from, 200, time.Minute*30) {
			// blocked mailbox, quarantine
			m.Quarantine("rate limit")
		}
	}
	return milter.RespContinue, nil
}

func (b *BogoMilter) Body(m *milter.Modifier) (milter.Response, error) {
	return milter.RespContinue, nil
}

/* VerifyLocal checks if local database contains named mailbox address */
func VerifyLocal(name string) bool {
	var value *string
	err := cdb.Lookup(
		LocalCdb,
		func(db *cdb.Reader) (err error) {
			value, err = db.Get(name)
			return err
		},
	)
	if err == nil && value != nil && len(*value) != 0 {
		return true
	}
	return false
}

/* RunServer creates and runs new BogoMilter server */
func RunServer(socket net.Listener) {
	// declare milter init function
	init := func() (milter.Milter, uint32, uint32) {
		return &BogoMilter{},
			milter.OptQuarantine,
			milter.OptNoConnect | milter.OptNoHelo | milter.OptNoRcptTo | milter.OptNoBody | milter.OptNoHeaders | milter.OptNoEOH
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
	var protocol, address string
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
	http.ListenAndServe(":1704", nil)
}
