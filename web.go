/* bogomilter is a milter service for postfix */
package main

import (
	"fmt"
	"net/http"
	"text/template"
)

/* ListTemplate returns information about currently tracked mailboxes */
const TemplateStats = `<html>
<head>
</head>
<body>
	<h3>List of mailboxes that have sent outgoing emails in the past 30 minutes</h3>
	<hr>
	<table>
	<thead style="font-weight: bold">
		<tr>
			<td width=400px>Name</td>
			<td>#</td>
			<td>Blocked</td>
			<td>Options</td>
		</tr>
	</thead>
	<tbody>
	{{ range $mb := . }}
		{{if $mb.Blocked }}<tr style="background: red">{{ else }}<tr>{{ end }}
			<td>{{ $mb.Name }}</td>
			<td>{{ len $mb.SentLog }}</td>
			<td>{{ $mb.Blocked }}</td>
			<td>
				{{ if $mb.Blocked }}
				<a href="/unblock?mailbox={{$mb.Name}}"><strong>unblock</strong></a>
				{{ else }}
				<a href="/block?mailbox={{$mb.Name}}"><strong>block</strong></a>
				{{ end }}
			</td>
		</tr>
	{{ end }}
	</tbody>
	</table>
</body>
</html>`

/* viewListMailboxes lists stats for mailboxes being tracked */
func viewListMailboxes(w http.ResponseWriter, r *http.Request) {
	// acquire lock
	MailboxMap.Mutex.Lock()
	defer MailboxMap.Mutex.Unlock()
	// dump mailboxes in json format
	Template := template.Must(template.New("stats").Parse(TemplateStats))
	if err := Template.Lookup("stats").Execute(w, MailboxMap.Data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

/* viewBlockMailbox adds a mailbox to the block list */
func viewBlockMailbox(w http.ResponseWriter, r *http.Request) {
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
}

/* viewUnblockMailbox removes a mailbox from the block list */
func viewUnblockMailbox(w http.ResponseWriter, r *http.Request) {
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
}
