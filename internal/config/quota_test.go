package config

import (
	"strings"
	"testing"
	"time"
)

func TestLimitWaitDefaultsToThirtyMinutes(t *testing.T) {
	c, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if c.Agent.LimitWait != 30*time.Minute {
		t.Errorf("agent.limit_wait = %v, want 30m", c.Agent.LimitWait)
	}
	c, err = Load(write(t, valid+"agent:\n  limit_wait: 10m\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Agent.LimitWait != 10*time.Minute {
		t.Errorf("agent.limit_wait = %v, want 10m", c.Agent.LimitWait)
	}
}

func TestNegativeLimitWaitIsRefused(t *testing.T) {
	_, err := Load(write(t, valid+"agent:\n  limit_wait: -1m\n"))
	if err == nil || !strings.Contains(err.Error(), "agent.limit_wait") {
		t.Fatalf("err = %v, want agent.limit_wait reported", err)
	}
}

func TestQuota(t *testing.T) {
	c, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if c.Quota.DailyUSD != nil || c.Quota.MonthlyUSD != nil {
		t.Errorf("quota without the section = %+v, want none", c.Quota)
	}
	c, err = Load(write(t, valid+"quota:\n  daily_usd: 5\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Quota.DailyUSD == nil || *c.Quota.DailyUSD != 5 || c.Quota.MonthlyUSD != nil {
		t.Errorf("quota = %+v, want daily 5 and no monthly", c.Quota)
	}
}

func TestNegativeQuotaIsRefused(t *testing.T) {
	_, err := Load(write(t, valid+"quota:\n  monthly_usd: -1\n"))
	if err == nil || !strings.Contains(err.Error(), "quota.monthly_usd") {
		t.Fatalf("err = %v, want quota.monthly_usd reported", err)
	}
	_, err = Load(write(t, valid+"quota:\n  daily_usd: -0.5\n"))
	if err == nil || !strings.Contains(err.Error(), "quota.daily_usd") {
		t.Fatalf("err = %v, want quota.daily_usd reported", err)
	}
}

func TestQuotaWithSubscriptionWarns(t *testing.T) {
	c, err := Load(write(t, valid+"quota:\n  daily_usd: 5\n"))
	if err != nil {
		t.Fatal(err)
	}
	w := c.Warnings()
	if len(w) != 1 || !strings.Contains(w[0], "quota") || !strings.Contains(w[0], "subscription") {
		t.Fatalf("warnings = %q, want one on the quota unused with a subscription", w)
	}
	c, err = Load(write(t, valid+"claude:\n  auth: api_key\nquota:\n  daily_usd: 5\n"))
	if err != nil {
		t.Fatal(err)
	}
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("warnings with api_key = %q, want none", w)
	}
	c, err = Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if w := c.Warnings(); len(w) != 0 {
		t.Errorf("warnings without a quota = %q, want none", w)
	}
}
