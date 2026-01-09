package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Environment string
	Server      ServerConfig
	JWT         JWTConfig
	Upstream    UpstreamConfig
	CORS        CORSConfig
	RateLimit   RateLimitConfig
	Cache       CacheConfig
	Logging     LoggingConfig
	Database    DatabaseConfig
}

type ServerConfig struct {
	Host         string
	Port         string
	ReadTimeout  int
	WriteTimeout int
	IdleTimeout  int
}

type JWTConfig struct {
	SecretKey string
	Issuer    string
	Audience  string
	ExpiresIn int
}

type UpstreamConfig struct {
	Services []ServiceConfig
}

type ServiceConfig struct {
	Name     string
	URL      string
	Timeout  int
	MaxRetry int
}

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

type RateLimitConfig struct {
	Enabled           bool
	RequestsPerMinute int
	BurstSize         int
}

type CacheConfig struct {
	Enabled bool
	TTL     int
	MaxSize int
	Redis   RedisConfig
}

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type LoggingConfig struct {
	Level      string
	JSONFormat bool
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSL      bool
}

func Load() (*Config, error) {
	fmt.Println("---------------[ Load .env ]---------------")
	cfg := &Config{
		Environment: getEnv("ENVIRONMENT", ""),
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", ""),
			Port:         getEnv("SERVER_PORT", ""),
			ReadTimeout:  getEnvInt("SERVER_READ_TIMEOUT", 0),
			WriteTimeout: getEnvInt("SERVER_WRITE_TIMEOUT", 0),
			IdleTimeout:  getEnvInt("SERVER_IDLE_TIMEOUT", 0),
		},
		JWT: JWTConfig{
			SecretKey: getEnv("JWT_SECRET_KEY", ""),
			Issuer:    getEnv("JWT_ISSUER", ""),
			Audience:  getEnv("JWT_AUDIENCE", ""),
			ExpiresIn: getEnvInt("JWT_EXPIRES_IN", 0),
		},
		CORS: CORSConfig{
			AllowedOrigins:   parseStringSlice(getEnv("CORS_ALLOWED_ORIGINS", "")),
			AllowedMethods:   parseStringSlice(getEnv("CORS_ALLOWED_METHODS", "")),
			AllowedHeaders:   parseStringSlice(getEnv("CORS_ALLOWED_HEADERS", "")),
			ExposedHeaders:   parseStringSlice(getEnv("CORS_EXPOSED_HEADERS", "")),
			AllowCredentials: getEnvBool("CORS_ALLOW_CREDENTIALS", false),
			MaxAge:           getEnvInt("CORS_MAX_AGE", 0),
		},
		RateLimit: RateLimitConfig{
			Enabled:           getEnvBool("RATE_LIMIT_ENABLED", false),
			RequestsPerMinute: getEnvInt("RATE_LIMIT_REQUESTS_PER_MINUTE", 0),
			BurstSize:         getEnvInt("RATE_LIMIT_BURST_SIZE", 0),
		},
		Cache: CacheConfig{
			Enabled: getEnvBool("CACHE_ENABLED", false),
			TTL:     getEnvInt("CACHE_TTL", 0),
			MaxSize: getEnvInt("CACHE_MAX_SIZE", 0),
			Redis: RedisConfig{
				Host:     getEnv("REDIS_HOST", ""),
				Port:     getEnv("REDIS_PORT", ""),
				Password: getEnv("REDIS_PASSWORD", ""),
				DB:       getEnvInt("REDIS_DB", 0),
			},
		},
		Logging: LoggingConfig{
			Level:      getEnv("LOG_LEVEL", ""),
			JSONFormat: getEnvBool("LOG_JSON_FORMAT", false),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DATABASE_HOST", ""),
			Port:     getEnv("DATABASE_PORT", ""),
			User:     getEnv("DATABASE_USER", ""),
			Password: getEnv("DATABASE_PASSWORD", ""),
			Name:     getEnv("DATABASE_NAME", ""),
			SSL:      getEnvBool("DATABASE_SSL", false),
		},
	}

	// Load upstream services from environment or file
	if err := cfg.loadUpstreamServices(); err != nil {
		return nil, fmt.Errorf("failed to load upstream services: %w", err)
	}
	println(cfg)
	return cfg, nil
}

func (c *Config) loadUpstreamServices() error {
	servicesYAML := getEnv("UPSTREAM_SERVICES_FILE", "config/services.yaml")

	data, err := os.ReadFile(servicesYAML)
	if err != nil {
		// Fallback to environment variables if file not found
		return c.loadUpstreamServicesFromEnv()
	}

	var services []ServiceConfig
	if err := yaml.Unmarshal(data, &services); err != nil {
		return fmt.Errorf("failed to parse services YAML: %w", err)
	}

	c.Upstream.Services = services
	return nil
}

func (c *Config) loadUpstreamServicesFromEnv() error {
	// Example: UPSTREAM_SERVICE_0_NAME=api UPSTREAM_SERVICE_0_URL=http://localhost:3000
	serviceCount := getEnvInt("UPSTREAM_SERVICE_COUNT", 1)

	for i := 0; i < serviceCount; i++ {
		prefix := fmt.Sprintf("UPSTREAM_SERVICE_%d_", i)
		name := getEnv(prefix+"NAME", fmt.Sprintf("service-%d", i))
		url := getEnv(prefix+"URL", "http://localhost:3000")

		if url == "" {
			continue
		}

		c.Upstream.Services = append(c.Upstream.Services, ServiceConfig{
			Name:     name,
			URL:      url,
			Timeout:  getEnvInt(prefix+"TIMEOUT", 30),
			MaxRetry: getEnvInt(prefix+"MAX_RETRY", 3),
		})
	}

	if len(c.Upstream.Services) == 0 {
		return fmt.Errorf("no upstream services configured")
	}

	return nil
}

// GetDatabaseDSN returns PostgreSQL connection string
// Format: postgres://user:password@host:port/dbname?sslmode=disable
func (c *Config) GetDatabaseDSN() string {
	sslMode := "disable"
	if c.Database.SSL {
		sslMode = "require"
	}

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.Name,
		sslMode,
	)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

func getEnvBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	return strings.ToLower(valueStr) == "true" || strings.ToLower(valueStr) == "1"
}

func parseStringSlice(input string) []string {
	var result []string
	for _, v := range strings.Split(input, ",") {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
