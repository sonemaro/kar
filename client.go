package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// client is a minimal Jira Cloud REST v3 client: JSON in/out, client-side
// throttling, and Retry-After aware backoff on 429/5xx.
type client struct {
	site  string
	email string
	token string

	mu       sync.Mutex
	next     time.Time
	minDelay time.Duration
	hc       *http.Client
}

func newClient(site, email, token string, rps int) *client {
	if rps <= 0 {
		rps = 4
	}
	return &client{
		site:     site,
		email:    email,
		token:    token,
		minDelay: time.Second / time.Duration(rps),
		hc:       &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *client) wait() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if c.next.After(now) {
		time.Sleep(c.next.Sub(now))
		now = time.Now()
	}
	c.next = now.Add(c.minDelay)
}

// do performs one request; out may be nil. Retries 429 and 5xx.
func (c *client) do(method, path string, body, out any) error {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = b
	}
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		c.wait()
		req, err := http.NewRequest(method, c.site+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.SetBasicAuth(c.email, c.token)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.hc.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		switch {
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.Atoi(ra); err == nil && secs > 0 && secs < 120 {
					time.Sleep(time.Duration(secs) * time.Second)
				}
			}
			lastErr = fmt.Errorf("%s %s: %s (attempt %d)", method, path, resp.Status, attempt+1)
			continue
		case resp.StatusCode >= 400:
			msg := summarizedBody(data)
			return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, msg)
		}
		if out != nil && len(data) > 0 {
			if err := json.Unmarshal(data, out); err != nil {
				return fmt.Errorf("%s %s: decode response: %w", method, path, err)
			}
		}
		return nil
	}
	return lastErr
}

func summarizedBody(data []byte) string {
	var e struct {
		Errors        map[string]string `json:"errors"`
		ErrorMessages []string          `json:"errorMessages"`
		Message       string            `json:"message"`
	}
	if err := json.Unmarshal(data, &e); err == nil {
		if len(e.ErrorMessages) > 0 {
			return e.ErrorMessages[0]
		}
		if e.Message != "" {
			return e.Message
		}
		for k, v := range e.Errors {
			return k + ": " + v
		}
	}
	if len(data) > 200 {
		return string(data[:200]) + "…"
	}
	return string(data)
}

// --- typed helpers over the documented REST v3 surface ---

// rawIssue keeps the fields blob raw so callers can decode both the stable
// fields and project-specific customfield ids from one search response.
type rawIssue struct {
	Key    string          `json:"key"`
	Fields json.RawMessage `json:"fields"`
}

type statusField struct {
	Name string `json:"name"`
}

type issueFields struct {
	Summary    string      `json:"summary"`
	Status     statusField `json:"status"`
	IssueLinks []struct {
		InwardIssue struct {
			Key string `json:"key"`
		} `json:"inwardIssue"`
		OutwardIssue struct {
			Key string `json:"key"`
		} `json:"outwardIssue"`
	} `json:"issuelinks"`
}

func (r rawIssue) decode() (issueFields, error) {
	var f issueFields
	err := json.Unmarshal(r.Fields, &f)
	return f, err
}

// hasField reports whether a (custom)field id carries a non-null value.
func (r rawIssue) hasField(id string) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(r.Fields, &m); err != nil {
		return false
	}
	v, ok := m[id]
	return ok && string(v) != "null"
}

func (c *client) myself() (string, error) {
	var out struct {
		DisplayName string `json:"displayName"`
	}
	if err := c.do("GET", "/rest/api/3/myself", nil, &out); err != nil {
		return "", err
	}
	return out.DisplayName, nil
}

func (c *client) project(key string) error {
	return c.do("GET", "/rest/api/3/project/"+key, nil, nil)
}

func (c *client) fieldExists(id string) (bool, error) {
	var fields []struct {
		ID string `json:"id"`
	}
	if err := c.do("GET", "/rest/api/3/field", nil, &fields); err != nil {
		return false, err
	}
	for _, f := range fields {
		if f.ID == id {
			return true, nil
		}
	}
	return false, nil
}

// search runs a JQL query via the documented POST /rest/api/3/search/jql.
func (c *client) search(jql string, maxResults int, fieldList []string) ([]rawIssue, error) {
	body := map[string]any{
		"jql":        jql,
		"maxResults": maxResults,
		"fields":     fieldList,
	}
	var out struct {
		Issues []rawIssue `json:"issues"`
	}
	if err := c.do("POST", "/rest/api/3/search/jql", body, &out); err != nil {
		return nil, err
	}
	return out.Issues, nil
}

func (c *client) transitionTo(key, status string) error {
	var list struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"transitions"`
	}
	if err := c.do("GET", "/rest/api/3/issue/"+key+"/transitions", nil, &list); err != nil {
		return err
	}
	for _, t := range list.Transitions {
		if strings.EqualFold(t.Name, status) {
			return c.do("POST", "/rest/api/3/issue/"+key+"/transitions",
				map[string]any{"transition": map[string]string{"id": t.ID}}, nil)
		}
	}
	return fmt.Errorf("no transition to %q available on %s", status, key)
}

func (c *client) editFields(key string, fields map[string]any) error {
	return c.do("PUT", "/rest/api/3/issue/"+key, map[string]any{"fields": fields}, nil)
}

func (c *client) addComment(key, text string) error {
	return c.do("POST", "/rest/api/3/issue/"+key+"/comment",
		map[string]any{"body": textToADF(text)}, nil)
}

func (c *client) relate(keyA, keyB string) error {
	body := map[string]any{
		"type":         map[string]string{"name": "Relates"},
		"inwardIssue":  map[string]string{"key": keyA},
		"outwardIssue": map[string]string{"key": keyB},
	}
	return c.do("POST", "/rest/api/3/issueLink", body, nil)
}

// intervalValue encodes a date the way Jira Product Discovery "interval"
// fields expect on the wire: a JSON object serialised as a string.
func intervalValue(date string) string {
	b, _ := json.Marshal(map[string]string{"start": date, "end": date})
	return string(b)
}
