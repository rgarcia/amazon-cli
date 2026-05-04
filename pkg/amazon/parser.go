package amazon

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	spaceRe        = regexp.MustCompile(`\s+`)
	orderIDRe      = regexp.MustCompile(`\b\d{3}-\d{7}-\d{7}\b`)
	productASINRe  = regexp.MustCompile(`(?i)(?:/dp/|/gp/product/)([a-z0-9]{10})|\b([a-z0-9]{10})\b`)
	startIndexRe   = regexp.MustCompile(`[?&]startIndex=(\d+)`)
	moneyRe        = regexp.MustCompile(`\$[0-9][0-9,]*(?:\.[0-9]{2})?`)
	grandTotalRe   = regexp.MustCompile(`(?i)\bgrand total:\s*(\$[0-9][0-9,]*(?:\.[0-9]{2})?)`)
	orderTotalRe   = regexp.MustCompile(`(?i)\border total:\s*(\$[0-9][0-9,]*(?:\.[0-9]{2})?)`)
	statusPatterns = []string{"Delivered", "Arriving", "Shipped", "Cancelled", "Canceled", "Returned", "Refunded", "Running late", "Out for delivery", "Preparing for shipment"}
)

func ParseOrdersPage(html, pageURL string, opts ListOrdersOptions) (*OrdersPage, error) {
	if looksLikeSignIn(html) {
		return nil, fmt.Errorf("Amazon sign-in page returned; refresh the Kernel profile login before retrying")
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	normalizeListOptions(&opts)

	orders := make([]Order, 0)
	seen := make(map[string]bool)
	doc.Find("[data-order-id], .order-card, .js-order-card").Each(func(_ int, s *goquery.Selection) {
		order := parseOrderCard(s, pageURL)
		if order.ID == "" || seen[order.ID] {
			return
		}
		seen[order.ID] = true
		orders = append(orders, order)
	})
	if len(orders) == 0 {
		doc.Find("a[href*='order-details'], a[href*='orderID=']").Each(func(_ int, s *goquery.Selection) {
			href, _ := s.Attr("href")
			id := orderIDFromString(href + " " + selectionText(s))
			if id == "" || seen[id] {
				return
			}
			seen[id] = true
			orders = append(orders, Order{ID: id, DetailURL: absoluteURL(pageURL, href)})
		})
	}

	nextStart, nextURL := parseNextPage(doc, opts.StartIndex, pageURL)
	return &OrdersPage{
		Orders:         orders,
		Page:           opts.Page,
		PageSize:       opts.PageSize,
		StartIndex:     opts.StartIndex,
		NextStartIndex: nextStart,
		TimeFilter:     opts.TimeFilter,
		URL:            pageURL,
		NextURL:        nextURL,
	}, nil
}

func ParseOrderDetail(html, pageURL, orderID string) (*OrderDetail, error) {
	if looksLikeSignIn(html) {
		return nil, fmt.Errorf("Amazon sign-in page returned; refresh the Kernel profile login before retrying")
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	text := selectionText(doc.Selection)
	id := firstNonEmpty(orderID, orderIDFromString(text))
	detail := &OrderDetail{
		Order: Order{
			ID:          id,
			OrderPlaced: extractBetweenLabels(text, "ORDER PLACED", "TOTAL", "ORDER #"),
			Total:       extractOrderTotal(text),
			ShipTo:      extractShipTo(text),
			Status:      extractStatus(text),
		},
		URL: pageURL,
	}
	detail.Items = extractItems(doc.Selection, pageURL)
	detail.Payments = extractSectionLines(doc, "payment")
	detail.Addresses = extractSectionLines(doc, "address")
	return detail, nil
}

func ParseProductSearchPage(html, pageURL string, opts SearchProductsOptions) (*ProductSearchPage, error) {
	if looksLikeSignIn(html) {
		return nil, fmt.Errorf("Amazon sign-in page returned; refresh the Kernel profile login before retrying")
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	normalizeSearchProductsOptions(&opts)

	products := make([]ProductSearchResult, 0)
	seen := make(map[string]bool)
	doc.Find(`div[data-component-type="s-search-result"]`).Each(func(_ int, s *goquery.Selection) {
		product := parseProductSearchResult(s, pageURL)
		if product.ASIN == "" || product.Title == "" || seen[product.ASIN] {
			return
		}
		seen[product.ASIN] = true
		products = append(products, product)
	})

	return &ProductSearchPage{
		Query:    opts.Query,
		Page:     opts.Page,
		Products: products,
		URL:      pageURL,
	}, nil
}

func ParseProductDetail(html, pageURL, asin string) (*ProductDetail, error) {
	if looksLikeSignIn(html) {
		return nil, fmt.Errorf("Amazon sign-in page returned; refresh the Kernel profile login before retrying")
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	asin = firstNonEmpty(strings.ToUpper(strings.TrimSpace(asin)), productASINFromString(pageURL))
	detail := &ProductDetail{
		ASIN:         asin,
		Title:        firstSelectionText(doc.Selection, "#productTitle", "h1#title"),
		Price:        firstSelectionText(doc.Selection, "#corePriceDisplay_desktop_feature_div .a-price .a-offscreen", "#corePrice_feature_div .a-price .a-offscreen", ".priceToPay .a-offscreen", ".apexPriceToPay .a-offscreen", "#priceblock_ourprice", "#priceblock_dealprice"),
		Rating:       firstSelectionText(doc.Selection, "#acrPopover .a-icon-alt", "#averageCustomerReviews .a-icon-alt", "[data-hook='rating-out-of-text']"),
		Reviews:      cleanReviewText(firstSelectionText(doc.Selection, "#acrCustomerReviewText", "[data-hook='total-review-count']")),
		Availability: firstSelectionText(doc.Selection, "#availability"),
		Merchant:     firstSelectionText(doc.Selection, "#merchant-info", "#tabular-buybox .tabular-buybox-text"),
		URL:          pageURL,
	}
	doc.Find("#feature-bullets li span.a-list-item").Each(func(_ int, s *goquery.Selection) {
		text := selectionText(s)
		if text != "" && !strings.Contains(strings.ToLower(text), "make sure this fits") {
			detail.Bullets = append(detail.Bullets, text)
		}
	})
	detail.Bullets = dedupeStrings(detail.Bullets)
	return detail, nil
}

func parseOrderCard(s *goquery.Selection, pageURL string) Order {
	text := selectionText(s)
	id, _ := s.Attr("data-order-id")
	if id == "" {
		id = orderIDFromString(text)
	}
	var detailURL string
	s.Find("a[href*='order-details'], a[href*='orderID=']").EachWithBreak(func(_ int, a *goquery.Selection) bool {
		href, _ := a.Attr("href")
		if orderIDFromString(href) == id || detailURL == "" {
			detailURL = absoluteURL(pageURL, href)
			return false
		}
		return true
	})
	return Order{
		ID:          id,
		OrderPlaced: extractBetweenLabels(text, "ORDER PLACED", "TOTAL", "SHIP TO", "ORDER #"),
		Total:       extractOrderTotal(text),
		ShipTo:      extractShipTo(text),
		Status:      extractStatus(text),
		Items:       extractItems(s, pageURL),
		DetailURL:   detailURL,
	}
}

func parseProductSearchResult(s *goquery.Selection, pageURL string) ProductSearchResult {
	asin, _ := s.Attr("data-asin")
	if asin == "" {
		s.Find("a[href*='/dp/'], a[href*='/gp/product/']").EachWithBreak(func(_ int, a *goquery.Selection) bool {
			href, _ := a.Attr("href")
			asin = productASINFromString(href)
			return asin == ""
		})
	}
	asin = strings.ToUpper(strings.TrimSpace(asin))
	return ProductSearchResult{
		ASIN:      asin,
		Title:     productSearchTitle(s),
		Price:     firstSelectionText(s, ".a-price .a-offscreen"),
		Rating:    productSearchRating(s),
		Reviews:   productSearchReviews(s),
		Sponsored: isSponsoredProductResult(s),
		URL:       absoluteURL(pageURL, "/dp/"+asin),
	}
}

func productSearchTitle(s *goquery.Selection) string {
	title := firstSelectionText(s, "h2 span", "h2")
	if title != "" {
		return title
	}
	var aria string
	s.Find("h2[aria-label], a[aria-label]").EachWithBreak(func(_ int, el *goquery.Selection) bool {
		aria, _ = el.Attr("aria-label")
		return strings.TrimSpace(aria) == ""
	})
	return cleanText(aria)
}

func productSearchRating(s *goquery.Selection) string {
	var rating string
	s.Find(".a-icon-alt, [aria-label*='out of 5 stars']").EachWithBreak(func(_ int, el *goquery.Selection) bool {
		text := firstNonEmpty(selectionText(el), attrText(el, "aria-label"))
		if strings.Contains(strings.ToLower(text), "out of 5") {
			rating = cleanText(text)
			return false
		}
		return true
	})
	return rating
}

func productSearchReviews(s *goquery.Selection) string {
	var reviews string
	s.Find("a[aria-label], span[aria-label]").EachWithBreak(func(_ int, el *goquery.Selection) bool {
		text := attrText(el, "aria-label")
		if isReviewCountLabel(text) {
			reviews = cleanReviewText(text)
			return false
		}
		return true
	})
	if reviews != "" {
		return reviews
	}
	s.Find("a[href*='customerReviews'] span, a[href*='#customerReviews'] span").EachWithBreak(func(_ int, el *goquery.Selection) bool {
		reviews = cleanReviewText(selectionText(el))
		return reviews == ""
	})
	return reviews
}

func isReviewCountLabel(text string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "out of 5") || strings.Contains(lower, "rating details") {
		return false
	}
	return strings.Contains(lower, " ratings") ||
		strings.Contains(lower, " reviews") ||
		strings.HasSuffix(lower, " rating") ||
		strings.HasSuffix(lower, " review")
}

func isSponsoredProductResult(s *goquery.Selection) bool {
	html, _ := goquery.OuterHtml(s)
	lowerHTML := strings.ToLower(html)
	if strings.Contains(lowerHTML, "sponsored") ||
		strings.Contains(lowerHTML, "puis-sponsored") ||
		strings.Contains(lowerHTML, "ad-feedback") ||
		strings.Contains(lowerHTML, "data-ad-id") {
		return true
	}
	return strings.Contains(strings.ToLower(selectionText(s)), "sponsored")
}

func extractItems(s *goquery.Selection, pageURL string) []OrderItem {
	var items []OrderItem
	seen := make(map[string]bool)
	s.Find(".yohtmlc-product-title, a[href*='/dp/'], a[href*='/gp/product/']").Each(func(_ int, el *goquery.Selection) {
		title := selectionText(el)
		if title == "" || seen[title] || isActionText(title) {
			return
		}
		href, _ := el.Attr("href")
		if isFooterProductLink(href) {
			return
		}
		items = append(items, OrderItem{Title: title, URL: absoluteURL(pageURL, href)})
		seen[title] = true
	})
	return items
}

func extractBetweenLabels(text string, label string, nextLabels ...string) string {
	start := strings.Index(strings.ToUpper(text), label)
	if start < 0 {
		return ""
	}
	rest := strings.TrimSpace(text[start+len(label):])
	upperRest := strings.ToUpper(rest)
	end := len(rest)
	for _, next := range nextLabels {
		if idx := strings.Index(upperRest, next); idx >= 0 && idx < end {
			end = idx
		}
	}
	return cleanText(rest[:end])
}

func extractOrderTotal(text string) string {
	if m := grandTotalRe.FindStringSubmatch(text); len(m) == 2 {
		return m[1]
	}
	if m := orderTotalRe.FindStringSubmatch(text); len(m) == 2 {
		return m[1]
	}
	totalText := extractBetweenLabels(text, "TOTAL", "SHIP TO", "ORDER #", "PAYMENT METHOD", "ORDER SUMMARY", "ARRIVING", "DELIVERED", "YOUR RECENTLY VIEWED", "BACK TO TOP")
	if m := moneyRe.FindString(totalText); m != "" {
		return m
	}
	return totalText
}

func extractShipTo(text string) string {
	return extractBetweenLabels(text,
		"SHIP TO",
		"ORDER #",
		"PAYMENT METHOD",
		"ORDER SUMMARY",
		"ITEM(S) SUBTOTAL",
		"ARRIVING",
		"DELIVERED",
		"YOUR RECENTLY VIEWED",
		"BACK TO TOP",
	)
}

func extractStatus(text string) string {
	for _, status := range statusPatterns {
		if strings.Contains(strings.ToLower(text), strings.ToLower(status)) {
			return status
		}
	}
	return ""
}

func parseNextPage(doc *goquery.Document, currentStart int, pageURL string) (int, string) {
	candidates := make([]int, 0)
	urls := make(map[int]string)
	doc.Find("a[href*='startIndex=']").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		m := startIndexRe.FindStringSubmatch(href)
		if len(m) != 2 {
			return
		}
		idx, err := strconv.Atoi(m[1])
		if err != nil || idx <= currentStart {
			return
		}
		candidates = append(candidates, idx)
		urls[idx] = absoluteURL(pageURL, href)
	})
	if len(candidates) == 0 {
		return 0, ""
	}
	sort.Ints(candidates)
	next := candidates[0]
	return next, urls[next]
}

func extractSectionLines(doc *goquery.Document, contains string) []string {
	var out []string
	contains = strings.ToLower(contains)
	doc.Find("[class], [id]").Each(func(_ int, s *goquery.Selection) {
		class, _ := s.Attr("class")
		id, _ := s.Attr("id")
		if !strings.Contains(strings.ToLower(class+" "+id), contains) {
			return
		}
		text := selectionText(s)
		if contains == "address" && strings.Contains(strings.ToLower(text), "ending in") {
			return
		}
		if text != "" && len(text) < 300 {
			out = append(out, text)
		}
	})
	return dedupeStrings(out)
}

func orderIDFromString(s string) string {
	return orderIDRe.FindString(s)
}

func productASINFromString(s string) string {
	m := productASINRe.FindStringSubmatch(s)
	if len(m) == 0 {
		return ""
	}
	for _, v := range m[1:] {
		if v != "" {
			return strings.ToUpper(v)
		}
	}
	return strings.ToUpper(m[0])
}

func cleanText(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

func selectionText(s *goquery.Selection) string {
	clone := s.Clone()
	clone.Find("script, style, noscript, template, svg").Remove()
	return cleanText(clone.Text())
}

func firstSelectionText(s *goquery.Selection, selectors ...string) string {
	for _, selector := range selectors {
		var out string
		s.Find(selector).EachWithBreak(func(_ int, el *goquery.Selection) bool {
			out = selectionText(el)
			return out == ""
		})
		if out != "" {
			return out
		}
	}
	return ""
}

func attrText(s *goquery.Selection, name string) string {
	value, _ := s.Attr(name)
	return cleanText(value)
}

func cleanReviewText(s string) string {
	s = cleanText(strings.Trim(s, "()"))
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(s, "ratings"), "rating"))
}

func isActionText(s string) bool {
	lower := strings.ToLower(s)
	actions := []string{"buy it again", "view item", "write a product review", "return or replace", "track package", "invoice"}
	for _, action := range actions {
		if lower == action {
			return true
		}
	}
	return len(s) < 3
}

func isFooterProductLink(href string) bool {
	lower := strings.ToLower(href)
	return strings.Contains(lower, "ref_=footer") || strings.Contains(lower, "plattr=")
}

func looksLikeSignIn(html string) bool {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return false
	}
	doc.Find("script, style, noscript, template, svg").Remove()
	return doc.Find("form").FilterFunction(func(_ int, form *goquery.Selection) bool {
		action, _ := form.Attr("action")
		return form.Find("input[type='password'], input#ap_password, input[name='password']").Length() > 0 &&
			(strings.Contains(strings.ToLower(action), "/ap/signin") ||
				form.Find("input#ap_email, input[name='email'], input#ap_password").Length() > 0)
	}).Length() > 0
}

func absoluteURL(baseURL, href string) string {
	if href == "" {
		return ""
	}
	u, err := url.Parse(href)
	if err != nil {
		return href
	}
	if u.IsAbs() {
		return u.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}
	return base.ResolveReference(u).String()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(values))
	for _, v := range values {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
