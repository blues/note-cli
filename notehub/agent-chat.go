// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"net/http"
)

const agentChatURL = "https://chat.notehub.io"

// agentChat prints the public entry guide exactly as served, without authentication.
func agentChat(out io.Writer, client *http.Client) error {
	rsp, err := client.Get(agentChatURL)
	if err != nil {
		return fmt.Errorf("agent chat: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode < 200 || rsp.StatusCode >= 300 {
		return fmt.Errorf("agent chat: %s returned HTTP %s", agentChatURL, rsp.Status)
	}
	if _, err := io.Copy(out, rsp.Body); err != nil {
		return fmt.Errorf("agent chat: reading or writing guidance: %w", err)
	}
	return nil
}
