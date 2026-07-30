package api

import (
	"context"
	"net/http"

	"github.com/agilercloud/cli/internal/publicapi"
)

// GetSelfUser returns the caller's user record, including effective
// scopes granted by the current credentials.
func (c *Client) GetSelfUser(ctx context.Context) (*SelfUser, error) {
	resp, err := c.impl.GetSelfUserWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	return requireJSONResponse(resp.StatusCode(), resp.Body, resp.JSON200)
}

// GetBilling returns the caller's billing posture (card brand, address,
// budgets, force-update flag).
func (c *Client) GetBilling(ctx context.Context) (*Billing, error) {
	resp, err := c.impl.GetMeBillingWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	return requireJSONResponse(resp.StatusCode(), resp.Body, resp.JSON200)
}

// UpdateBilling patches billing settings (address, budget, etc.).
func (c *Client) UpdateBilling(ctx context.Context, in UpdateBillingInput) error {
	params := &publicapi.UpdateMeBillingParams{IdempotencyKey: idempotencyKey()}
	resp, err := c.impl.UpdateMeBillingWithResponse(ctx, params, in)
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}

// ListBillingTransactions returns transactions grouped by month, following
// Link rel="next" pagination so callers see the full history (the page
// titled "complete history" in the SPA is otherwise capped at 12 months).
func (c *Client) ListBillingTransactions(ctx context.Context) ([]BillingMonth, error) {
	return paginateAll(func(cursor *string) ([]BillingMonth, http.Header, error) {
		resp, err := c.impl.ListMeBillingTransactionsWithResponse(ctx, &publicapi.ListMeBillingTransactionsParams{Cursor: cursor})
		if err != nil {
			return nil, nil, err
		}
		return requireJSONPage(resp.StatusCode(), resp.Body, resp.JSON200, resp.HTTPResponse.Header)
	})
}

// GetBillingStatement streams the PDF for a billing period (YYYY-MM).
// The generated client buffers binary responses, so we use a raw GET.
// The caller owns closing the returned body.
func (c *Client) GetBillingStatement(ctx context.Context, period string) (*http.Response, error) {
	resp, err := c.rawGET(ctx, "/v1/users/me/billing/statements/"+period)
	if err != nil {
		return nil, err
	}
	if err := decodeErrorFromHTTPResponse(resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ListNotifications returns notifications for the caller, following Link
// rel="next" pagination so callers see the full list.
func (c *Client) ListNotifications(ctx context.Context) ([]Notification, error) {
	return paginateAll(func(cursor *string) ([]Notification, http.Header, error) {
		resp, err := c.impl.ListMeNotificationsWithResponse(ctx, &publicapi.ListMeNotificationsParams{Cursor: cursor})
		if err != nil {
			return nil, nil, err
		}
		return requireJSONPage(resp.StatusCode(), resp.Body, resp.JSON200, resp.HTTPResponse.Header)
	})
}

// DeleteNotification dismisses a notification by ID.
func (c *Client) DeleteNotification(ctx context.Context, notificationID string) error {
	resp, err := c.impl.DeleteMeNotificationWithResponse(
		ctx, notificationID, &publicapi.DeleteMeNotificationParams{},
	)
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}
