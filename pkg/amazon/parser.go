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

func cleanText(s string) string {
	return strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
}

func selectionText(s *goquery.Selection) string {
	clone := s.Clone()
	clone.Find("script, style, noscript, template, svg").Remove()
	return cleanText(clone.Text())
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
	lower := strings.ToLower(html)
	return strings.Contains(lower, "ap_password") ||
		strings.Contains(lower, `name="password"`) ||
		strings.Contains(lower, "authportal") ||
		(strings.Contains(lower, "sign in") && strings.Contains(lower, "enter your password"))
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
