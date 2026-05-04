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

const sampleProductSearchHTML = `
<html><body>
  <div data-component-type="s-search-result" data-asin="B000ORG123">
    <h2><span>Organic Result</span></h2>
    <span class="a-price"><span class="a-offscreen">$12.34</span></span>
    <span class="a-icon-alt">4.7 out of 5 stars</span>
    <a aria-label="4.7 out of 5 stars, rating details"></a>
    <span aria-label="1,234 ratings"></span>
  </div>
  <div data-component-type="s-search-result" data-asin="B000SPN456" class="puis-sponsored-label-text">
    <span>Sponsored</span>
    <h2><span>Sponsored Result</span></h2>
    <span class="a-price"><span class="a-offscreen">$24.68</span></span>
    <span class="a-icon-alt">4.2 out of 5 stars</span>
    <a aria-label="42 ratings"></a>
  </div>
</body></html>`

const sampleProductDetailHTML = `
<html><body>
  <span id="productTitle">Logitech M510 Wireless Mouse</span>
  <div id="corePriceDisplay_desktop_feature_div"><span class="a-price"><span class="a-offscreen">$27.99</span></span></div>
  <div id="averageCustomerReviews"><span class="a-icon-alt">4.6 out of 5 stars</span></div>
  <span id="acrCustomerReviewText">(34,832 ratings)</span>
  <div id="availability">
    <script>window.bad = "not availability"; var authPortalLink = "/ap/signin";</script>
    <span>In Stock</span>
  </div>
  <div id="merchant-info">Sold by Amazon.com</div>
  <div id="feature-bullets">
    <ul>
      <li><span class="a-list-item">Contoured shape with soft rubber grips</span></li>
      <li><span class="a-list-item">Make sure this fits by entering your model number.</span></li>
    </ul>
  </div>
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

func TestParseOrdersPageIgnoresScriptSignInMarkers(t *testing.T) {
	html := `
<html><body>
  <script>
    window.template = '<input id="ap_password" name="password">';
  </script>
  <div class="order-card js-order-card" data-order-id="111-2222222-3333333">
    <div>ORDER PLACED March 14, 2026 TOTAL $42.17 SHIP TO Raf ORDER # 111-2222222-3333333</div>
    <div>Delivered March 16</div>
    <a href="/gp/your-account/order-details?orderID=111-2222222-3333333">View order details</a>
  </div>
</body></html>`

	page, err := ParseOrdersPage(html, "https://www.amazon.com/gp/your-account/order-history?orderFilter=year-2026", ListOrdersOptions{
		TimeFilter: "year-2026",
	})
	require.NoError(t, err)
	require.Len(t, page.Orders, 1)
	assert.Equal(t, "111-2222222-3333333", page.Orders[0].ID)
}

func TestParseOrdersPageRejectsActualSignInForm(t *testing.T) {
	html := `
<html><body>
  <form action="/ap/signin">
    <input id="ap_email" name="email">
    <input id="ap_password" name="password" type="password">
  </form>
</body></html>`

	_, err := ParseOrdersPage(html, "https://www.amazon.com/gp/your-account/order-history?orderFilter=year-2026", ListOrdersOptions{
		TimeFilter: "year-2026",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Amazon sign-in page returned")
}

func TestParseProductSearchPage(t *testing.T) {
	page, err := ParseProductSearchPage(sampleProductSearchHTML, "https://www.amazon.com/s?k=wireless+mouse", SearchProductsOptions{
		Query: "wireless mouse",
		Page:  1,
	})
	require.NoError(t, err)
	require.Len(t, page.Products, 2)

	assert.Equal(t, "wireless mouse", page.Query)
	assert.Equal(t, "B000ORG123", page.Products[0].ASIN)
	assert.Equal(t, "Organic Result", page.Products[0].Title)
	assert.Equal(t, "$12.34", page.Products[0].Price)
	assert.Equal(t, "4.7 out of 5 stars", page.Products[0].Rating)
	assert.Equal(t, "1,234", page.Products[0].Reviews)
	assert.False(t, page.Products[0].Sponsored)
	assert.Equal(t, "https://www.amazon.com/dp/B000ORG123", page.Products[0].URL)

	assert.Equal(t, "B000SPN456", page.Products[1].ASIN)
	assert.True(t, page.Products[1].Sponsored)
}

func TestParseProductDetail(t *testing.T) {
	product, err := ParseProductDetail(sampleProductDetailHTML, "https://www.amazon.com/dp/B087Z5WDJ2", "B087Z5WDJ2")
	require.NoError(t, err)

	assert.Equal(t, "B087Z5WDJ2", product.ASIN)
	assert.Equal(t, "Logitech M510 Wireless Mouse", product.Title)
	assert.Equal(t, "$27.99", product.Price)
	assert.Equal(t, "4.6 out of 5 stars", product.Rating)
	assert.Equal(t, "34,832", product.Reviews)
	assert.Equal(t, "In Stock", product.Availability)
	assert.NotContains(t, product.Availability, "window.bad")
	assert.Equal(t, "Sold by Amazon.com", product.Merchant)
	assert.Equal(t, []string{"Contoured shape with soft rubber grips"}, product.Bullets)
}
