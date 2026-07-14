package sdk

import (
	"context"
	"fmt"
	"net/http"
)

// Invoice is a billing invoice on the account, as served by
// GET /account/invoices. Invoices are keyed by Number (an integer), not a
// string identifier. The endpoint is gated: it requires a billing-manager API
// key and returns 403 otherwise.
type Invoice struct {
	// Number is the invoice number — the key used to download the PDF.
	Number int `json:"number"`
	// Kind is the invoice kind (e.g. "subscription").
	Kind string `json:"kind"`
	// Currency is the ISO currency code.
	Currency string `json:"currency"`
	// Total is the invoice total (a decimal string).
	Total string `json:"total"`
	// VAT is the VAT component (a decimal string).
	VAT string `json:"vat"`
	// Paid reports whether the invoice has been paid.
	Paid bool `json:"paid"`
	// PaidAt is the payment timestamp, nil when unpaid.
	PaidAt *string `json:"paid_at"`
	// CreatedAt is the invoice creation timestamp.
	CreatedAt string `json:"created_at"`
	// DownloadURL is the direct PDF download URL.
	DownloadURL string `json:"download_url"`
}

// ListInvoices returns the account's invoices. Requires a billing-manager API
// key; a non-billing key yields a 403 *APIError.
func (c *Client) ListInvoices(ctx context.Context, opts *ListOptions) ([]Invoice, error) {
	var resp []Invoice
	if err := c.get(ctx, appendListParams("/account/invoices", opts), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// DownloadInvoice fetches the raw PDF bytes for a single invoice, keyed by its
// number. The show endpoint is PDF-only (there is no JSON single-invoice form).
// Requires a billing-manager API key.
func (c *Client) DownloadInvoice(ctx context.Context, number int) ([]byte, error) {
	return c.doRaw(ctx, http.MethodGet, fmt.Sprintf("/account/invoices/%d.pdf", number))
}
