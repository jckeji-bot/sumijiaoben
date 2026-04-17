package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	apiURL     = "https://www.binance.com/bapi/growth/v1/friendly/growth-paas/resource/summary/list"
	resourceID = 47651
	pageSize   = 100
)

type apiRequest struct {
	ResourceID int `json:"resourceId"`
	PageIndex  int `json:"pageIndex"`
	PageSize   int `json:"pageSize"`
}

type apiResponse struct {
	Code    string `json:"code"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		ResourceSummaryList struct {
			PageIndex int        `json:"pageIndex"`
			PageSize  int        `json:"pageSize"`
			Data      []apiEntry `json:"data"`
		} `json:"resourceSummaryList"`
	} `json:"data"`
}

type apiEntry struct {
	Sequence      int     `json:"sequence"`
	NickName      string  `json:"nickName"`
	UserID        string  `json:"userId"`
	TradingVolume float64 `json:"tradingVolume"`
	Grade         float64 `json:"grade"`
}

// Entry holds one leaderboard row.
type Entry struct {
	Rank     int
	UserID   string
	Username string
	Volume   string
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

var directHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			Resolver: &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					// Bypass local proxy DNS (Surge/Clash) by using public DNS.
					d := net.Dialer{Timeout: 5 * time.Second}
					return d.DialContext(ctx, "udp", "8.8.8.8:53")
				},
			},
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

func buildLeaderboardRequest(body []byte) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.binance.com/zh-CN/activity/trading-competition/spot-altcoin-festival-wave-XAUt")
	req.Header.Set("Origin", "https://www.binance.com")
	return req, nil
}

// scrapeLeaderboard fetches the top-100 leaderboard via direct HTTP POST.
func scrapeLeaderboard() ([]Entry, error) {
	body, err := json.Marshal(apiRequest{
		ResourceID: resourceID,
		PageIndex:  1,
		PageSize:   pageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	do := func(client *http.Client) (*http.Response, error) {
		req, err := buildLeaderboardRequest(body)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		return client.Do(req)
	}

	// Try direct connection first (bypasses broken proxy env).
	resp, err := do(directHTTPClient)
	if err != nil {
		firstErr := err
		// Fallback: try with env proxy settings.
		resp, err = do(httpClient)
		if err != nil {
			return nil, fmt.Errorf("direct failed (%v); env proxy also failed: %w", firstErr, err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if apiResp.Code != "000000" {
		return nil, fmt.Errorf("api error code=%s msg=%s", apiResp.Code, apiResp.Message)
	}

	raw := apiResp.Data.ResourceSummaryList.Data
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty leaderboard data")
	}

	entries := make([]Entry, 0, len(raw))
	for _, r := range raw {
		username := r.NickName
		if username == "" {
			username = r.UserID
		}
		vol := r.TradingVolume
		if vol == 0 {
			vol = r.Grade
		}
		entries = append(entries, Entry{
			Rank:     r.Sequence,
			UserID:   r.UserID,
			Username: username,
			Volume:   fmt.Sprintf("%.4f", vol),
		})
	}
	return entries, nil
}
