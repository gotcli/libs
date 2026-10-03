// Package database opens a PostgreSQL pool from settings supplied by the
// calling service. It does not read configuration itself.
package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	sharedauth "github.com/gotcli/libs/auth"
	"github.com/gotcli/libs/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Config is built by each service from its own configuration. Host, Port,
// Name and Username are required. Tuning fields left at zero use the
// library's fallback values (see withFallbacks).
type Config struct {
	Host     string
	Port     int
	Name     string
	Username string
	Password string
	SSLMode  string // e.g. "disable", "require"; empty means "require"

	SlowQueryThreshold time.Duration
	MaxOpenConns       int
	MaxIdleConns       int
	ConnMaxLifetime    time.Duration
	ConnMaxIdleTime    time.Duration
	ConnectAttempts    int
	ConnectBackoff     time.Duration
}

// Validate reports missing connection settings and negative tuning values.
func (c Config) Validate() error {
	var problems []string
	if strings.TrimSpace(c.Host) == "" {
		problems = append(problems, "host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		problems = append(problems, "port must be between 1 and 65535")
	}
	if strings.TrimSpace(c.Name) == "" {
		problems = append(problems, "name is required")
	}
	if strings.TrimSpace(c.Username) == "" {
		problems = append(problems, "username is required")
	}
	if c.MaxOpenConns < 0 || c.MaxIdleConns < 0 || c.ConnectAttempts < 0 ||
		c.SlowQueryThreshold < 0 || c.ConnMaxLifetime < 0 || c.ConnMaxIdleTime < 0 || c.ConnectBackoff < 0 {
		problems = append(problems, "tuning values must not be negative")
	}
	if len(problems) > 0 {
		return errors.New("invalid database config: " + strings.Join(problems, "; "))
	}
	return nil
}

func (c Config) withFallbacks() Config {
	if c.SSLMode == "" {
		c.SSLMode = "require"
	}
	if c.SlowQueryThreshold == 0 {
		c.SlowQueryThreshold = 200 * time.Millisecond
	}
	if c.MaxOpenConns == 0 {
		c.MaxOpenConns = 20
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 5
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = 30 * time.Minute
	}
	if c.ConnMaxIdleTime == 0 {
		c.ConnMaxIdleTime = 5 * time.Minute
	}
	if c.ConnectAttempts == 0 {
		c.ConnectAttempts = 10
	}
	if c.ConnectBackoff == 0 {
		c.ConnectBackoff = 2 * time.Second
	}
	return c
}

// DSN builds a URL-form connection string so credentials containing spaces or
// quotes are escaped correctly.
func (c Config) DSN() string {
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "require"
	}
	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.Username, c.Password),
		Host:     net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		Path:     "/" + c.Name,
		RawQuery: url.Values{"sslmode": {sslMode}}.Encode(),
	}
	return dsn.String()
}

// Connect opens the pool, waits for PostgreSQL to accept connections and
// registers the workspace scope. It retries with a fixed backoff until ctx is
// cancelled or ConnectAttempts is exhausted.
//
// Errors are translated, so unique and foreign-key violations surface as
// gorm.ErrDuplicatedKey and gorm.ErrForeignKeyViolated.
func Connect(ctx context.Context, cfg Config) (*gorm.DB, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cfg = cfg.withFallbacks()
	conn, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		TranslateError:       true,
		DisableAutomaticPing: true,
		Logger:               logger.NewGORMLogger(cfg.SlowQueryThreshold),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		return nil, fmt.Errorf("access database pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	for attempt := 1; ; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = sqlDB.PingContext(pingCtx)
		cancel()
		if err == nil {
			break
		}
		if attempt >= cfg.ConnectAttempts {
			_ = sqlDB.Close()
			return nil, fmt.Errorf("database unreachable after %d attempts: %w", cfg.ConnectAttempts, err)
		}
		log.Printf("database not ready (attempt %d/%d): %v", attempt, cfg.ConnectAttempts, err)
		select {
		case <-ctx.Done():
			_ = sqlDB.Close()
			return nil, ctx.Err()
		case <-time.After(cfg.ConnectBackoff):
		}
	}

	if err := sharedauth.RegisterWorkspaceScope(conn); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("register workspace scope: %w", err)
	}
	return conn, nil
}

// Close closes the underlying pool; it is safe to call with a nil conn.
func Close(conn *gorm.DB) {
	if conn == nil {
		return
	}
	if sqlDB, err := conn.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
