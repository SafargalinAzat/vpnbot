package marzban

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type MarzbanClient struct {
	mu         sync.Mutex
	BaseURL    string
	Username   string
	Password   string
	Token      string
	TokenExp   time.Time
	HTTPClient *http.Client
}

type AuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type UserCreate struct {
	Username string `json:"username"`
	Proxies  struct {
		Vless struct{} `json:"vless"`
	} `json:"proxies"`
	Inbounds struct {
		Vless []string `json:"vless"`
	} `json:"inbounds"`
	Expire    int    `json:"expire"`
	DataLimit int64  `json:"data_limit"`
	Status    string `json:"status"`
}

type UserResponse struct {
	Username        string                 `json:"username"`
	Status          string                 `json:"status"`
	Expire          int                    `json:"expire"`
	DataLimit       int64                  `json:"data_limit"`
	UsedTraffic     int64                  `json:"used_traffic"`
	LifetimeTraffic int64                  `json:"lifetime_used_traffic"`
	SubscriptionURL string                 `json:"subscription_url"`
	CreatedAt       string                 `json:"created_at"`
	Links           []string               `json:"links"`
	Proxies         map[string]interface{} `json:"proxies"`
	Inbounds        map[string]interface{} `json:"inbounds"`
	Note            string                 `json:"note"`
	SubUpdatedAt    string                 `json:"sub_updated_at"`
	OnlineAt        string                 `json:"online_at"`
}

func NewMarzbanClient(baseURL, username, password string) *MarzbanClient {
	return &MarzbanClient{
		BaseURL:    baseURL,
		Username:   username,
		Password:   password,
		HTTPClient: &http.Client{Timeout: 40 * time.Second},
	}
}

func (c *MarzbanClient) authenticate() error {
	// Создаем form-data
	data := url.Values{}
	data.Set("username", c.Username)
	data.Set("password", c.Password)

	req, err := http.NewRequest("POST", c.BaseURL+"/api/admin/token",
		strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Важно: правильный Content-Type для form-data
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("auth failed with status %d: %s", resp.StatusCode, string(body))
	}

	var authResp AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	c.Token = authResp.AccessToken
	c.TokenExp = time.Now().Add(23 * time.Hour)
	return nil
}

func (c *MarzbanClient) ensureAuth() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Token == "" || time.Now().After(c.TokenExp) {
		return c.authenticate()
	}
	return nil
}

func (c *MarzbanClient) CreateUser(username string, expireDays int, dataLimitGB int64) (*UserResponse, error) {
	if err := c.ensureAuth(); err != nil {
		return nil, err
	}

	expire := int(time.Now().AddDate(0, 0, expireDays).Unix())

	user := UserCreate{
		Username: username,
		Proxies: struct {
			Vless struct{} `json:"vless"`
		}{Vless: struct{}{}},
		Inbounds: struct {
			Vless []string `json:"vless"`
		}{Vless: []string{"VLESS TCP REALITY"}},
		Expire:    expire,
		DataLimit: dataLimitGB * 1024 * 1024 * 1024,
		Status:    "active",
	}

	jsonData, err := json.Marshal(user)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user data: %w", err)
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/api/user", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа для отладки
	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("create user failed with status %d: %s",
			resp.StatusCode, string(bodyBytes))
	}

	var userResp UserResponse
	if err := json.Unmarshal(bodyBytes, &userResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &userResp, nil
}

func (c *MarzbanClient) GetUser(username string) (*UserResponse, error) {
	if err := c.ensureAuth(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", c.BaseURL+"/api/user/"+username, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // User not found
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get user failed with status %d: %s", resp.StatusCode, string(body))
	}

	var userResp UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &userResp, nil
}
