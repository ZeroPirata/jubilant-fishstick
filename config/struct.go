package config

import "time"

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Cache    CacheConfig
	Project  ProjectConfig
	Ai       AiConfig
}

type ServerConfig struct {
	Host string `env:"SERVER_HOST,required"`
	Port string `env:"SERVER_PORT,required"`
}

type AiConfig struct {
	Key   string `env:"API_KEY_AI"`
	Url   string `env:"API_URL_AI"`
	Model string `env:"MODEL_AI"`
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
