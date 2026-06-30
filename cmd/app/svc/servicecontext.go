package svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/HappyLadySauce/Knowledge-Core/internal/config"
)

// ServiceContext wires shared infrastructure for HTTP handlers and background work.
// ServiceContext 为 HTTP 处理器与后台任务提供共享的基础设施连接。
type ServiceContext struct {
	Config *config.Config
	DB     *sql.DB
}

// NewServiceContext opens PostgreSQL and verifies the required schema.
// NewServiceContext 打开 PostgreSQL 并校验必要 schema。
func NewServiceContext(ctx context.Context, cfg *config.Config) (*ServiceContext, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if cfg.Database == nil {
		return nil, fmt.Errorf("database config is nil")
	}
	if cfg.JWT == nil {
		return nil, fmt.Errorf("jwt config is nil")
	}
	if cfg.WebSocket == nil {
		return nil, fmt.Errorf("websocket config is nil")
	}

	db, err := openPostgres(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := verifySchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &ServiceContext{
		Config: cfg,
		DB:     db,
	}, nil
}

// Close releases database resources.
// Close 释放数据库资源。
func (s *ServiceContext) Close() error {
	var err error
	if s.DB != nil {
		err = errors.Join(err, s.DB.Close())
	}
	return err
}

func openPostgres(ctx context.Context, cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}

func verifySchema(ctx context.Context, db *sql.DB) error {
	requiredTables := []string{
		"users", "refresh_tokens", "login_attempts", "documents", "document_blocks",
		"document_ops", "document_revisions", "categories", "tags", "document_tags",
	}
	for _, table := range requiredTables {
		var exists bool
		err := db.QueryRowContext(ctx, `SELECT to_regclass('public.`+table+`') IS NOT NULL`).Scan(&exists)
		if err != nil {
			return fmt.Errorf("verify postgres schema: %w", err)
		}
		if !exists {
			return fmt.Errorf("postgres schema is not migrated: missing table %s; run make migrate", table)
		}
	}
	return nil
}
