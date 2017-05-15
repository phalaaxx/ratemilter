/* bogomilter is a milter service for postfix */
package main

import (
	"bytes"
	"fmt"
	"sync"
	"time"
)

/* Mailbox object to keep outgoing rate */
type Mailbox struct {
	Name    string
	Blocked bool
	SentLog []time.Time
}

/* Size returns approximate size of memory consumed by Mailbox object */
func (m Mailbox) Size() uint64 {
	return uint64(len(m.Name) + len(m.SentLog)*24 + 1)
}

/* MarshalJSON implements the json.Marshaller interface */
func (m Mailbox) MarshalJSON() ([]byte, error) {
	buffer := new(bytes.Buffer)
	fmt.Fprintf(buffer,
		`{"name":"%s","blocked":%v,"count":%d}`,
		m.Name,
		m.Blocked,
		len(m.SentLog),
	)
	return buffer.Bytes(), nil
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

/* Size returns approximate size in bytes used in memory cache */
func (m *MailboxMemoryCache) Size() uint64 {
	var size uint64
	for _, mailbox := range m.Data {
		size += mailbox.Size()
	}
	return size
}

/* MarshalJSON implements json.Marshaler interface */
func (m *MailboxMemoryCache) MarshalJSON() ([]byte, error) {
	buffer := new(bytes.Buffer)
	// write opening bracket
	if _, err := buffer.WriteString(fmt.Sprintf(`{"cache":%d,"mailboxes":[`, m.Size())); err != nil {
		return nil, err
	}
	// walk mailboxes
	first := true
	for _, mailbox := range m.Data {
		if b, err := mailbox.MarshalJSON(); err != nil {
			return nil, err
		} else {
			// do not write delimiter for first item in the list
			if first {
				first = false
			} else {
				// write delimiter
				if _, err := buffer.WriteRune(','); err != nil {
					return nil, err
				}
			}
			// write data
			if _, err := buffer.Write(b); err != nil {
				return nil, err
			}
		}
	}
	// write closing bracket
	if _, err := buffer.WriteString("]}"); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

/* GetBlocked returs list of blocked mailboxes */
func (m *MailboxMemoryCache) GetBlocked() ([]Mailbox) {
	var result []Mailbox
	for _, mailbox := range m.Data {
		if mailbox.Blocked {
			result = append(result, mailbox)
		}
	}
	return result
}

func NewMailboxMemoryCache() *MailboxMemoryCache {
	return &MailboxMemoryCache{Data: make(map[string]Mailbox)}
}
