package amazon

type Options struct {
	KernelAPIKey      string
	KernelBaseURL     string
	KernelProfileID   string
	KernelProfileName string
	AmazonBaseURL     string
	BrowserID         string
	BrowserTimeout    int
	BrowserCachePath  string
	BrowserCacheKey   string
	RequestTimeout    int
	Debug             bool
}

type ListOrdersOptions struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	StartIndex int    `json:"start_index"`
	TimeFilter string `json:"time_filter"`
}

type OrdersPage struct {
	Orders         []Order `json:"orders"`
	Page           int     `json:"page"`
	PageSize       int     `json:"page_size"`
	StartIndex     int     `json:"start_index"`
	NextStartIndex int     `json:"next_start_index,omitempty"`
	TimeFilter     string  `json:"time_filter"`
	URL            string  `json:"url"`
	NextURL        string  `json:"next_url,omitempty"`
}

type Order struct {
	ID          string      `json:"id"`
	OrderPlaced string      `json:"order_placed,omitempty"`
	Total       string      `json:"total,omitempty"`
	ShipTo      string      `json:"ship_to,omitempty"`
	Status      string      `json:"status,omitempty"`
	Items       []OrderItem `json:"items,omitempty"`
	DetailURL   string      `json:"detail_url,omitempty"`
}

type OrderItem struct {
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
}

type OrderDetail struct {
	Order
	Payments  []string `json:"payments,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
	URL       string   `json:"url"`
}

type HTTPError struct {
	Method string
	URL    string
	Status int64
	Body   string
}

func (e *HTTPError) Error() string {
	return e.Method + " " + e.URL + ": " + statusText(e.Status)
}

func statusText(status int64) string {
	if status == 0 {
		return "request failed"
	}
	return "HTTP " + itoa64(status)
}

func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
