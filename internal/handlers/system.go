package handlers

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

)

// InstallDatabase checks for database schema and installs it if missing
// @Summary Install Database Schema
// @Description Executes consolidated SQL migrations for ClickHouse and PostgreSQL
// @Tags System
// @Accept json
// @Produce json
// @Security ServerToken
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /system/install [post]
func (h *Handler) InstallDatabase(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	results := make(map[string]string)
	hasError := false

	// 1. PostgreSQL Installation
	pgSchemaPath := filepath.Join("migrations", "postgres", "001_initial_schema.sql")
	if err := h.executePostgresSQL(ctx, pgSchemaPath); err != nil {
		results["postgres"] = "failed: " + err.Error()
		hasError = true
	} else {
		results["postgres"] = "success"
	}

	// 2. ClickHouse Installation
	chSchemaPath := filepath.Join("migrations", "clickhouse", "001_initial_schema.sql")
	if err := h.executeClickHouseSQL(ctx, chSchemaPath); err != nil {
		results["clickhouse"] = "failed: " + err.Error()
		hasError = true
	} else {
		results["clickhouse"] = "success"
	}

	statusCode := http.StatusOK
	if hasError {
		statusCode = http.StatusInternalServerError
	}

	h.jsonResponse(w, statusCode, map[string]interface{}{
		"status":  "completed",
		"results": results,
		"error":   hasError,
	})
}

// executePostgresSQL reads a SQL file and executes it on Postgres
func (h *Handler) executePostgresSQL(ctx context.Context, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		h.logger.Errorw("failed to read schema file", "db", "PostgreSQL", "path", path, "error", err)
		return err
	}

	_, err = h.pg.Exec(ctx, string(content))
	if err != nil {
		h.logger.Errorw("failed to execute schema", "db", "PostgreSQL", "error", err)
		return err
	}

	h.logger.Infow("successfully installed schema", "db", "PostgreSQL")
	return nil
}

// executeClickHouseSQL reads a SQL file and executes it on ClickHouse
func (h *Handler) executeClickHouseSQL(ctx context.Context, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		h.logger.Errorw("failed to read schema file", "db", "ClickHouse", "path", path, "error", err)
		return err
	}

	// ClickHouse driver often prefers individual statements for complex DDL
	statements := strings.Split(string(content), ";")
	for _, stmt := range statements {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" {
			continue
		}

		if err := h.ch.Exec(ctx, trimmed); err != nil {
			h.logger.Warnw("statement execution warning", "db", "ClickHouse", "error", err, "statement", trimmed[:min(len(trimmed), 50)]+"...")
			return err
		}
	}

	h.logger.Infow("successfully installed schema", "db", "ClickHouse")
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
