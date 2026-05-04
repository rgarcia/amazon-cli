package amazon

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTransport struct {
	body string
	url  string
}

func (f *fakeTransport) Curl(_ context.Context, _ string, req CurlRequest) (*CurlResponse, error) {
	f.url = req.URL
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
	assert.Equal(t, "Delivered", order.Status)
	assert.Equal(t, "USB-C Cable", order.Items[0].Title)
}

func TestBrowserNewParamsUsesProfileName(t *testing.T) {
	params := newBrowserNewParams(Options{
		KernelProfileName: "amazon",
		BrowserTimeout:    300,
	})
	b, err := json.Marshal(params)
	require.NoError(t, err)
	assert.JSONEq(t, `{"profile":{"name":"amazon"},"timeout_seconds":300}`, string(b))
}

func TestBrowserNewParamsPrefersProfileID(t *testing.T) {
	params := newBrowserNewParams(Options{
		KernelProfileID:   "profile_123",
		KernelProfileName: "amazon",
		BrowserTimeout:    300,
	})
	b, err := json.Marshal(params)
	require.NoError(t, err)
	assert.JSONEq(t, `{"profile":{"id":"profile_123"},"timeout_seconds":300}`, string(b))
	assert.NotContains(t, string(b), `"name"`)
}
