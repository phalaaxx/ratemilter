/* bogomilter is a milter service for postfix */
package main

import (
	"sync"
	"time"
)

/* Mailbox object to keep outgoing rate */
type Mailbox struct {
	Name    string
	Blocked bool
	SentLog []time.Time
}

/* MailboxMemoryCache  */
type MailboxMemoryCache struct {
	Data  map[string]Mailbox
	Mutex sync.Mutex
}

/* IsBlocked returns true if mailbox is blocked from sending emails */
func (m *MailboxMemoryCache) IsBlocked(name string, RateLimit int, Duration time.Duration) bool {
	// acquire mutex lock
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	// get mailbox object
	mailbox, ok := m.Data[name]
	if !ok {
		// mailbox object does not exist, create a new one
		mailbox = Mailbox{
			Name:    name,
			Blocked: false,
		}
	}
	// check if mailbox is already blocked
	if mailbox.Blocked {
		return true
	}
	// clean up sent log
	threshold := time.Now().Add(-Duration)
	var SentLog []time.Time
	for _, ts := range mailbox.SentLog {
		if threshold.Before(ts) {
			SentLog = append(SentLog, ts)
		}
	}
	// add current timestamp and replace SentLog
	SentLog = append(SentLog, time.Now())
	mailbox.SentLog = SentLog
	// check if RateLimit is exceeded
	if len(mailbox.SentLog) >= RateLimit {
		mailbox.Blocked = true
		// TODO: sent notification to administrator
		// run in a goroutine to avoid excessive lock
	}
	m.Data[name] = mailbox
	return mailbox.Blocked
}

/* CleanUp unused memory */
func (m *MailboxMemoryCache) CleanUp(Duration time.Duration) {
	// acquire lock
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	// walk all mailboxes
	threshold := time.Now().Add(-Duration)
	for name, mailbox := range m.Data {
		// blocked mailboxes should stay blocked
		if mailbox.Blocked {
			continue
		}
		// clean up expired records
		var SentLog []time.Time
		for _, ts := range mailbox.SentLog {
			if threshold.Before(ts) {
				SentLog = append(SentLog, ts)
			}
		}
		// remove cache for expired records
		if len(SentLog) == 0 {
			delete(m.Data, name)
			continue
		}
		// set new log
		mailbox.SentLog = SentLog
		m.Data[name] = mailbox
	}
}

func NewMailboxMemoryCache() *MailboxMemoryCache {
	return &MailboxMemoryCache{Data: make(map[string]Mailbox)}
}
