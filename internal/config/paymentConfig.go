// paymentConfig.go — potongan yang perlu DITAMBAHKAN ke internal/config/config.go
//
// Bukan file utuh: salin bagian yang relevan ke config.go milikmu.

package config

/*
========================================================================
1. Tambahkan dua struct ini
========================================================================

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

========================================================================
2. Tambahkan ke struct Config
========================================================================

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	CORS     CORSConfig
	Payment  PaymentConfig   // ⬅ baru
	Mail     MailConfig      // ⬅ baru
}

========================================================================
3. Tambahkan ke Load(), di dalam literal cfg
========================================================================

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

========================================================================
4. Tambahkan validasi ke Config.validate()
========================================================================

	if err := c.Payment.validate(c.IsProduction()); err != nil {
		errs = append(errs, fmt.Errorf("payment: %w", err))
	}
	if err := c.Mail.validate(); err != nil {
		errs = append(errs, fmt.Errorf("mail: %w", err))
	}

========================================================================
5. Tambahkan method validasi
========================================================================

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

========================================================================
6. Tambahkan helper ini ke envLoader
========================================================================

// optionalInt membaca env opsional bertipe angka, memakai fallback kalau
// tidak diisi. Berbeda dari mustInt yang mencatat error saat kosong.
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
*/