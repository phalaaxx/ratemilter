/* bogomilter is a milter service for postfix */
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

/* MailboxStats contains information about milter status */
type MailboxStats struct {
	Mailboxes []Mailbox `json:"mailboxes"`
	CacheSize uint64    `json:"cache"`
}

/* viewApiHandler handles API calls to the milter */
func viewApiHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// acquire lock
		MailboxMap.Mutex.Lock()
		defer MailboxMap.Mutex.Unlock()
		// if monitor parameter is provided only render list of blocked mailboxes
		switch r.URL.Query().Get("monitor") {
		case "true":
			// compile list of mailbox names
			var list []string
			for _, mailbox := range MailboxMap.GetBlocked() {
				list = append(list, mailbox.Name)
			}
			// return OK if list is empty
			if len(list) == 0 {
				fmt.Fprintf(w, "OK")
			} else {
				fmt.Fprintf(w, "blocked:%s", strings.Join(list, ","))
			}
		default:
			// render json data
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(MailboxMap); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	case "POST":
		query := r.URL.Query()
		switch query.Get("method") {
		case "block":
			// get mailbox name
			mailboxName := r.URL.Query().Get("mailbox")
			if !VerifyLocal(mailboxName) {
				http.Error(w, "unknown mailbox", http.StatusNotFound)
				return
			}
			// acquire lock
			MailboxMap.Mutex.Lock()
			defer MailboxMap.Mutex.Unlock()
			// look up existing mailbox object
			mailbox, ok := MailboxMap.Data[mailboxName]
			if !ok {
				mailbox = Mailbox{Name: mailboxName}
			}
			// block and save mailbox to cache
			mailbox.Blocked = true
			MailboxMap.Data[mailboxName] = mailbox
			// return ok status
			fmt.Fprintf(w, "OK")
		case "unblock":
			// get mailbox name
			mailboxName := r.URL.Query().Get("mailbox")
			if !VerifyLocal(mailboxName) {
				http.Error(w, "unknown mailbox", http.StatusNotFound)
				return
			}
			// acquire lock
			MailboxMap.Mutex.Lock()
			defer MailboxMap.Mutex.Unlock()
			// look up mailbox object from cache
			mailbox, ok := MailboxMap.Data[mailboxName]
			if !ok || !mailbox.Blocked {
				fmt.Fprintf(w, "not blocked")
				return
			}
			// unblock and save to cache
			mailbox.Blocked = false
			MailboxMap.Data[mailboxName] = mailbox
			// return ok status
			fmt.Fprintf(w, "OK")
		default:
			http.Error(w, "unknown method parameter", http.StatusBadRequest)
		}
	}
}
