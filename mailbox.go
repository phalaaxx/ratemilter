/* ratemilter is a milter service for postfix */
package main

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
	"time"
)

/* Message contains a mailbox message information */
type Message struct {
	QueueTime time.Time `json:"time"`
	QueueID   string    `json:"queueid"`
}

/* Size returns approximate in-memory size of Message data */
func (m Message) Size() uint64 {
	return uint64(len(m.QueueID) + 24)
}

/* Mailbox object to keep outgoing rate */
type Mailbox struct {
	Name     string    `json:"mailbox"`
	Blocked  bool      `json:"blocked"`
	Messages []Message `json:"messages"`
}

/* Size returns approximate size of memory consumed by Mailbox object */
func (m Mailbox) Size() uint64 {
	size := uint64(len(m.Name) + 1)
	for _, sent := range m.Messages {
		size += sent.Size()
	}
	return size
}

/* MailboxMemoryCache  */
type MailboxMemoryCache struct {
	Data  map[string]Mailbox
	Mutex sync.Mutex
}

/* IsBlocked returns true if mailbox is blocked from sending emails */
func (m *MailboxMemoryCache) IsBlocked(name, QueueID string, RateLimit int, Duration time.Duration) bool {
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
	var Messages []Message
	for _, message := range mailbox.Messages {
		if threshold.Before(message.QueueTime) {
			Messages = append(Messages, message)
		}
	}
	// add current timestamp and replace Messages
	Messages = append(Messages, Message{time.Now(), QueueID})
	mailbox.Messages = Messages
	// check if RateLimit is exceeded
	if len(mailbox.Messages) >= RateLimit {
		mailbox.Blocked = true
		// get list of tracked QueueIDs for current mailbox
		var QueueIDs []string
		for _, message := range mailbox.Messages {
			QueueIDs = append(QueueIDs, message.QueueID)
		}
		// hold old messages that are still in the queue
		go HoldQueueMessages(QueueIDs)
		// TODO: sent notification to administrator
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
		var Messages []Message
		for _, message := range mailbox.Messages {
			if threshold.Before(message.QueueTime) {
				Messages = append(Messages, message)
			}
		}
		// remove cache for expired records
		if len(Messages) == 0 {
			delete(m.Data, name)
			continue
		}
		// set new log
		mailbox.Messages = Messages
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

/* RenderJson serializes cache data in json format */
func (m *MailboxMemoryCache) RenderJson() ([]byte, error) {
	buffer := new(bytes.Buffer)
	// write opening bracket
	if _, err := buffer.WriteString(fmt.Sprintf(`{"cache":%d,"mailboxes":[`, m.Size())); err != nil {
		return nil, err
	}
	// get sorted list of mailbox names
	var list []string
	for _, mailbox := range m.Data {
		list = append(list, mailbox.Name)
	}
	sort.Strings(list)
	// walk mailboxes
	first := true
	for _, mailboxName := range list {
		// write comma as record separator
		if first {
			first = false
		} else {
			if _, err := buffer.WriteRune(','); err != nil {
				return nil, err
			}
		}
		// dump mailbox data
		mailbox := m.Data[mailboxName]
		_, err := fmt.Fprintf(buffer,
			`{"name":"%s","blocked":%v,"count":%d}`,
			mailbox.Name,
			mailbox.Blocked,
			len(mailbox.Messages),
		)
		if err != nil {
			return nil, err
		}
	}
	// write closing bracket
	if _, err := buffer.WriteString("]}"); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

/* GetBlocked returs list of blocked mailboxes */
func (m *MailboxMemoryCache) GetBlocked() []Mailbox {
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
