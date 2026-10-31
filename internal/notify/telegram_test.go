package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const token = "123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw0"

func TestSendPostsPlainText(t *testing.T) {
	var path string
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.Method + " " + r.URL.Path
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	tg := &Telegram{Token: token, ChatID: "-10042", BaseURL: srv.URL}
	if err := tg.Send(context.Background(), "<b>hi</b>"); err != nil {
		t.Fatal(err)
	}
	if path != "POST /bot"+token+"/sendMessage" {
		t.Fatalf("path = %q", path)
	}
	if got["chat_id"] != "-10042" || got["text"] != "<b>hi</b>" {
		t.Fatalf("body = %v", got)
	}
	if _, ok := got["parse_mode"]; ok {
		t.Fatalf("parse_mode was sent: %v", got)
	}
}

func TestSendKeepsTheTokenOutOfErrors(t *testing.T) {
	for name, base := range map[string]string{
		"refused": "http://127.0.0.1:1",
		"bad url": "http://[::1",
	} {
		t.Run(name, func(t *testing.T) {
			tg := &Telegram{Token: token, ChatID: "1", BaseURL: base}
			err := tg.Send(context.Background(), "x")
			if err == nil {
				t.Fatal("no error")
			}
			if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "AAHdq") {
				t.Fatalf("token in error: %v", err)
			}
		})
	}
}

func TestSendReportsTheAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"ok":false,"description":"Bad Request: chat not found"}`))
	}))
	defer srv.Close()
	err := (&Telegram{Token: token, ChatID: "1", BaseURL: srv.URL}).Send(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "chat not found") {
		t.Fatalf("err = %v", err)
	}
}

func TestSendGivesUpAfterTheTimeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-block }))
	defer srv.Close()
	defer close(block)
	tg := &Telegram{Token: token, ChatID: "1", BaseURL: srv.URL, Timeout: 50 * time.Millisecond}
	start := time.Now()
	if err := tg.Send(context.Background(), "x"); err == nil {
		t.Fatal("no error")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("took %v", d)
	}
	if (&Telegram{}).timeout() != 10*time.Second {
		t.Fatal("default timeout is not 10s")
	}
}
