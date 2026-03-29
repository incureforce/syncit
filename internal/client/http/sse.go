package clienthttp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// StreamEvents reads the server's SSE stream until ctx is done or the connection errors.
// Each complete "data:" payload (JSON) is passed to onData. Comment and ping lines are ignored.
func (c *Client) StreamEvents(ctx context.Context, clientID string, onData func([]byte) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, joinURL(c.BaseURL, "/v1/client/events"), nil)
	if err != nil {
		return err
	}
	req.Header.Set(HeaderClientID, clientID)
	req.Header.Set("Accept", "text/event-stream")

	hc := c.httpClient()
	if hc.Timeout > 0 {
		nc := *hc
		nc.Timeout = 0
		hc = &nc
	}

	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("events: %s: %s", resp.Status, string(b))
	}

	br := bufio.NewReader(resp.Body)
	var dataLines []string
	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		payload := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		return onData([]byte(payload))
	}

	for {
		line, err := br.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return flush()
			}
			return err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
}
