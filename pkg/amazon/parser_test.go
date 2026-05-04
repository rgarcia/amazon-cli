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

func TestParseOrderDetailIgnoresScriptText(t *testing.T) {
	html := `
<html><body>
  <h1>Order Details</h1>
  <div>
    ORDER PLACED May 3, 2026
    TOTAL
    <script>
      const TOTAL_WIDTH_TAKEN_IN_PAGE = 1000;
      window.ue && ue.count && ue.count('CSMLibrarySize', 102118);
    </script>
    $19.99
    SHIP TO Raf
    ORDER # 113-0964028-0277852
  </div>
  <div>Arriving tomorrow</div>
  <a href="/dp/B000TEST">Replacement Charger</a>
</body></html>`

	order, err := ParseOrderDetail(html, "https://www.amazon.com/gp/your-account/order-details?orderID=113-0964028-0277852", "113-0964028-0277852")
	require.NoError(t, err)
	assert.Equal(t, "May 3, 2026", order.OrderPlaced)
	assert.Equal(t, "$19.99", order.Total)
	assert.NotContains(t, order.Total, "TOTAL_WIDTH_TAKEN_IN_PAGE")
	assert.Equal(t, "Raf", order.ShipTo)
	assert.Equal(t, "Arriving", order.Status)
}

func TestParseOrderDetailUsesBoundedSummaryFields(t *testing.T) {
	html := `
<html><body>
  <div>ORDER PLACED May 3, 2026 TOTAL: $8.99 Shipping & Handling: $0.00 Grand Total: $9.77 Arriving today</div>
  <div>SHIP TO Rafael Garcia 354 FAIR OAKS ST Payment method AMEX ending in 2000 Order Summary Item(s) Subtotal: $8.99</div>
  <a href="/dp/B0DM9R266X?ref_=ppx_hzod_title_dt_b_fed_asin_title_0_0">Superer 5V Fast Charger</a>
  <a href="/dp/product/B084KP3NG6?plattr=SCFOOT&ref_=footer_ACB">Amazon Secured Card</a>
  <div class="payment-info">Payment method AMEX ending in 2000</div>
  <div class="shipping-address">AMEX ending in 2000</div>
  <footer>Back to top Get to Know Us Careers</footer>
</body></html>`

	order, err := ParseOrderDetail(html, "https://www.amazon.com/gp/your-account/order-details?orderID=113-0964028-0277852", "113-0964028-0277852")
	require.NoError(t, err)
	assert.Equal(t, "$9.77", order.Total)
	assert.Equal(t, "Rafael Garcia 354 FAIR OAKS ST", order.ShipTo)
	require.Len(t, order.Items, 1)
	assert.Equal(t, "Superer 5V Fast Charger", order.Items[0].Title)
	assert.Empty(t, order.Addresses)
}
