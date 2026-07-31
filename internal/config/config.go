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
	Payment  PaymentConfig   // ⬅ baru
	Mail     MailConfig      // ⬅ baru
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

type PaymentConfig struct {
	// Provider: "midtrans" atau "fake"
	Provider     string
	ServerKey    string
	IsProduction bool

	// FakeSecret hanya dipakai FakeGateway saat pengembangan.
	FakeSecret string
}

type MailConfig struct {
	// Driver: "smtp" atau "log"
	Driver    string
	Host      string
	Port      int
	Username  string
	Password  string
	FromName  string
	FromEmail string
	ShopName  string
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
				Payment: PaymentConfig{
			Provider:     loader.must("PAYMENT_PROVIDER"),
			ServerKey:    loader.optional("PAYMENT_SERVER_KEY"),
			IsProduction: strings.EqualFold(loader.must("APP_ENV"), "production"),
			FakeSecret:   loader.optional("PAYMENT_FAKE_SECRET"),
		},
		Mail: MailConfig{
			Driver:    loader.must("MAIL_DRIVER"),
			Host:      loader.optional("MAIL_HOST"),
			Port:      loader.optionalInt("MAIL_PORT", 587),
			Username:  loader.optional("MAIL_USERNAME"),
			Password:  loader.optional("MAIL_PASSWORD"),
			FromName:  loader.must("MAIL_FROM_NAME"),
			FromEmail: loader.must("MAIL_FROM_EMAIL"),
			ShopName:  loader.must("APP_NAME"),
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
	if err := c.Payment.validate(c.IsProduction()); err != nil {
		errs = append(errs, fmt.Errorf("payment: %w", err))
	}
	if err := c.Mail.validate(); err != nil {
		errs = append(errs, fmt.Errorf("mail: %w", err))
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

func (p PaymentConfig) validate(isProd bool) error {
	switch p.Provider {
	case "midtrans":
		if p.ServerKey == "" {
			return errors.New("PAYMENT_SERVER_KEY wajib diisi untuk provider midtrans")
		}
	case "fake":
		// Ini penjaga paling penting di seluruh file: FakeGateway menerima
		// notifikasi pembayaran yang ditandatangani sendiri. Kalau hidup di
		// production, siapa pun bisa menandai pesanannya lunas.
		if isProd {
			return errors.New("provider 'fake' tidak boleh dipakai di production")
		}
	default:
		return fmt.Errorf("PAYMENT_PROVIDER tidak dikenal: %q", p.Provider)
	}
	return nil
}
 
func (m MailConfig) validate() error {
	switch m.Driver {
	case "smtp":
		if m.Host == "" {
			return errors.New("MAIL_HOST wajib diisi untuk driver smtp")
		}
		if m.Port <= 0 {
			return errors.New("MAIL_PORT harus lebih besar dari 0")
		}
	case "log":
		// Tidak butuh apa-apa.
	default:
		return fmt.Errorf("MAIL_DRIVER tidak dikenal: %q", m.Driver)
	}
	if m.FromEmail == "" {
		return errors.New("MAIL_FROM_EMAIL wajib diisi")
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

// AccessExpireDuration returns the JWT access token lifetime as time.Duration.
func (j JWTConfig) AccessExpireDuration() time.Duration {
	return time.Duration(j.AccessExpireMinute) * time.Minute
}

// RefreshExpireDuration returns the JWT refresh token lifetime as time.Duration.
func (j JWTConfig) RefreshExpireDuration() time.Duration {
	return time.Duration(j.RefreshExpireHour) * time.Hour
}

// Constants for validation rules.
const (
	minSecretLength = 32
)

// --------- env loader ---------

// envLoader collects errors while reading environment variables so that
// all missing or invalid values can be reported in a single error, rather
// than failing one at a time.
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

func (e *envLoader) optionalInt(key string, fallback int) int {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		e.errs = append(e.errs, fmt.Errorf("%s harus berupa angka (dapat %q)", key, val))
		return fallback
	}
	return parsed
}
