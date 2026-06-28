package svc

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	_ "modernc.org/sqlite"

	"github.com/HappyLadySauce/Knowledge-Core/internal/config"
)

// ServiceContext wires shared infrastructure for HTTP handlers and background work.
// ServiceContext 为 HTTP 处理器与后台任务提供共享的基础设施连接。
type ServiceContext struct {
	Config *config.Config
	DB     *sql.DB
}

// NewServiceContext opens the local SQLite index database and verifies connectivity.
// NewServiceContext 打开本地 SQLite 索引数据库并校验连通性。
func NewServiceContext(ctx context.Context, cfg *config.Config) (*ServiceContext, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if cfg.SQLite == nil {
		return nil, fmt.Errorf("sqlite config is nil")
	}

	db, err := openSQLite(ctx, cfg)
	if err != nil {
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

func openSQLite(ctx context.Context, cfg *config.Config) (*sql.DB, error) {
	dbPath, err := filepath.Abs(cfg.SQLite.Path)
	if err != nil {
		return nil, fmt.Errorf("resolve sqlite path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("create sqlite directory: %w", err)
	}

	dsn := sqliteDSN(dbPath, int(cfg.SQLite.BusyTimeout.Milliseconds()))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable sqlite wal: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return db, nil
}

func sqliteDSN(path string, busyTimeoutMS int) string {
	values := url.Values{}
	values.Set("_pragma", "busy_timeout("+strconv.Itoa(busyTimeoutMS)+")")
	return "file:" + filepath.ToSlash(path) + "?" + values.Encode()
}
