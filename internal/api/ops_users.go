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
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, errEmptyBody
	}
	return resp.JSON200, nil
}

// UpdateSelfUser patches the caller's user record.
func (c *Client) UpdateSelfUser(ctx context.Context, in UpdateSelfUserInput) error {
	params := &publicapi.UpdateSelfUserParams{IdempotencyKey: idempotencyKey()}
	resp, err := c.impl.UpdateSelfUserWithResponse(ctx, params, in)
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}

// GetBilling returns the caller's billing posture (card brand, address,
// budgets, force-update flag).
func (c *Client) GetBilling(ctx context.Context) (*Billing, error) {
	resp, err := c.impl.GetMeBillingWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, errEmptyBody
	}
	return resp.JSON200, nil
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

// ListBillingTransactions returns recent transactions, grouped by month
// as the server returns them. The CLI is free to flatten for display.
func (c *Client) ListBillingTransactions(ctx context.Context) ([]BillingMonth, error) {
	resp, err := c.impl.ListMeBillingTransactionsWithResponse(ctx, &publicapi.ListMeBillingTransactionsParams{})
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}
	return *resp.JSON200, nil
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

// ListNotifications returns recent notifications for the caller.
func (c *Client) ListNotifications(ctx context.Context) ([]Notification, error) {
	resp, err := c.impl.ListMeNotificationsWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, nil
	}
	return *resp.JSON200, nil
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
