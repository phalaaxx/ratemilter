# ratemilter

A postfix milter service for rate limiting outgoing messages. The filter also runs a web service to list, block and unblock mailboxes from sending messages. 
When a mailbox has been blocked every next message will be put into quarantine (in postfix hold queue) until manually released or removed. 
Currently the filter automatically sets a mailbox into a blocking mode when more than 200 outgoing messages are detected for no more than 30 minutes. These limits are hardcoded for now.
Outgoing maessages are checked by envelope from address and a cdb file containing all local mailboxes.

When performing automatic mailbox block, tracked messages for the past 30 minutes that have not yet been delivered will be moved into the hold queue.
