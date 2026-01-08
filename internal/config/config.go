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

func Load() (*Config, error) {
	cfg := &Config{
		Environment: getEnv("ENVIRONMENT", "development"),
		Server: ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         getEnv("SERVER_PORT", "8080"),
			ReadTimeout:  getEnvInt("SERVER_READ_TIMEOUT", 15),
			WriteTimeout: getEnvInt("SERVER_WRITE_TIMEOUT", 15),
			IdleTimeout:  getEnvInt("SERVER_IDLE_TIMEOUT", 60),
		},
		JWT: JWTConfig{
			SecretKey: getEnv("JWT_SECRET_KEY", "your-secret-key-change-in-production"),
			Issuer:    getEnv("JWT_ISSUER", "api-gateway"),
			Audience:  getEnv("JWT_AUDIENCE", "api"),
			ExpiresIn: getEnvInt("JWT_EXPIRES_IN", 3600),
		},
		CORS: CORSConfig{
			AllowedOrigins:   parseStringSlice(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:4200")),
			AllowedMethods:   parseStringSlice(getEnv("CORS_ALLOWED_METHODS", "GET,POST,PUT,DELETE,PATCH,OPTIONS")),
			AllowedHeaders:   parseStringSlice(getEnv("CORS_ALLOWED_HEADERS", "Content-Type,Authorization")),
			ExposedHeaders:   parseStringSlice(getEnv("CORS_EXPOSED_HEADERS", "Content-Length,X-Total-Count")),
			AllowCredentials: getEnvBool("CORS_ALLOW_CREDENTIALS", true),
			MaxAge:           getEnvInt("CORS_MAX_AGE", 3600),
		},
		RateLimit: RateLimitConfig{
			Enabled:           getEnvBool("RATE_LIMIT_ENABLED", true),
			RequestsPerMinute: getEnvInt("RATE_LIMIT_REQUESTS_PER_MINUTE", 60),
			BurstSize:         getEnvInt("RATE_LIMIT_BURST_SIZE", 10),
		},
		Cache: CacheConfig{
			Enabled: getEnvBool("CACHE_ENABLED", false),
			TTL:     getEnvInt("CACHE_TTL", 300),
			MaxSize: getEnvInt("CACHE_MAX_SIZE", 1000),
			Redis: RedisConfig{
				Host:     getEnv("REDIS_HOST", "localhost"),
				Port:     getEnv("REDIS_PORT", "6379"),
				Password: getEnv("REDIS_PASSWORD", ""),
				DB:       getEnvInt("REDIS_DB", 0),
			},
		},
		Logging: LoggingConfig{
			Level:      getEnv("LOG_LEVEL", "info"),
			JSONFormat: getEnvBool("LOG_JSON_FORMAT", true),
		},
	}

	// Load upstream services from environment or file
	if err := cfg.loadUpstreamServices(); err != nil {
		return nil, fmt.Errorf("failed to load upstream services: %w", err)
	}

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

	for i := range serviceCount {
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
