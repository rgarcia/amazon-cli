package amazon

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/kernel/kernel-go-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTransport struct {
	body        string
	url         string
	browserID   string
	createID    string
	createCalls int
	curlErrors  []error
}

func (f *fakeTransport) CreateBrowser(_ context.Context, _ Options) (string, error) {
	f.createCalls++
	if f.createID == "" {
		return "", fmt.Errorf("missing fake create ID")
	}
	f.browserID = f.createID
	return f.createID, nil
}

func (f *fakeTransport) Curl(_ context.Context, browserID string, req CurlRequest) (*CurlResponse, error) {
	f.browserID = browserID
	f.url = req.URL
	if len(f.curlErrors) > 0 {
		err := f.curlErrors[0]
		f.curlErrors = f.curlErrors[1:]
		return nil, err
	}
	return &CurlResponse{Status: 200, Body: f.body}, nil
}

func (f *fakeTransport) RenderHTML(_ context.Context, _ string, _ string, _ int) (string, error) {
	return f.body, nil
}

func (f *fakeTransport) DeleteBrowser(_ context.Context, _ string) error {
	return nil
}

func TestListOrdersBuildsAmazonOrderHistoryURL(t *testing.T) {
	ft := &fakeTransport{body: sampleOrdersHTML}
	client := NewClientWithTransport(Options{AmazonBaseURL: "https://www.amazon.com"}, ft, "brw_123")

	page, err := client.ListOrders(context.Background(), ListOrdersOptions{
		Page:       2,
		PageSize:   10,
		StartIndex: -1,
		TimeFilter: "year-2026",
	})
	require.NoError(t, err)
	assert.Contains(t, ft.url, "/gp/your-account/order-history?")
	assert.Contains(t, ft.url, "orderFilter=year-2026")
	assert.Contains(t, ft.url, "startIndex=10")
	assert.Contains(t, ft.url, "disableCsd=missing-library")
	assert.Equal(t, 1, len(page.Orders))
	assert.Equal(t, "111-2222222-3333333", page.Orders[0].ID)
}

func TestGetOrderBuildsOrderDetailURL(t *testing.T) {
	ft := &fakeTransport{body: sampleOrderDetailHTML}
	client := NewClientWithTransport(Options{AmazonBaseURL: "https://www.amazon.com"}, ft, "brw_123")

	order, err := client.GetOrder(context.Background(), "111-2222222-3333333")
	require.NoError(t, err)
	assert.Contains(t, ft.url, "/gp/your-account/order-details?")
	assert.Contains(t, ft.url, "orderID=111-2222222-3333333")
	assert.Contains(t, ft.url, "disableCsd=missing-library")
	assert.Equal(t, "Delivered", order.Status)
	assert.Equal(t, "USB-C Cable", order.Items[0].Title)
}

func TestBrowserNewParamsUsesProfileName(t *testing.T) {
	params := newBrowserNewParams(Options{
		KernelProfileName: "amazon",
		BrowserTimeout:    600,
	})
	b, err := json.Marshal(params)
	require.NoError(t, err)
	assert.JSONEq(t, `{"profile":{"name":"amazon"},"timeout_seconds":600}`, string(b))
}

func TestBrowserNewParamsPrefersProfileID(t *testing.T) {
	params := newBrowserNewParams(Options{
		KernelProfileID:   "profile_123",
		KernelProfileName: "amazon",
		BrowserTimeout:    600,
	})
	b, err := json.Marshal(params)
	require.NoError(t, err)
	assert.JSONEq(t, `{"profile":{"id":"profile_123"},"timeout_seconds":600}`, string(b))
	assert.NotContains(t, string(b), `"name"`)
}

func TestClientUsesCachedBrowserWithoutProbe(t *testing.T) {
	cachePath := t.TempDir() + "/browsers.json"
	opts := Options{
		AmazonBaseURL:     "https://www.amazon.com",
		KernelProfileName: "amazon",
		BrowserTimeout:    600,
		BrowserCachePath:  cachePath,
		BrowserCacheKey:   "personal",
	}
	require.NoError(t, writeCachedBrowserID(opts, "brw_cached"))

	ft := &fakeTransport{body: sampleOrdersHTML}
	client, err := newClient(context.Background(), normalizeOptions(opts), ft)
	require.NoError(t, err)

	page, err := client.ListOrders(context.Background(), ListOrdersOptions{TimeFilter: "year-2026"})
	require.NoError(t, err)
	assert.Equal(t, "brw_cached", ft.browserID)
	assert.Equal(t, 0, ft.createCalls)
	assert.Equal(t, 1, len(page.Orders))
}

func TestClientRecreatesCachedBrowserOnKernelNotFound(t *testing.T) {
	cachePath := t.TempDir() + "/browsers.json"
	opts := Options{
		AmazonBaseURL:     "https://www.amazon.com",
		KernelProfileName: "amazon",
		BrowserTimeout:    600,
		BrowserCachePath:  cachePath,
		BrowserCacheKey:   "personal",
	}

	ft := &fakeTransport{
		body:       sampleOrdersHTML,
		createID:   "brw_replacement",
		curlErrors: []error{&kernel.Error{StatusCode: 404}},
	}
	client := NewClientWithTransport(opts, ft, "brw_stale")
	client.refreshOnMissing = true

	page, err := client.ListOrders(context.Background(), ListOrdersOptions{TimeFilter: "year-2026"})
	require.NoError(t, err)
	assert.Equal(t, "brw_replacement", ft.browserID)
	assert.Equal(t, 1, len(page.Orders))

	cached, ok := readCachedBrowserID(opts)
	require.True(t, ok)
	assert.Equal(t, "brw_replacement", cached)
}
