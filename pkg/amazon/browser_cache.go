package amazon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type browserCacheFile struct {
	Browsers map[string]cachedBrowser `json:"browsers"`
}

type cachedBrowser struct {
	BrowserID         string `json:"browser_id"`
	KernelBaseURL     string `json:"kernel_base_url,omitempty"`
	KernelProfileID   string `json:"kernel_profile_id,omitempty"`
	KernelProfileName string `json:"kernel_profile_name,omitempty"`
	AmazonBaseURL     string `json:"amazon_base_url,omitempty"`
	TimeoutSeconds    int    `json:"timeout_seconds,omitempty"`
	UpdatedAt         string `json:"updated_at,omitempty"`
}

func readCachedBrowserID(opts Options) (string, bool) {
	if opts.BrowserCachePath == "" || opts.BrowserCacheKey == "" {
		return "", false
	}
	cache, err := readBrowserCache(opts.BrowserCachePath)
	if err != nil {
		return "", false
	}
	record, ok := cache.Browsers[opts.BrowserCacheKey]
	if !ok || !record.matches(opts) {
		return "", false
	}
	return record.BrowserID, record.BrowserID != ""
}

func writeCachedBrowserID(opts Options, browserID string) error {
	if opts.BrowserCachePath == "" || opts.BrowserCacheKey == "" || browserID == "" {
		return nil
	}
	cache, err := readBrowserCache(opts.BrowserCachePath)
	if err != nil {
		cache = browserCacheFile{Browsers: make(map[string]cachedBrowser)}
	}
	if cache.Browsers == nil {
		cache.Browsers = make(map[string]cachedBrowser)
	}
	cache.Browsers[opts.BrowserCacheKey] = cachedBrowser{
		BrowserID:         browserID,
		KernelBaseURL:     opts.KernelBaseURL,
		KernelProfileID:   opts.KernelProfileID,
		KernelProfileName: opts.KernelProfileName,
		AmazonBaseURL:     opts.AmazonBaseURL,
		TimeoutSeconds:    opts.BrowserTimeout,
		UpdatedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	return writeBrowserCache(opts.BrowserCachePath, cache)
}

func readBrowserCache(path string) (browserCacheFile, error) {
	var cache browserCacheFile
	b, err := os.ReadFile(path)
	if err != nil {
		return cache, err
	}
	if err := json.Unmarshal(b, &cache); err != nil {
		return cache, err
	}
	return cache, nil
}

func writeBrowserCache(path string, cache browserCacheFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0600)
}

func (b cachedBrowser) matches(opts Options) bool {
	return b.KernelBaseURL == opts.KernelBaseURL &&
		b.KernelProfileID == opts.KernelProfileID &&
		b.KernelProfileName == opts.KernelProfileName &&
		b.AmazonBaseURL == opts.AmazonBaseURL &&
		b.TimeoutSeconds == opts.BrowserTimeout
}
