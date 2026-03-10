package payments

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type StripeProvider struct {
	secretKey     string
	webhookSecret string
	baseURL       string
	http          *http.Client
}

func NewStripeProvider(secretKey, webhookSecret, baseURL string) *StripeProvider {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "https://api.stripe.com"
	}

	return &StripeProvider{
		secretKey:     strings.TrimSpace(secretKey),
		webhookSecret: strings.TrimSpace(webhookSecret),
		baseURL:       base,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (p *StripeProvider) IsConfigured() bool {
	return p != nil && strings.TrimSpace(p.secretKey) != ""
}

func (p *StripeProvider) CreateCustomerCheckoutSession(ctx context.Context, input CustomerCheckoutSessionParams) (CustomerCheckoutSession, error) {
	if !p.IsConfigured() {
		return CustomerCheckoutSession{}, fmt.Errorf("stripe secret key is not configured")
	}

	values := url.Values{}
	values.Set("mode", "payment")
	values.Set("success_url", strings.TrimSpace(input.SuccessURL))
	values.Set("cancel_url", strings.TrimSpace(input.CancelURL))
	values.Add("payment_method_types[]", "card")
	values.Set("line_items[0][price_data][currency]", strings.ToLower(strings.TrimSpace(input.Currency)))
	values.Set("line_items[0][price_data][unit_amount]", strconv.FormatInt(input.AmountCents, 10))
	values.Set("line_items[0][price_data][product_data][name]", strings.TrimSpace(input.Description))
	values.Set("line_items[0][quantity]", "1")
	values.Set("metadata[tenant_id]", strings.TrimSpace(input.TenantID))
	values.Set("metadata[customer_id]", strings.TrimSpace(input.CustomerID))
	values.Set("metadata[source]", "barbersloyalties")
	values.Set("client_reference_id", strings.TrimSpace(input.CustomerID))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/checkout/sessions", bytes.NewBufferString(values.Encode()))
	if err != nil {
		return CustomerCheckoutSession{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.secretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.http.Do(req)
	if err != nil {
		return CustomerCheckoutSession{}, fmt.Errorf("create checkout session request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CustomerCheckoutSession{}, fmt.Errorf("read checkout session response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return CustomerCheckoutSession{}, fmt.Errorf("stripe checkout session error: status=%d body=%s", resp.StatusCode, string(body))
	}

	var payload struct {
		ID            string `json:"id"`
		URL           string `json:"url"`
		PaymentIntent string `json:"payment_intent"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return CustomerCheckoutSession{}, fmt.Errorf("parse checkout session response: %w", err)
	}
	if payload.ID == "" || payload.URL == "" {
		return CustomerCheckoutSession{}, fmt.Errorf("stripe checkout session missing id or url")
	}

	return CustomerCheckoutSession{
		ID:            payload.ID,
		URL:           payload.URL,
		PaymentIntent: payload.PaymentIntent,
	}, nil
}

func (p *StripeProvider) RefundPayment(ctx context.Context, input PaymentRefundParams) (PaymentRefundResult, error) {
	// TODO(stripe-real): replace with full Stripe Refund API flow once constrained refund endpoint is implemented.
	_ = ctx
	_ = input
	return PaymentRefundResult{}, fmt.Errorf("stripe refund is not implemented yet")
}

func (p *StripeProvider) ParsePaymentWebhook(payload []byte) (PaymentWebhookEvent, error) {
	return parsePaymentWebhookPayload(payload)
}

func (p *StripeProvider) VerifyPaymentWebhookSignature(payload []byte, signatureHeader string) error {
	return VerifyStripeSignature(payload, signatureHeader, p.webhookSecret, time.Now().UTC(), 5*time.Minute)
}

func VerifyStripeSignature(payload []byte, header, secret string, now time.Time, tolerance time.Duration) error {
	secret = strings.TrimSpace(secret)
	header = strings.TrimSpace(header)
	if secret == "" {
		return fmt.Errorf("stripe webhook secret is not configured")
	}
	if header == "" {
		return fmt.Errorf("missing stripe-signature header")
	}
	if tolerance <= 0 {
		tolerance = 5 * time.Minute
	}

	fields := map[string][]string{}
	parts := strings.Split(header, ",")
	for _, part := range parts {
		piece := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(piece) != 2 {
			continue
		}
		fields[piece[0]] = append(fields[piece[0]], piece[1])
	}

	timestamps := fields["t"]
	sigs := fields["v1"]
	if len(timestamps) == 0 || len(sigs) == 0 {
		return fmt.Errorf("invalid stripe-signature format")
	}

	ts, err := strconv.ParseInt(timestamps[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid stripe signature timestamp")
	}

	timestamp := time.Unix(ts, 0)
	if now.Sub(timestamp) > tolerance || timestamp.Sub(now) > tolerance {
		return fmt.Errorf("stripe signature is outside tolerance")
	}

	signedPayload := fmt.Sprintf("%d.%s", ts, string(payload))
	expected := computeHMACSHA256Hex(secret, signedPayload)
	for _, sig := range sigs {
		if hmac.Equal([]byte(strings.ToLower(sig)), []byte(strings.ToLower(expected))) {
			return nil
		}
	}

	return fmt.Errorf("stripe signature mismatch")
}

func BuildStripeSignatureHeader(payload []byte, secret string, ts int64) string {
	sig := computeHMACSHA256Hex(secret, fmt.Sprintf("%d.%s", ts, string(payload)))
	pairs := []string{fmt.Sprintf("t=%d", ts), fmt.Sprintf("v1=%s", sig)}
	sort.Strings(pairs)
	return strings.Join(pairs, ",")
}

func computeHMACSHA256Hex(secret, message string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}
