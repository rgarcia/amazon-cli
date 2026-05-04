package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rgarcia/amazon-cli/pkg/amazon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShowAnyAutoRendersDetailView(t *testing.T) {
	var out bytes.Buffer
	order := amazon.OrderDetail{
		Order: amazon.Order{
			ID:          "111-2222222-3333333",
			OrderPlaced: "March 14, 2026",
			Total:       "$42.17",
			ShipTo:      "Raf",
			Status:      "Delivered",
			Items: []amazon.OrderItem{
				{Title: "USB-C Cable", URL: "https://www.amazon.com/example"},
			},
			DetailURL: "https://www.amazon.com/detail",
		},
		Payments:  []string{"Visa ending in 1234"},
		Addresses: []string{"Seattle, WA"},
		URL:       "https://www.amazon.com/detail",
	}

	err := ShowAny(&out, order, "auto", "")
	require.NoError(t, err)

	text := out.String()
	assert.Contains(t, text, "ID            111-2222222-3333333")
	assert.Contains(t, text, "Order Placed  March 14, 2026")
	assert.Contains(t, text, "Detail URL    https://www.amazon.com/detail")
	assert.Contains(t, text, "\nItems\n")
	assert.Contains(t, text, "Title        URL")
	assert.Contains(t, text, "USB-C Cable")
	assert.Contains(t, text, "\nPayments\n  - Visa ending in 1234\n")
	assert.False(t, strings.HasPrefix(strings.TrimSpace(text), "{"))
}

func TestShowAnyJSONRemainsOptIn(t *testing.T) {
	var out bytes.Buffer
	order := amazon.OrderDetail{
		Order: amazon.Order{ID: "111-2222222-3333333"},
		URL:   "https://www.amazon.com/detail",
	}

	err := ShowAny(&out, order, "json", "")
	require.NoError(t, err)

	text := strings.TrimSpace(out.String())
	assert.True(t, strings.HasPrefix(text, "{"))
	assert.Contains(t, text, `"id": "111-2222222-3333333"`)
}
