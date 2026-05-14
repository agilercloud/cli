package api

import (
	"context"
	"net/http"

	"github.com/agilercloud/cli/internal/publicapi"
)

// ListBackups returns a project's backup records.
func (c *Client) ListBackups(ctx context.Context, projectID string) ([]Backup, error) {
	resp, err := c.impl.ListProjectBackupsWithResponse(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, errEmptyBody
	}
	return *resp.JSON200, nil
}

// CreateBackup triggers a manual backup.
func (c *Client) CreateBackup(ctx context.Context, projectID string) (*Backup, error) {
	params := &publicapi.CreateProjectBackupParams{IdempotencyKey: idempotencyKey()}
	resp, err := c.impl.CreateProjectBackupWithResponse(ctx, projectID, params)
	if err != nil {
		return nil, err
	}
	if err := checkStatus(resp.StatusCode(), resp.Body); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, errEmptyBody
	}
	return resp.JSON201, nil
}

// DeleteBackup removes a backup record.
func (c *Client) DeleteBackup(ctx context.Context, projectID, backupID string) error {
	resp, err := c.impl.DeleteProjectBackupWithResponse(
		ctx, projectID, backupID, &publicapi.DeleteProjectBackupParams{},
	)
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}

// RestoreBackup restores a project from the named backup.
func (c *Client) RestoreBackup(ctx context.Context, projectID, backupID string) error {
	params := &publicapi.RestoreProjectBackupParams{IdempotencyKey: idempotencyKey()}
	resp, err := c.impl.RestoreProjectBackupWithResponse(ctx, projectID, backupID, params)
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}

// GetBackupPolicy returns the per-project backup schedule.
func (c *Client) GetBackupPolicy(ctx context.Context, projectID string) (*BackupPolicy, error) {
	resp, err := c.impl.GetProjectBackupPolicyWithResponse(ctx, projectID)
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

// SetBackupPolicy patches the per-project backup schedule.
func (c *Client) SetBackupPolicy(ctx context.Context, projectID string, in UpdateBackupPolicy) error {
	resp, err := c.impl.UpdateProjectBackupPolicyWithResponse(
		ctx, projectID, &publicapi.UpdateProjectBackupPolicyParams{}, in,
	)
	if err != nil {
		return err
	}
	return checkStatus(resp.StatusCode(), resp.Body)
}

// BackupArtifact selects which artifact of a backup to download.
type BackupArtifact int

const (
	BackupDatabase BackupArtifact = iota
	BackupStorage
)

// DownloadBackup streams a backup artifact. The generated client buffers
// binary responses, so we issue a raw GET. The caller owns closing the
// returned body.
func (c *Client) DownloadBackup(ctx context.Context, projectID, backupID string, kind BackupArtifact) (*http.Response, error) {
	sub := "database"
	if kind == BackupStorage {
		sub = "storage"
	}
	resp, err := c.rawGET(ctx, "/v1/projects/"+projectID+"/backups/"+backupID+"/"+sub)
	if err != nil {
		return nil, err
	}
	if err := decodeErrorFromHTTPResponse(resp); err != nil {
		return nil, err
	}
	return resp, nil
}
