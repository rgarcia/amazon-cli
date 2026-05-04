package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/kernel/kernel-go-sdk"
	"github.com/kernel/kernel-go-sdk/option"
)

type BrowserTransport interface {
	Curl(ctx context.Context, browserID string, req CurlRequest) (*CurlResponse, error)
	RenderHTML(ctx context.Context, browserID string, targetURL string, timeoutSeconds int) (string, error)
	DeleteBrowser(ctx context.Context, browserID string) error
}

type CurlRequest struct {
	Method        string
	URL           string
	Headers       map[string]string
	TimeoutMillis int64
}

type CurlResponse struct {
	Status  int64
	Body    string
	Headers map[string][]string
}

type Client struct {
	opts             Options
	transport        BrowserTransport
	browserID        string
	createdBrowserID string
}

func NewClient(ctx context.Context, opts Options) (*Client, error) {
	if opts.AmazonBaseURL == "" {
		opts.AmazonBaseURL = "https://www.amazon.com"
	}
	if opts.BrowserTimeout <= 0 {
		opts.BrowserTimeout = 300
	}
	if opts.RequestTimeout <= 0 {
		opts.RequestTimeout = 30
	}

	transport, err := NewKernelTransport(opts)
	if err != nil {
		return nil, err
	}
	client := &Client{opts: opts, transport: transport, browserID: opts.BrowserID}
	if client.browserID == "" {
		id, err := transport.CreateBrowser(ctx, opts)
		if err != nil {
			return nil, err
		}
		client.browserID = id
		client.createdBrowserID = id
		if opts.Debug {
			fmt.Fprintf(os.Stderr, "created Kernel browser %s\n", id)
		}
	}
	return client, nil
}

func NewClientWithTransport(opts Options, transport BrowserTransport, browserID string) *Client {
	if opts.AmazonBaseURL == "" {
		opts.AmazonBaseURL = "https://www.amazon.com"
	}
	if opts.RequestTimeout <= 0 {
		opts.RequestTimeout = 30
	}
	return &Client{opts: opts, transport: transport, browserID: browserID}
}

func (c *Client) Close(ctx context.Context) error {
	if c.createdBrowserID == "" || c.opts.KeepBrowser {
		return nil
	}
	return c.transport.DeleteBrowser(ctx, c.createdBrowserID)
}

func (c *Client) ListOrders(ctx context.Context, opts ListOrdersOptions) (*OrdersPage, error) {
	normalizeListOptions(&opts)
	u := c.ordersURL(opts.TimeFilter, opts.StartIndex)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	page, err := ParseOrdersPage(body, u, opts)
	if err != nil {
		return nil, err
	}
	if len(page.Orders) == 0 && needsRenderedDOM(body) {
		rendered, err := c.transport.RenderHTML(ctx, c.browserID, u, c.opts.RequestTimeout)
		if err != nil {
			return nil, err
		}
		page, err = ParseOrdersPage(rendered, u, opts)
		if err != nil {
			return nil, err
		}
	}
	return page, nil
}

func (c *Client) GetOrder(ctx context.Context, orderID string) (*OrderDetail, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, fmt.Errorf("order ID required")
	}
	u := c.orderURL(orderID)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	detail, err := ParseOrderDetail(body, u, orderID)
	if err != nil {
		return nil, err
	}
	if detail.ID == "" || len(detail.Items) == 0 && needsRenderedDOM(body) {
		rendered, err := c.transport.RenderHTML(ctx, c.browserID, u, c.opts.RequestTimeout)
		if err != nil {
			return nil, err
		}
		detail, err = ParseOrderDetail(rendered, u, orderID)
		if err != nil {
			return nil, err
		}
	}
	return detail, nil
}

func (c *Client) get(ctx context.Context, targetURL string) (string, error) {
	if c.opts.Debug {
		fmt.Fprintf(os.Stderr, "GET %s via browser %s\n", targetURL, c.browserID)
	}
	resp, err := c.transport.Curl(ctx, c.browserID, CurlRequest{
		Method: "GET",
		URL:    targetURL,
		Headers: map[string]string{
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Accept-Language": "en-US,en;q=0.9",
		},
		TimeoutMillis: int64(c.opts.RequestTimeout) * 1000,
	})
	if err != nil {
		return "", err
	}
	if resp.Status >= 400 {
		return "", &HTTPError{Method: "GET", URL: targetURL, Status: resp.Status, Body: resp.Body}
	}
	return resp.Body, nil
}

func (c *Client) ordersURL(timeFilter string, startIndex int) string {
	base := strings.TrimRight(c.opts.AmazonBaseURL, "/")
	v := url.Values{}
	v.Set("orderFilter", timeFilter)
	v.Set("startIndex", fmt.Sprintf("%d", startIndex))
	return base + "/gp/your-account/order-history?" + v.Encode()
}

