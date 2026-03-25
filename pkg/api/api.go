// Package api provides WeChat API client implementation.
package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	DefaultBaseURL           = "https://ilinkai.weixin.qq.com"
	DefaultCDNBaseURL        = "https://novac2c.cdn.weixin.qq.com/c2c"
	DefaultLongPollTimeout   = 35 * time.Second
	DefaultAPITimeout        = 15 * time.Second
	DefaultConfigTimeout     = 10 * time.Second
	SessionExpiredErrCode    = -14
)

// Client is the WeChat API client.
type Client struct {
	BaseURL     string
	Token       string
	HTTPClient  *http.Client
	RouteTag    string
	Timeout     time.Duration
	LongPollTimeout time.Duration
	version     string
}

// ClientOption is a function that configures the Client.
type ClientOption func(*Client)

// WithToken sets the authentication token.
func WithToken(token string) ClientOption {
	return func(c *Client) {
		c.Token = token
	}
}

// WithBaseURL sets the base URL.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.BaseURL = baseURL
	}
}

// WithRouteTag sets the route tag for SKRouteTag header.
func WithRouteTag(routeTag string) ClientOption {
	return func(c *Client) {
		c.RouteTag = routeTag
	}
}

// WithTimeout sets the default timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.Timeout = timeout
	}
}

// WithLongPollTimeout sets the long-poll timeout.
func WithLongPollTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.LongPollTimeout = timeout
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.HTTPClient = httpClient
	}
}

// NewClient creates a new WeChat API client.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		BaseURL:         DefaultBaseURL,
		HTTPClient:      &http.Client{Timeout: 60 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: false}}},
		Timeout:         DefaultAPITimeout,
		LongPollTimeout: DefaultLongPollTimeout,
		version:         "2.0.1",
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// buildBaseInfo creates the base info for API requests.
func (c *Client) buildBaseInfo() *BaseInfo {
	return &BaseInfo{ChannelVersion: c.version}
}

// randomWechatUin generates a random X-WECHAT-UIN header value.
func randomWechatUin() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	uint32Val := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", uint32Val))), nil
}

// buildHeaders creates HTTP headers for API requests.
func (c *Client) buildHeaders(body []byte) (map[string]string, error) {
	uin, err := randomWechatUin()
	if err != nil {
		return nil, fmt.Errorf("failed to generate X-WECHAT-UIN: %w", err)
	}

	headers := map[string]string{
		"Content-Type":      "application/json",
		"AuthorizationType": "ilink_bot_token",
		"Content-Length":    fmt.Sprintf("%d", len(body)),
		"X-WECHAT-UIN":      uin,
	}

	if c.Token != "" {
		headers["Authorization"] = "Bearer " + c.Token
	}

	if c.RouteTag != "" {
		headers["SKRouteTag"] = c.RouteTag
	}

	return headers, nil
}

// doRequest performs an HTTP POST request.
func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody interface{}, timeout time.Duration) ([]byte, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	baseURL := c.BaseURL
	if !bytes.HasSuffix([]byte(baseURL), []byte("/")) {
		baseURL += "/"
	}

	reqURL, err := url.Parse(baseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	headers, err := c.buildHeaders(body)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Create a client with the specific timeout
	client := c.HTTPClient
	if timeout > 0 {
		client = &http.Client{
			Timeout:   timeout,
			Transport: c.HTTPClient.Transport,
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error: %s: %s", resp.Status, string(respBody))
	}

	return respBody, nil
}

// GetUpdates performs a long-poll request to get new messages.
func (c *Client) GetUpdates(ctx context.Context, getUpdatesBuf string) (*GetUpdatesResp, error) {
	req := &GetUpdatesReq{
		GetUpdatesBuf: getUpdatesBuf,
		BaseInfo:      c.buildBaseInfo(),
	}

	respBody, err := c.doRequest(ctx, "ilink/bot/getupdates", req, c.LongPollTimeout)
	if err != nil {
		// Check for timeout (normal for long-poll)
		if ctx.Err() == context.DeadlineExceeded {
			return &GetUpdatesResp{Ret: 0, Msgs: []*WeixinMessage{}, GetUpdatesBuf: getUpdatesBuf}, nil
		}
		return nil, err
	}

	var resp GetUpdatesResp
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

// SendMessage sends a message to a user.
func (c *Client) SendMessage(ctx context.Context, msg *WeixinMessage) error {
	req := &SendMessageReq{Msg: msg}
	if req.Msg != nil {
		req.Msg.MessageType = int(MessageTypeBot)
		req.Msg.MessageState = int(MessageStateFinish)
	}

	_, err := c.doRequest(ctx, "ilink/bot/sendmessage", req, c.Timeout)
	return err
}

// GetUploadURL gets CDN upload pre-signed parameters.
func (c *Client) GetUploadURL(ctx context.Context, req *GetUploadUrlReq) (*GetUploadUrlResp, error) {
	req.BaseInfo = c.buildBaseInfo()
	respBody, err := c.doRequest(ctx, "ilink/bot/getuploadurl", req, c.Timeout)
	if err != nil {
		return nil, err
	}

	var resp GetUploadUrlResp
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

// GetConfig gets account configuration.
func (c *Client) GetConfig(ctx context.Context, ilinkUserID, contextToken string) (*GetConfigResp, error) {
	req := &GetConfigReq{
		ILinkUserID:  ilinkUserID,
		ContextToken: contextToken,
		BaseInfo:     c.buildBaseInfo(),
	}

	respBody, err := c.doRequest(ctx, "ilink/bot/getconfig", req, DefaultConfigTimeout)
	if err != nil {
		return nil, err
	}

	var resp GetConfigResp
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &resp, nil
}

// SendTyping sends a typing indicator.
func (c *Client) SendTyping(ctx context.Context, ilinkUserID, typingTicket string, status TypingStatus) error {
	req := &SendTypingReq{
		ILinkUserID:  ilinkUserID,
		TypingTicket: typingTicket,
		Status:       int(status),
	}

	_, err := c.doRequest(ctx, "ilink/bot/sendtyping", req, DefaultConfigTimeout)
	return err
}

// GetQRCode fetches a QR code for login.
func (c *Client) GetQRCode(ctx context.Context, botType string) (*QRCodeResponse, error) {
	if botType == "" {
		botType = "3" // default ILINK bot type
	}

	reqURL := fmt.Sprintf("%s/ilink/bot/get_bot_qrcode?bot_type=%s", c.BaseURL, url.QueryEscape(botType))

	httpReq, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	if c.RouteTag != "" {
		httpReq.Header.Set("SKRouteTag", c.RouteTag)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("QR code fetch failed: %s: %s", resp.Status, string(body))
	}

	var qrResp QRCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&qrResp); err != nil {
		return nil, err
	}

	return &qrResp, nil
}

// PollQRStatus polls the QR code status.
func (c *Client) PollQRStatus(ctx context.Context, qrcode string) (*StatusResponse, error) {
	reqURL := fmt.Sprintf("%s/ilink/bot/get_qrcode_status?qrcode=%s", c.BaseURL, url.QueryEscape(qrcode))

	httpReq, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("iLink-App-ClientVersion", "1")
	if c.RouteTag != "" {
		httpReq.Header.Set("SKRouteTag", c.RouteTag)
	}

	// Use long-poll timeout for this request
	client := &http.Client{
		Timeout:   35 * time.Second,
		Transport: c.HTTPClient.Transport,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("QR status poll failed: %s: %s", resp.Status, string(body))
	}

	var statusResp StatusResponse
	if err := json.Unmarshal(body, &statusResp); err != nil {
		return nil, err
	}

	return &statusResp, nil
}
