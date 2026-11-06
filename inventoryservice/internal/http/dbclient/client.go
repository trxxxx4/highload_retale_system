package dbclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrOutOfStock     = errors.New("out of stock")
	ErrInvalidState   = errors.New("invalid state")
	ErrAmountMismatch = errors.New("amount mismatch")
	ErrConflict       = errors.New("conflict")
)

type UnavailableError struct {
	Err     error
	NotSent bool
}

func (e *UnavailableError) Error() string {
	return "db-service unavailable: " + e.Err.Error()
}

func (e *UnavailableError) Unwrap() error {
	return e.Err
}

type Product struct {
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	PriceCents int    `json:"price_cents"`
	Available  int    `json:"available"`
}

type ProductsResponse struct {
	Items []Product `json:"items"`
}

type Stock struct {
	SKU       string `json:"sku"`
	OnHand    int    `json:"on_hand"`
	Reserved  int    `json:"reserved"`
	Available int    `json:"available"`
}

type StockCacheItem struct {
	SKU       string `json:"sku"`
	Available int    `json:"available"`
}

type StockPage struct {
	Items     []StockCacheItem `json:"items"`
	NextAfter string           `json:"next_after"`
}

type CreateReservationRequest struct {
	ID             uuid.UUID `json:"id"`
	UserID         uuid.UUID `json:"user_id"`
	SKU            string    `json:"sku"`
	Qty            int       `json:"qty"`
	IdempotencyKey string    `json:"idempotency_key"`
	TTLSeconds     int       `json:"ttl_seconds"`
}

type Reservation struct {
	ID              uuid.UUID `json:"id"`
	UserID          uuid.UUID `json:"user_id"`
	SKU             string    `json:"sku"`
	Qty             int       `json:"qty"`
	Status          string    `json:"status"`
	EffectiveStatus string    `json:"effective_status"`
	ExpiresAt       time.Time `json:"expires_at"`
	AmountCents     int       `json:"amount_cents"`
}

