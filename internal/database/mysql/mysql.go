package mysql

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/Allinost/go-backend-core/internal/config"
)

// Pool 封装 *sql.DB，提供健康检查
type Pool struct {
	DB *sql.DB
}

// NewPool 根据配置创建 MySQL 连接池
func NewPool(cfg config.MySQLConfig) (*Pool, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql 打开连接失败 [%s:%d/%s]: %w", cfg.Host, cfg.Port, cfg.DBName, err)
	}

	if cfg.MaxOpen > 0 {
		db.SetMaxOpenConns(cfg.MaxOpen)
	} else {
		db.SetMaxOpenConns(25)
	}
	if cfg.MaxIdle > 0 {
		db.SetMaxIdleConns(cfg.MaxIdle)
	} else {
		db.SetMaxIdleConns(5)
	}
	if cfg.ConnMaxLifetime != "" {
		d, err := time.ParseDuration(cfg.ConnMaxLifetime)
		if err == nil {
			db.SetConnMaxLifetime(d)
		}
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("mysql ping 失败 [%s:%d/%s]: %w", cfg.Host, cfg.Port, cfg.DBName, err)
	}

	return &Pool{DB: db}, nil
}

// Close 关闭连接池
func (p *Pool) Close() error {
	if p.DB != nil {
		return p.DB.Close()
	}
	return nil
}

// Health 健康检查
func (p *Pool) Health() error {
	if p.DB == nil {
		return fmt.Errorf("mysql 连接未初始化")
	}
	return p.DB.Ping()
}
