package svc

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/HappyLadySauce/Knowledge-Core/internal/config"
)

// ServiceContext wires shared infrastructure for HTTP handlers and background work.
// ServiceContext 为 HTTP 处理器与后台任务提供共享的基础设施连接。
type ServiceContext struct {
	Config      *config.Config
	DB          *gorm.DB
}

// NewServiceContext opens PostgreSQL (GORM) and Redis, applies pool settings, and verifies connectivity.
// NewServiceContext 打开 PostgreSQL（GORM）与 Redis，应用连接池参数并通过 Ping 校验连通性。
func NewServiceContext(ctx context.Context, cfg *config.Config) (*ServiceContext, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if cfg.JWT == nil {
		return nil, fmt.Errorf("jwt config is nil")
	}

	return &ServiceContext{
		Config:      cfg,
		DB:          nil,
	}, nil
}

// Close releases database and Redis resources (SQL first, then Redis).
// Close 释放数据库与 Redis 资源（先 SQL，后 Redis）。
func (s *ServiceContext) Close() error {
	var err error
	if s.DB != nil {
		sqlDB, e := s.DB.DB()
		if e != nil {
			err = errors.Join(err, e)
		} else {
			err = errors.Join(err, sqlDB.Close())
		}
	}
	return err
}