type ExpiredReservation struct {
	ID  uuid.UUID `json:"id"`
	SKU string    `json:"sku"`
	Qty int       `json:"qty"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type CancelReservationRequest struct {
	UserID uuid.UUID `json:"user_id"`
}

type CanceledReservation struct {
	SKU string `json:"sku"`
	QTY int    `json:"qty"`
}

func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
	}
}

func NewClient(
	baseURL string,
	httpClient *http.Client,
) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

func (c *Client) Products(
	ctx context.Context,
	limit, offset int,
) (ProductsResponse, error) {
	var result ProductsResponse

	path := fmt.Sprintf(
		"/internal/products?limit=%d&offset=%d",
		limit,
		offset,
	)

	err := c.do(
		ctx,
		http.MethodGet,
		path,
		nil,
		&result,
	)

	return result, err
}

func (c *Client) Stock(
	ctx context.Context,
	sku string,
) (Stock, error) {
	var result Stock

	path := "/internal/stock/" + url.PathEscape(sku)

	err := c.do(
		ctx,
		http.MethodGet,
		path,
		nil,
		&result,
	)

	return result, err
}

func (c *Client) CreateReservation(
	ctx context.Context,
	input CreateReservationRequest,
) (Reservation, error) {
	var result Reservation

	err := c.do(
		ctx,
		http.MethodPost,
		"/internal/reservations",
		input,
		&result,
	)

	return result, err
}

func (c *Client) GetReservation(
	ctx context.Context,
	id uuid.UUID,
) (Reservation, error) {
	var result Reservation

	path := "/internal/reservations/" + id.String()

	err := c.do(
		ctx,
		http.MethodGet,
		path,
		nil,
		&result,
	)

	return result, err
}

func (c *Client) Pay(
	ctx context.Context,
	id uuid.UUID,
	paymentID string,
	amountCents int,
) error {
	body := map[string]any{
		"payment_id":   paymentID,
		"amount_cents": amountCents,
	}

	path := "/internal/reservations/" + id.String() + "/pay"

	return c.do(
		ctx,
		http.MethodPost,
		path,
		body,
		nil,
	)
}

func (c *Client) Cancel(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) (*CanceledReservation, error) {
	path := "/internal/reservations/" + id.String() + "/cancel"

	input := CancelReservationRequest{
		UserID: userID,
	}

	result := CanceledReservation{}
	err := c.do(
		ctx,
		http.MethodPost,
		path,
		input,
		&result,
	)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (c *Client) Expire(
	ctx context.Context,
	limit int,
) ([]ExpiredReservation, error) {
	var result struct {
		Items []ExpiredReservation `json:"items"`
	}

	body := map[string]int{
		"limit": limit,
	}

	err := c.do(
		ctx,
		http.MethodPost,
		"/internal/reservations/expire",
		body,
		&result,
	)
	if err != nil {
		return nil, err
	}

	return result.Items, nil
}

func (c *Client) do(
	ctx context.Context,
	method, path string,
	input, output any,
) error {
	var body io.Reader

	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}

		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		c.baseURL+path,
		body,
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &UnavailableError{
			Err:     err,
			NotSent: isDialError(err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode >= 500 {
			_, _ = io.Copy(io.Discard, resp.Body)

			return &UnavailableError{
				Err: fmt.Errorf(
					"unexpected status %d",
					resp.StatusCode,
				),
			}
		}

		return decodeAPIError(resp)
	}

	if output == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(output); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func isDialError(err error) bool {
	var opErr *net.OpError

	return errors.As(err, &opErr) && opErr.Op == "dial"
}

type apiErrorBody struct {
	Error json.RawMessage `json:"error"`
}

type apiErrorDetails struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func decodeAPIError(resp *http.Response) error {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf(
			"db-service returned status %d",
			resp.StatusCode,
		)
	}

	var body apiErrorBody
	if err := json.Unmarshal(data, &body); err != nil {
		return fmt.Errorf(
			"db-service returned status %d",
			resp.StatusCode,
		)
	}

	var code string

	if err := json.Unmarshal(body.Error, &code); err != nil {
		var details apiErrorDetails

		if err := json.Unmarshal(body.Error, &details); err == nil {
			code = details.Code
		}
	}

	switch code {
	case "not_found":
		return ErrNotFound
	case "out_of_stock":
		return ErrOutOfStock
	case "invalid_state":
		return ErrInvalidState
	case "amount_mismatch":
		return ErrAmountMismatch
	case "idempotency_conflict", "payment_conflict":
		return ErrConflict
	default:
		return fmt.Errorf(
			"db-service returned status %d, error %q",
			resp.StatusCode,
			code,
		)
	}
}

func (c *Client) Ready(ctx context.Context) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		c.baseURL+"/readyz",
		nil,
	)
	if err != nil {
		return fmt.Errorf("create readyz request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("db-service readyz request: %w", err)
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"db-service is not ready: status %d",
			resp.StatusCode,
		)
	}

	return nil
}

func (c *Client) Stocks(
	ctx context.Context,
	after string,
	limit int,
) (StockPage, error) {
	var result StockPage

	query := url.Values{}
	query.Set("after", after)
	query.Set("limit", strconv.Itoa(limit))

	path := "/internal/stock?" + query.Encode()

	err := c.do(
		ctx,
		http.MethodGet,
		path,
		nil,
		&result,
	)

	return result, err
}

// RequestJSON отправляет запрос в db-service и возвращает
// исходные HTTP-статус, заголовки и тело ответа.
// HTTP-ошибки db-service не преобразуются в другие статусы.
func (c *Client) RequestJSON(
	ctx context.Context,
	method string,
	path string,
	input any,
) (int, http.Header, []byte, error) {
	var body io.Reader

	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return 0, nil, nil, fmt.Errorf(
				"marshal request: %w",
				err,
			)
		}

		body = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		method,
		c.baseURL+path,
		body,
	)
	if err != nil {
		return 0, nil, nil, fmt.Errorf(
			"create request: %w",
			err,
		)
	}

	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, nil, &UnavailableError{
			Err:     err,
			NotSent: isDialError(err),
		}
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, fmt.Errorf(
			"read db-service response: %w",
			err,
		)
	}

	return resp.StatusCode, resp.Header.Clone(), responseBody, nil
}