func (c *Client) orderURL(orderID string) string {
	base := strings.TrimRight(c.opts.AmazonBaseURL, "/")
	v := url.Values{}
	v.Set("orderID", orderID)
	return base + "/gp/your-account/order-details?" + v.Encode()
}

func normalizeListOptions(opts *ListOrdersOptions) {
	if opts.PageSize <= 0 {
		opts.PageSize = 10
	}
	if opts.Page <= 0 {
		opts.Page = 1
	}
	if opts.StartIndex < 0 {
		opts.StartIndex = (opts.Page - 1) * opts.PageSize
	} else {
		opts.Page = opts.StartIndex/opts.PageSize + 1
	}
	if opts.TimeFilter == "" {
		opts.TimeFilter = "year-2026"
	}
}

func needsRenderedDOM(html string) bool {
	return strings.Contains(html, "csd-encrypted-sensitive")
}

type KernelTransport struct {
	client kernel.Client
}

func NewKernelTransport(opts Options) (*KernelTransport, error) {
	if opts.KernelAPIKey == "" {
		return nil, fmt.Errorf("Kernel API key is required")
	}
	clientOpts := []option.RequestOption{option.WithAPIKey(opts.KernelAPIKey)}
	if opts.KernelBaseURL != "" {
		clientOpts = append(clientOpts, option.WithBaseURL(strings.TrimRight(opts.KernelBaseURL, "/")+"/"))
	}
	return &KernelTransport{client: kernel.NewClient(clientOpts...)}, nil
}

func (t *KernelTransport) CreateBrowser(ctx context.Context, opts Options) (string, error) {
	var res browserCreateResponse
	if err := t.client.Execute(ctx, http.MethodPost, "browsers", newBrowserCreateParams(opts), &res); err != nil {
		return "", err
	}
	return res.SessionID, nil
}

type browserCreateParams struct {
	ProfileID      string `json:"profile_id,omitempty"`
	ProfileName    string `json:"profile_name,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type browserCreateResponse struct {
	SessionID string `json:"session_id"`
}

func newBrowserCreateParams(opts Options) browserCreateParams {
	params := browserCreateParams{
		TimeoutSeconds: opts.BrowserTimeout,
	}
	if opts.KernelProfileID != "" {
		params.ProfileID = opts.KernelProfileID
	} else {
		params.ProfileName = opts.KernelProfileName
	}
	return params
}

func (t *KernelTransport) Curl(ctx context.Context, browserID string, req CurlRequest) (*CurlResponse, error) {
	method := kernel.BrowserCurlParamsMethodGet
	if req.Method != "" {
		method = kernel.BrowserCurlParamsMethod(strings.ToUpper(req.Method))
	}
	res, err := t.client.Browsers.Curl(ctx, browserID, kernel.BrowserCurlParams{
		URL:              req.URL,
		Method:           method,
		Headers:          req.Headers,
		TimeoutMs:        kernel.Int(req.TimeoutMillis),
		ResponseEncoding: kernel.BrowserCurlParamsResponseEncodingUtf8,
	})
	if err != nil {
		return nil, err
	}
	return &CurlResponse{Status: res.Status, Body: res.Body, Headers: res.Headers}, nil
}

func (t *KernelTransport) RenderHTML(ctx context.Context, browserID string, targetURL string, timeoutSeconds int) (string, error) {
	u, _ := json.Marshal(targetURL)
	code := fmt.Sprintf(`
await page.goto(%s, { waitUntil: 'domcontentloaded' });
try {
  await page.waitForFunction(() => {
    const cards = Array.from(document.querySelectorAll('.order-card'));
    return cards.length === 0 || cards.some(card => !card.querySelector('.csd-encrypted-sensitive'));
  }, { timeout: %d });
} catch (err) {}
return await page.evaluate(() => document.documentElement.outerHTML);
`, string(u), timeoutSeconds*1000)
	res, err := t.client.Browsers.Playwright.Execute(ctx, browserID, kernel.BrowserPlaywrightExecuteParams{
		Code:       code,
		TimeoutSec: kernel.Int(int64(timeoutSeconds + 5)),
	})
	if err != nil {
		return "", err
	}
	if !res.Success {
		return "", fmt.Errorf("playwright render failed: %s", res.Error)
	}
	html, ok := res.Result.(string)
	if !ok {
		return "", fmt.Errorf("playwright render returned %T, want string", res.Result)
	}
	return html, nil
}

func (t *KernelTransport) DeleteBrowser(ctx context.Context, browserID string) error {
	return t.client.Browsers.DeleteByID(ctx, browserID)
}
