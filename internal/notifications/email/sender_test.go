package email

import (
	htmltemplate "html/template"
	"strings"
	"testing"
	texttemplate "text/template"
)

func TestTemplatesParseAndRender(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		isHTML bool
		data   any
	}{
		{
			name:   "payment_success.html",
			path:   "templates/payment_success.html",
			isHTML: true,
			data: map[string]string{
				"PaymentID":    "pay-123",
				"Amount":       "$10.00",
				"MerchantName": "Acme Corp",
			},
		},
		{
			name:   "payment_success.txt",
			path:   "templates/payment_success.txt",
			isHTML: false,
			data: map[string]string{
				"PaymentID":    "pay-123",
				"Amount":       "$10.00",
				"MerchantName": "Acme Corp",
			},
		},
		{
			name:   "refund_in_progress.html",
			path:   "templates/refund_in_progress.html",
			isHTML: true,
			data: map[string]string{
				"RefundID":     "ref-456",
				"Amount":       "$10.00",
				"MerchantName": "Acme Corp",
				"RefundReason": "Duplicate charge",
			},
		},
		{
			name:   "refund_in_progress.txt",
			path:   "templates/refund_in_progress.txt",
			isHTML: false,
			data: map[string]string{
				"RefundID":     "ref-456",
				"Amount":       "$10.00",
				"MerchantName": "Acme Corp",
				"RefundReason": "Duplicate charge",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf strings.Builder
			if tc.isHTML {
				tmpl, err := htmltemplate.New(tc.name).ParseFiles(tc.path)
				if err != nil {
					t.Fatalf("parse %s: %v", tc.path, err)
				}
				if err := tmpl.Execute(&buf, tc.data); err != nil {
					t.Fatalf("render %s: %v", tc.path, err)
				}
			} else {
				tmpl, err := texttemplate.New(tc.name).ParseFiles(tc.path)
				if err != nil {
					t.Fatalf("parse %s: %v", tc.path, err)
				}
				if err := tmpl.Execute(&buf, tc.data); err != nil {
					t.Fatalf("render %s: %v", tc.path, err)
				}
			}

			if buf.Len() == 0 {
				t.Fatalf("render %s produced empty output", tc.path)
			}
		})
	}
}
