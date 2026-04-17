package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	CORS     CORSConfig
}

type AppConfig struct {
	Name     string
	Env      string
	Port     string
	Timezone string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
	MaxConns int32
	MinConns int32
}

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
}

type JWTConfig struct {
	Secret             string
	RefreshSecret      string
	Issuer             string
	Audience           string
	AccessExpireMinute int
	RefreshExpireHour  int
	Algorithm          string
}

type CORSConfig struct {
	AllowedOrigins []string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	loader := &envLoader{}

	cfg := &Config{
		App: AppConfig{
			Name:     loader.must("APP_NAME"),
			Env:      loader.must("APP_ENV"),
			Port:     loader.must("APP_PORT"),
			Timezone: loader.must("APP_TIMEZONE"),
		},
		Database: DatabaseConfig{
			Host:     loader.must("DB_HOST"),
			Port:     loader.mustInt("DB_PORT"),
			User:     loader.must("DB_USER"),
			Password: loader.must("DB_PASSWORD"),
			Name:     loader.must("DB_NAME"),
			SSLMode:  loader.must("DB_SSLMODE"),
			MaxConns: int32(loader.mustInt("DB_MAX_CONNS")),
			MinConns: int32(loader.mustInt("DB_MIN_CONNS")),
		},
		Redis: RedisConfig{
			Addr:         loader.must("REDIS_ADDR"),
			Password:     loader.optional("REDIS_PASSWORD"),
			DB:           loader.mustInt("REDIS_DB"),
			PoolSize:     loader.mustInt("REDIS_POOL_SIZE"),
			MinIdleConns: loader.mustInt("REDIS_MIN_IDLE_CONNS"),
		},
		JWT: JWTConfig{
			Secret:             loader.must("JWT_SECRET"),
			RefreshSecret:      loader.must("JWT_REFRESH_SECRET"),
			Issuer:             loader.must("JWT_ISSUER"),
			Audience:           loader.must("JWT_AUDIENCE"),
			AccessExpireMinute: loader.mustInt("JWT_ACCESS_EXPIRE_MINUTE"),
			RefreshExpireHour:  loader.mustInt("JWT_REFRESH_EXPIRE_HOUR"),
			Algorithm:          loader.must("JWT_ALGORITHM"),
		},
		CORS: CORSConfig{
			AllowedOrigins: splitCSV(loader.must("CORS_ALLOWED_ORIGINS")),
		},
	}

	if err := loader.err(); err != nil {
		return nil, fmt.Errorf("config: missing or invalid env vars:\n%w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: validation failed: %w", err)
	}

	return cfg, nil
}

func (c *Config) validate() error {
	var errs []error

	if err := c.JWT.validate(c.IsProduction()); err != nil {
		errs = append(errs, fmt.Errorf("jwt: %w", err))
	}
	if err := c.Database.validate(c.IsProduction()); err != nil {
		errs = append(errs, fmt.Errorf("database: %w", err))
	}

	return errors.Join(errs...)
}

func (j JWTConfig) validate(isProd bool) error {
	if j.Secret == j.RefreshSecret {
		return errors.New("JWT_SECRET and JWT_REFRESH_SECRET must be different")
	}
	if j.AccessExpireMinute <= 0 {
		return errors.New("JWT_ACCESS_EXPIRE_MINUTE must be greater than 0")
	}
	if j.RefreshExpireHour <= 0 {
		return errors.New("JWT_REFRESH_EXPIRE_HOUR must be greater than 0")
	}
	if isProd {
		if len(j.Secret) < minSecretLength {
			return fmt.Errorf("JWT_SECRET must be at least %d characters in production", minSecretLength)
		}
		if len(j.RefreshSecret) < minSecretLength {
			return fmt.Errorf("JWT_REFRESH_SECRET must be at least %d characters in production", minSecretLength)
		}
	}
	return nil
}

func (d DatabaseConfig) validate(isProd bool) error {
	if d.MinConns > d.MaxConns {
		return errors.New("DB_MIN_CONNS cannot be greater than DB_MAX_CONNS")
	}
	if isProd && d.SSLMode == "disable" {
		return errors.New("DB_SSLMODE must not be 'disable' in production")
	}
	return nil
}

func (c *Config) DatabaseDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.Database.User, c.Database.Password,
		c.Database.Host, c.Database.Port,
		c.Database.Name, c.Database.SSLMode,
	)
}

func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.App.Env, "production")
}

func (j JWTConfig) AccessExpireDuration() time.Duration {
	return time.Duration(j.AccessExpireMinute) * time.Minute
}

func (j JWTConfig) RefreshExpireDuration() time.Duration {
	return time.Duration(j.RefreshExpireHour) * time.Hour
}

const (
	minSecretLength = 32
)

// --------- env loader ---------

type envLoader struct {
	errs []error
}

func (e *envLoader) must(key string) string {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		e.errs = append(e.errs, fmt.Errorf("%s is required", key))
		return ""
	}
	return val
}

func (e *envLoader) mustInt(key string) int {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		e.errs = append(e.errs, fmt.Errorf("%s is required", key))
		return 0
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		e.errs = append(e.errs, fmt.Errorf("%s must be a valid integer (got %q)", key, val))
		return 0
	}
	return parsed
}

func (e *envLoader) optional(key string) string {
	return os.Getenv(key)
}

func (e *envLoader) err() error {
	return errors.Join(e.errs...)
}

// --------- helpers ---------

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}