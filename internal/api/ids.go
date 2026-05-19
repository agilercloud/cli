package api

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func parseOptionalUUID(label, value string) (*uuid.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil {
		return nil, fmt.Errorf("%s must be a valid UUID", label)
	}
	return &id, nil
}
