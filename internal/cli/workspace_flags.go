package cli

import (
	"fmt"
	"strings"

	"github.com/agilercloud/cli/internal/app"
	"github.com/google/uuid"
)

func configuredWorkspaceID(a *app.App) string {
	if a == nil || a.Config == nil {
		return ""
	}
	return strings.TrimSpace(a.Config.WorkspaceID)
}

func parseWorkspaceID(value string) (uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return uuid.Nil, nil
	}
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, fmt.Errorf("workspace must be a valid UUID")
	}
	return id, nil
}

func normalizeWorkspaceID(value string) (string, error) {
	id, err := parseWorkspaceID(value)
	if err != nil {
		return "", err
	}
	if id == uuid.Nil {
		return "", nil
	}
	return id.String(), nil
}
