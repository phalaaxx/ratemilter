package main

import (
	"fmt"
	"os/exec"
)

/* HoldQueueMessages moves messages by a list queue IDs in hold queue */
func HoldQueueMessages(QueueIDs []string) error {
	// only run subprocess if there are queue ids in the list
	if len(QueueIDs) == 0 {
		return nil
	}
	// prepare command and pipe to its stdin
	cmd := exec.Command("/usr/bin/sudo", "/usr/sbin/postsuper", "-h", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Println("StdinPipe():", err)
		return err
	}
	// start subprocess
	if err := cmd.Start(); err != nil {
		fmt.Println("Start():", err)
		return err
	}
	// write queue ids sequentially
	for _, qid := range QueueIDs {
		if _, err := fmt.Fprintf(stdin, qid); err != nil {
			fmt.Println("Fprintf():", err)
			return err
		}
	}
	// close stdin to signal end of input
	if err := stdin.Close(); err != nil {
		fmt.Println("Close():", err)
		return err
	}
	// wait for subprocess to end
	if err := cmd.Wait(); err != nil {
		fmt.Println("Wait():", err)
		return err
	}
	return nil
}
