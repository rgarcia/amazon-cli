package amazon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleOrdersHTML = `
<html><body>
  <div class="order-card js-order-card" data-order-id="111-2222222-3333333">
    <div>ORDER PLACED March 14, 2026 TOTAL $42.17 SHIP TO Raf ORDER # 111-2222222-3333333</div>
    <div>Delivered March 16</div>
    <a class="yohtmlc-product-title" href="/gp/product/B000TEST">USB-C Cable</a>
    <a href="/gp/your-account/order-details?orderID=111-2222222-3333333">View order details</a>
  </div>
  <a href="/gp/your-account/order-history?orderFilter=year-2026&startIndex=10">Next</a>
</body></html>`

const sampleOrderDetailHTML = `
<html><body>
  <h1>Order Details</h1>
  <div>ORDER PLACED March 14, 2026 TOTAL $42.17 SHIP TO Raf ORDER # 111-2222222-3333333</div>
  <div>Delivered March 16</div>
  <a href="/gp/product/B000TEST">USB-C Cable</a>
  <div class="payment-info">Payment Visa ending in 1234</div>
  <div class="shipping-address">Address Seattle, WA</div>
</body></html>`

func TestParseOrdersPage(t *testing.T) {
	page, err := ParseOrdersPage(sampleOrdersHTML, "https://www.amazon.com/gp/your-account/order-history?orderFilter=year-2026&startIndex=0", ListOrdersOptions{
		Page:       1,
		PageSize:   10,
		StartIndex: 0,
		TimeFilter: "year-2026",
	})
	require.NoError(t, err)
	require.Len(t, page.Orders, 1)
	order := page.Orders[0]
	assert.Equal(t, "111-2222222-3333333", order.ID)
	assert.Equal(t, "March 14, 2026", order.OrderPlaced)
	assert.Equal(t, "$42.17", order.Total)
	assert.Equal(t, "Raf", order.ShipTo)
	assert.Equal(t, "Delivered", order.Status)
	assert.Equal(t, "USB-C Cable", order.Items[0].Title)
	assert.Equal(t, 10, page.NextStartIndex)
}

func TestParseOrderDetail(t *testing.T) {
	order, err := ParseOrderDetail(sampleOrderDetailHTML, "https://www.amazon.com/gp/your-account/order-details?orderID=111-2222222-3333333", "111-2222222-3333333")
	require.NoError(t, err)
	assert.Equal(t, "111-2222222-3333333", order.ID)
	assert.Equal(t, "March 14, 2026", order.OrderPlaced)
	assert.Equal(t, "$42.17", order.Total)
	assert.Equal(t, "Raf", order.ShipTo)
	assert.Equal(t, "Delivered", order.Status)
	assert.Equal(t, "USB-C Cable", order.Items[0].Title)
	assert.Contains(t, order.Payments[0], "Visa")
	assert.Contains(t, order.Addresses[0], "Seattle")
}
