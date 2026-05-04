package amazon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/kernel/kernel-go-sdk"
	"github.com/kernel/kernel-go-sdk/option"
)

type BrowserTransport interface {
	CreateBrowser(ctx context.Context, opts Options) (string, error)
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
	refreshOnMissing bool
}

func NewClient(ctx context.Context, opts Options) (*Client, error) {
	opts = normalizeOptions(opts)

	transport, err := NewKernelTransport(opts)
	if err != nil {
		return nil, err
	}
	return newClient(ctx, opts, transport)
}

func newClient(ctx context.Context, opts Options, transport BrowserTransport) (*Client, error) {
	client := &Client{
		opts:             opts,
		transport:        transport,
		browserID:        opts.BrowserID,
		refreshOnMissing: opts.BrowserID == "",
	}
	if client.browserID == "" {
		if id, ok := readCachedBrowserID(opts); ok {
			client.browserID = id
			if opts.Debug {
				fmt.Fprintf(os.Stderr, "reusing cached Kernel browser %s\n", id)
			}
		} else if err := client.createBrowser(ctx); err != nil {
			return nil, err
		}
	}
	return client, nil
}

func NewClientWithTransport(opts Options, transport BrowserTransport, browserID string) *Client {
	opts = normalizeOptions(opts)
	return &Client{opts: opts, transport: transport, browserID: browserID, refreshOnMissing: browserID == ""}
}

func normalizeOptions(opts Options) Options {
	if opts.AmazonBaseURL == "" {
		opts.AmazonBaseURL = "https://www.amazon.com"
	}
	if opts.BrowserTimeout <= 0 {
		opts.BrowserTimeout = 600
	}
	if opts.RequestTimeout <= 0 {
		opts.RequestTimeout = 30
	}
	return opts
}

func (c *Client) Close(ctx context.Context) error {
	return nil
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
		rendered, err := c.renderHTML(ctx, u)
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
		rendered, err := c.renderHTML(ctx, u)
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
	resp, err := c.curl(ctx, CurlRequest{
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

func (c *Client) curl(ctx context.Context, req CurlRequest) (*CurlResponse, error) {
	resp, err := c.transport.Curl(ctx, c.browserID, req)
	if isMissingBrowserError(err) && c.refreshOnMissing {
		if refreshErr := c.replaceMissingBrowser(ctx); refreshErr != nil {
			return nil, refreshErr
		}
		return c.transport.Curl(ctx, c.browserID, req)
	}
	return resp, err
}

func (c *Client) renderHTML(ctx context.Context, targetURL string) (string, error) {
	html, err := c.transport.RenderHTML(ctx, c.browserID, targetURL, c.opts.RequestTimeout)
	if isMissingBrowserError(err) && c.refreshOnMissing {
		if refreshErr := c.replaceMissingBrowser(ctx); refreshErr != nil {
			return "", refreshErr
		}
		return c.transport.RenderHTML(ctx, c.browserID, targetURL, c.opts.RequestTimeout)
	}
	return html, err
}

func (c *Client) replaceMissingBrowser(ctx context.Context) error {
	oldID := c.browserID
	if c.opts.Debug {
		fmt.Fprintf(os.Stderr, "Kernel browser %s not found; creating replacement\n", oldID)
	}
	if err := c.createBrowser(ctx); err != nil {
		return fmt.Errorf("Kernel browser %s not found and creating replacement failed: %w", oldID, err)
	}
	return nil
}

func (c *Client) createBrowser(ctx context.Context) error {
	id, err := c.transport.CreateBrowser(ctx, c.opts)
	if err != nil {
		return err
	}
	c.browserID = id
	if err := writeCachedBrowserID(c.opts, id); err != nil && c.opts.Debug {
		fmt.Fprintf(os.Stderr, "cache browser: %s\n", err)
	}
	if c.opts.Debug {
		fmt.Fprintf(os.Stderr, "created Kernel browser %s\n", id)
	}
	return nil
}

func isMissingBrowserError(err error) bool {
	if err == nil {
		return false
	}
	var apierr *kernel.Error
	return errors.As(err, &apierr) && apierr.StatusCode == http.StatusNotFound
}

func (c *Client) ordersURL(timeFilter string, startIndex int) string {
	base := strings.TrimRight(c.opts.AmazonBaseURL, "/")
	v := url.Values{}
	v.Set("orderFilter", timeFilter)
	v.Set("startIndex", fmt.Sprintf("%d", startIndex))
	v.Set("disableCsd", "missing-library")
	return base + "/gp/your-account/order-history?" + v.Encode()
}

func (c *Client) orderURL(orderID string) string {
	base := strings.TrimRight(c.opts.AmazonBaseURL, "/")
	v := url.Values{}
	v.Set("orderID", orderID)
	v.Set("disableCsd", "missing-library")
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
	res, err := t.client.Browsers.New(ctx, newBrowserNewParams(opts))
	if err != nil {
		return "", err
	}
	return res.SessionID, nil
}

func newBrowserNewParams(opts Options) kernel.BrowserNewParams {
	params := kernel.BrowserNewParams{
		TimeoutSeconds: kernel.Int(int64(opts.BrowserTimeout)),
	}
	params.Profile = newBrowserProfileParam(opts)
	return params
}

func newBrowserProfileParam(opts Options) kernel.BrowserProfileParam {
	if opts.KernelProfileID != "" {
		return kernel.BrowserProfileParam{ID: kernel.String(opts.KernelProfileID)}
	}
	if opts.KernelProfileName != "" {
		return kernel.BrowserProfileParam{Name: kernel.String(opts.KernelProfileName)}
	}
	return kernel.BrowserProfileParam{}
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
