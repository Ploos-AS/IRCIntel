package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Submitter interface {
	Submit(context.Context, Observation) error
}

type WriterSubmitter struct {
	Writer io.Writer
}

func (s WriterSubmitter) Submit(_ context.Context, observation Observation) error {
	if s.Writer == nil { return errors.New("writer submitter requires a writer") }
	return json.NewEncoder(s.Writer).Encode(observation)
}

type HTTPSubmitter struct {
	URL        string
	Token      string
	Client     *http.Client
	Retries    int
	RetryDelay time.Duration
	Sleep      func(context.Context, time.Duration) error
}

func (s HTTPSubmitter) Submit(ctx context.Context, observation Observation) error {
	if s.URL == "" { return errors.New("http submitter url is required") }
	payload, err := json.Marshal(observation)
	if err != nil { return err }
	client := s.Client
	if client == nil { client = &http.Client{Timeout: 10 * time.Second} }
	retries := s.Retries
	if retries < 0 { retries = 0 }
	delay := s.RetryDelay
	if delay <= 0 { delay = time.Second }
	sleep := s.Sleep
	if sleep == nil { sleep = sleepContext }

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(payload))
		if err != nil { return err }
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "ircintel-agent")
		if s.Token != "" { req.Header.Set("Authorization", "Bearer "+s.Token) }

		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 { return nil }
			err = fmt.Errorf("core returned HTTP %d", resp.StatusCode)
		}
		lastErr = err
		if attempt == retries { break }
		if err := sleep(ctx, delay); err != nil { return err }
		delay *= 2
	}
	return lastErr
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
