// Package notify tells the user, outside the forge, that an issue needs them. Sending is a side channel: a failure
// is the caller's to record, and never stops the work it reports on.
package notify

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

// Telegram sends messages through the Telegram Bot API.
type Telegram struct {
	Token   string
	ChatID  string
	BaseURL string        // https://api.telegram.org when empty
	HTTP    *http.Client  // http.DefaultClient when nil
	Timeout time.Duration // 10s when zero
}

func (t *Telegram) timeout() time.Duration {
	if t.Timeout > 0 {
		return t.Timeout
	}
	return 10 * time.Second
}

// Send posts text to the chat as plain text: no parse_mode, so nothing in it is read as markup.
//
// The request URL carries the bot token, and net/http puts the URL into its errors. Send therefore never returns
// such an error as is; it reports what went wrong without the URL.
func (t *Telegram) Send(ctx context.Context, text string) error {
	base := t.BaseURL
	if base == "" {
		base = "https://api.telegram.org"
	}
	body, err := json.Marshal(map[string]string{"chat_id": t.ChatID, "text": text})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, t.timeout())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/bot"+t.Token+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return errors.New("telegram: the request could not be built")
	}
	req.Header.Set("Content-Type", "application/json")
	client := t.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("telegram: no answer within %v", t.timeout())
		case errors.Is(err, context.Canceled):
			return errors.New("telegram: canceled")
		}
		return errors.New("telegram: the request failed before an answer")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	var out struct {
		Description string `json:"description"`
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	_ = json.Unmarshal(data, &out)
	return fmt.Errorf("telegram: %d %s", resp.StatusCode, out.Description)
}
