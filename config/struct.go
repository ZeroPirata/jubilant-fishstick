package config

import "time"

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Cache    CacheConfig
	Project  ProjectConfig
	Ai       AiConfig
	Worker   WorkerConfig
	ScrapeAi ScrapeAiConfig
	Hash     HashConfig
	Jwt      JwtConfig
}

type JwtConfig struct {
	Secret     string        `env:"JWT_SECRET,required"`
	Expiration time.Duration `env:"JWT_EXPIRATION" envDefault:"24h"`
}
type HashConfig struct {
	Argon2Memory      uint32 `env:"HASH_ARGON2_MEMORY" envDefault:"32768"`  // 32MB in KB
	Argon2Parallelism uint8  `env:"HASH_ARGON2_PARALLELISM" envDefault:"1"` // Ideal for 1 vCPU in cloud
	Argon2Iterations  uint32 `env:"HASH_ARGON2_ITERATIONS" envDefault:"3"`
	Argon2SaltLen     uint32 `env:"HASH_ARGON2_SALT_LEN" envDefault:"16"`
	Argon2KeyLen      uint32 `env:"HASH_ARGON2_KEY_LEN" envDefault:"32"`
	Argon2Pepper      string `env:"HASH_ARGON2_PEPPER,required"`
}

type ServerConfig struct {
	Host              string `env:"SERVER_HOST,required"`
	Port              string `env:"SERVER_PORT,required"`
	CORSAllowedOrigin string `env:"CORS_ALLOWED_ORIGIN" envDefault:"*"`
}

type AiConfig struct {
	Key      string        `env:"API_KEY_AI"`
	Url      string        `env:"API_URL_AI"`
	Model    string        `env:"API_MODEL_AI"`
	Provider string        `env:"API_PROVIDER"`
	Timeout  time.Duration `env:"API_AI_TIMEOUT"`
}

type ScrapeAiConfig struct {
	Activate bool          `env:"SCRAPE_AI_ACTIVATE"`
	Provider string        `env:"SCRAPE_AI_PROVIDER"`
	Key      string        `env:"SCRAPE_AI_KEY"`
	Model    string        `env:"SCRAPE_AI_MODEL"`
	Url      string        `env:"SCRAPE_AI_URL"`
	Timeout  time.Duration `env:"SCRAPE_AI_TIMEOUT"`
}

type ProjectConfig struct {
	Name           string        `env:"PROJECT_NAME"`
	Version        string        `env:"VERSION"`
	Debug          bool          `env:"DEBUG" envDefault:"false"`
	LoggerFolder   string        `env:"LOGGER_FOLDER"`
	TLS            TLSConfigs    `env:"TLS"`
	ContextTimeout time.Duration `env:"CONTEXT_TIMEOUT" envDefault:"30s"`
}

type TLSConfigs struct {
	Enabled  bool   `env:"TLS_ENABLED" envDefault:"false"`
	CertFile string `env:"TLS_CERT_FILE"`
	KeyFile  string `env:"TLS_KEY_FILE"`
	CAFile   string `env:"TLS_CA_FILE"`
}

type DatabaseConfig struct {
	Host              string        `env:"POSTGRES_HOST,required"`
	Port              int           `env:"POSTGRES_PORT,required"`
	User              string        `env:"POSTGRES_USER,required"`
	Password          string        `env:"POSTGRES_PASSWORD,required"`
	Name              string        `env:"POSTGRES_DB,required"`
	SSLMode           string        `env:"POSTGRES_SSL_MODE"`
	MaxConnections    int           `env:"POSTGRES_MAX_CONNECTIONS"`
	MinConnections    int           `env:"POSTGRES_MIN_CONNECTIONS"`
	MaxConnLifetime   time.Duration `env:"POSTGRES_MAX_CONN_LIFETIME"`
	MaxConnIdleTime   time.Duration `env:"POSTGRES_MAX_CONN_IDLE_TIME"`
	HealthCheckPeriod time.Duration `env:"POSTGRES_HEALTH_CHECK_PERIOD"`
	ConnectTimeout    time.Duration `env:"POSTGRES_CONNECT_TIMEOUT"`
}

type CacheConfig struct {
	Addr     string `env:"REDIS_ADDR,required"`
	Password string `env:"REDIS_PASSWORD"`
	DB       int    `env:"REDIS_DB" envDefault:"0"`
}

type WorkerConfig struct {
	MaxConcurrent int           `env:"WORKER_MAX_CONCURRENT" envDefault:"5"`
	BatchSize     int           `env:"WORKER_BATCH_SIZE" envDefault:"20"`
	Interval      time.Duration `env:"WORKER_INTERVAL" envDefault:"30s"`
}
