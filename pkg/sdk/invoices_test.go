package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListInvoices(t *testing.T) {
	paidAt := "2026-06-01T00:00:00Z"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/invoices", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		_ = json.NewEncoder(w).Encode([]Invoice{
			{
				Number: 1001, Kind: "subscription", Currency: "USD",
				Total: "25.00", VAT: "0.00", Paid: true, PaidAt: &paidAt,
				CreatedAt: "2026-06-01T00:00:00Z", DownloadURL: "https://example.com/1001.pdf",
			},
			{Number: 1002, Kind: "subscription", Currency: "USD", Total: "25.00", Paid: false},
		})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	invoices, err := c.ListInvoices(context.Background(), nil)
	require.NoError(t, err)
	assert.Len(t, invoices, 2)
	assert.Equal(t, 1001, invoices[0].Number)
	assert.True(t, invoices[0].Paid)
	require.NotNil(t, invoices[0].PaidAt)
	assert.Equal(t, paidAt, *invoices[0].PaidAt)
	assert.Nil(t, invoices[1].PaidAt)
}

func TestListInvoicesForbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "billing manager required"})
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.ListInvoices(context.Background(), nil)
	require.Error(t, err)
	apiErr, ok := err.(*APIError)
	require.True(t, ok)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
}

func TestDownloadInvoice(t *testing.T) {
	pdf := []byte("%PDF-1.4 fake invoice bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account/invoices/1001.pdf", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(pdf)
	}))
	defer server.Close()

	c := newTestClient(t, server)
	data, err := c.DownloadInvoice(context.Background(), 1001)
	require.NoError(t, err)
	assert.Equal(t, pdf, data)
}
