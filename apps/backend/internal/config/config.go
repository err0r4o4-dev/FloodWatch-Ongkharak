package config

import (
	"fmt"
	"net/netip"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultReadinessTimeout = 2 * time.Second

const maxFixtureSourceCodeLength = 64

type Config struct {
	Environment                string
	APIPort                    int
	DatabaseURL                string
	RedisURL                   string
	MinIOEndpoint              string
	MinIOAccessKey             string
	MinIOSecretKey             string
	MinIOBucket                string
	CORSAllowedOrigins         []string
	TrustedProxyCIDRs          []netip.Prefix
	JWTIssuer                  string
	JWTSecret                  string
	JWTAccessTTL               time.Duration
	RefreshTokenTTL            time.Duration
	CookieName                 string
	CookieSecure               bool
	CookieSameSite             http.SameSite
	AIServiceURL               string
	SMTPHost                   string
	SMTPPort                   int
	SMTPUsername               string
	SMTPPassword               string
	SMTPFrom                   string
	SMTPRequireTLS             bool
	SwaggerEnabled             bool
	ReadinessTimeout           time.Duration
	IngestionFixtureURL        string
	IngestionFixtureSourceCode string
	VAPIDPublicKey             string
	VAPIDPrivateKey            string
	VAPIDSubscriber            string
	GoogleClientID             string
	GoogleClientSecret         string
	GoogleRedirectURI          string
	AuthFrontendOrigin         string
}

func Load() (Config, error) {
	environment, err := required("APP_ENV")
	if err != nil {
		return Config{}, err
	}
	if environment != "development" && environment != "test" && environment != "production" {
		return Config{}, fmt.Errorf("APP_ENV must be development, test, or production")
	}

	apiPort, err := port("API_PORT")
	if err != nil {
		return Config{}, err
	}
	databaseURL, err := requiredURL("DATABASE_URL", "postgres", "postgresql")
	if err != nil {
		return Config{}, err
	}
	redisURL, err := requiredURL("REDIS_URL", "redis", "rediss")
	if err != nil {
		return Config{}, err
	}
	minioEndpoint, err := requiredURL("MINIO_ENDPOINT", "http", "https")
	if err != nil {
		return Config{}, err
	}
	if minioEndpoint.Path != "" && minioEndpoint.Path != "/" {
		return Config{}, fmt.Errorf("MINIO_ENDPOINT must not include a path")
	}
	minioAccessKey, err := required("MINIO_ACCESS_KEY")
	if err != nil {
		return Config{}, err
	}
	minioSecretKey, err := required("MINIO_SECRET_KEY")
	if err != nil {
		return Config{}, err
	}
	minioBucket, err := required("MINIO_BUCKET")
	if err != nil {
		return Config{}, err
	}
	corsOrigins, err := origins("CORS_ALLOWED_ORIGINS")
	if err != nil {
		return Config{}, err
	}
	trustedProxyCIDRs, err := trustedProxyCIDRs()
	if err != nil {
		return Config{}, err
	}
	googleSettings, err := loadGoogle(environment, corsOrigins)
	if err != nil {
		return Config{}, err
	}
	smtpSettings, err := loadSMTP(environment)
	if err != nil {
		return Config{}, err
	}
	jwtIssuer := strings.TrimSpace(os.Getenv("JWT_ISSUER"))
	if jwtIssuer == "" {
		jwtIssuer = "floodwatch-ongkharak-local"
		if environment == "production" {
			jwtIssuer = "floodwatch-ongkharak"
		}
	}
	jwtSecret, err := required("JWT_SECRET")
	if err != nil {
		return Config{}, err
	}
	if len(jwtSecret) < 32 || len(jwtSecret) > 4096 || (environment == "production" && (strings.Contains(strings.ToLower(jwtSecret), "development-only") || strings.Contains(strings.ToLower(jwtSecret), "dev-only") || strings.Contains(strings.ToLower(jwtSecret), "local-only"))) {
		return Config{}, fmt.Errorf("JWT_SECRET must be a strong non-development secret of 32-4096 bytes")
	}
	accessTTL, err := duration("JWT_ACCESS_TTL", 15*time.Minute, time.Minute, time.Hour)
	if err != nil {
		return Config{}, err
	}
	refreshTTL, err := duration("REFRESH_TOKEN_TTL", 7*24*time.Hour, time.Hour, 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	if refreshTTL <= accessTTL {
		return Config{}, fmt.Errorf("REFRESH_TOKEN_TTL must exceed JWT_ACCESS_TTL")
	}
	cookieName, err := required("COOKIE_NAME")
	if err != nil {
		return Config{}, err
	}
	if len(cookieName) > 64 || !validCookieName(cookieName) {
		return Config{}, fmt.Errorf("COOKIE_NAME must contain only letters, digits, underscores, or hyphens")
	}
	cookieSecureValue, err := required("COOKIE_SECURE")
	if err != nil {
		return Config{}, err
	}
	cookieSecure, err := strconv.ParseBool(cookieSecureValue)
	if err != nil || (environment == "production" && !cookieSecure) {
		return Config{}, fmt.Errorf("COOKIE_SECURE must be true in production and a valid boolean elsewhere")
	}
	cookieSameSiteValue, err := required("COOKIE_SAMESITE")
	if err != nil {
		return Config{}, err
	}
	var cookieSameSite http.SameSite
	switch cookieSameSiteValue {
	case "Lax":
		cookieSameSite = http.SameSiteLaxMode
	case "Strict":
		cookieSameSite = http.SameSiteStrictMode
	default:
		return Config{}, fmt.Errorf("COOKIE_SAMESITE must be Lax or Strict")
	}
	aiServiceURL, err := requiredURL("AI_SERVICE_URL", "http", "https")
	if err != nil {
		return Config{}, err
	}
	swaggerEnabled, err := swaggerEnabled(environment)
	if err != nil {
		return Config{}, err
	}
	fixtureURL, fixtureSourceCode, err := ingestionFixture(environment)
	if err != nil {
		return Config{}, err
	}
	vapidPublicKey := strings.TrimSpace(os.Getenv("VAPID_PUBLIC_KEY"))
	vapidPrivateKey := strings.TrimSpace(os.Getenv("VAPID_PRIVATE_KEY"))
	vapidSubscriber := strings.TrimSpace(os.Getenv("VAPID_SUBSCRIBER"))
	if environment == "production" || vapidPublicKey != "" || vapidPrivateKey != "" || vapidSubscriber != "" {
		for _, entry := range []struct{ name, value string }{
			{"VAPID_PUBLIC_KEY", vapidPublicKey},
			{"VAPID_PRIVATE_KEY", vapidPrivateKey},
			{"VAPID_SUBSCRIBER", vapidSubscriber},
		} {
			if entry.value == "" {
				return Config{}, fmt.Errorf("%s is required", entry.name)
			}
		}
	}

	return Config{
		Environment:                environment,
		APIPort:                    apiPort,
		DatabaseURL:                databaseURL.String(),
		RedisURL:                   redisURL.String(),
		MinIOEndpoint:              strings.TrimRight(minioEndpoint.String(), "/"),
		MinIOAccessKey:             minioAccessKey,
		MinIOSecretKey:             minioSecretKey,
		MinIOBucket:                minioBucket,
		CORSAllowedOrigins:         corsOrigins,
		TrustedProxyCIDRs:          trustedProxyCIDRs,
		JWTIssuer:                  jwtIssuer,
		JWTSecret:                  jwtSecret,
		JWTAccessTTL:               accessTTL,
		RefreshTokenTTL:            refreshTTL,
		CookieName:                 cookieName,
		CookieSecure:               cookieSecure,
		CookieSameSite:             cookieSameSite,
		AIServiceURL:               aiServiceURL.String(),
		SMTPHost:                   smtpSettings.Host,
		SMTPPort:                   smtpSettings.Port,
		SMTPUsername:               smtpSettings.Username,
		SMTPPassword:               smtpSettings.Password,
		SMTPFrom:                   smtpSettings.From,
		SMTPRequireTLS:             smtpSettings.RequireTLS,
		SwaggerEnabled:             swaggerEnabled,
		ReadinessTimeout:           defaultReadinessTimeout,
		IngestionFixtureURL:        fixtureURL,
		IngestionFixtureSourceCode: fixtureSourceCode,
		VAPIDPublicKey:             vapidPublicKey,
		VAPIDPrivateKey:            vapidPrivateKey,
		VAPIDSubscriber:            vapidSubscriber,
		GoogleClientID:             googleSettings.clientID,
		GoogleClientSecret:         googleSettings.clientSecret,
		GoogleRedirectURI:          googleSettings.redirectURI,
		AuthFrontendOrigin:         googleSettings.frontendOrigin,
	}, nil
}

func ingestionFixture(environment string) (string, string, error) {
	fixtureURL := strings.TrimSpace(os.Getenv("INGESTION_FIXTURE_URL"))
	fixtureSourceCode := strings.TrimSpace(os.Getenv("INGESTION_FIXTURE_SOURCE_CODE"))
	if fixtureURL == "" && fixtureSourceCode == "" {
		return "", "", nil
	}
	if environment == "production" {
		return "", "", fmt.Errorf("INGESTION_FIXTURE_URL is only allowed in development or test")
	}
	if fixtureURL == "" {
		return "", "", fmt.Errorf("INGESTION_FIXTURE_URL is required when INGESTION_FIXTURE_SOURCE_CODE is set")
	}
	if fixtureSourceCode == "" {
		return "", "", fmt.Errorf("INGESTION_FIXTURE_SOURCE_CODE is required when INGESTION_FIXTURE_URL is set")
	}
	parsed, err := requiredURLValue("INGESTION_FIXTURE_URL", fixtureURL, "http", "https")
	if err != nil {
		return "", "", err
	}
	if parsed.Path != "" && parsed.Path != "/" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
		return "", "", fmt.Errorf("INGESTION_FIXTURE_URL must be an origin URL")
	}
	if !validFixtureSourceCode(fixtureSourceCode) {
		return "", "", fmt.Errorf("INGESTION_FIXTURE_SOURCE_CODE must be a SYNTH_* source code")
	}
	return strings.TrimRight(parsed.String(), "/"), fixtureSourceCode, nil
}

func validFixtureSourceCode(value string) bool {
	if !strings.HasPrefix(value, "SYNTH_") || len(value) > maxFixtureSourceCodeLength {
		return false
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func duration(name string, fallback, minimum, maximum time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, fmt.Errorf("%s must be a duration between %s and %s", name, minimum, maximum)
	}
	return parsed, nil
}

func validCookieName(name string) bool {
	for _, character := range name {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func required(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func requiredURL(name string, schemes ...string) (*url.URL, error) {
	value, err := required(name)
	if err != nil {
		return nil, err
	}
	return requiredURLValue(name, value, schemes...)
}

func requiredURLValue(name, value string, schemes ...string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("%s must be an absolute URL", name)
	}
	for _, scheme := range schemes {
		if parsed.Scheme == scheme {
			return parsed, nil
		}
	}
	return nil, fmt.Errorf("%s has an unsupported URL scheme", name)
}

func port(name string) (int, error) {
	value, err := required(name)
	if err != nil {
		return 0, err
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || parsed > 65535 {
		return 0, fmt.Errorf("%s must be a valid TCP port", name)
	}
	return parsed, nil
}

func origins(name string) ([]string, error) {
	value, err := required(name)
	if err != nil {
		return nil, err
	}
	origins := strings.Split(value, ",")
	for index, origin := range origins {
		parsed, parseErr := url.ParseRequestURI(strings.TrimSpace(origin))
		if parseErr != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, fmt.Errorf("%s contains an invalid origin", name)
		}
		origins[index] = parsed.String()
	}
	return origins, nil
}

func swaggerEnabled(environment string) (bool, error) {
	if environment == "production" {
		return false, nil
	}
	value := strings.TrimSpace(os.Getenv("SWAGGER_ENABLED"))
	if value == "" {
		return environment == "development", nil
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("SWAGGER_ENABLED must be true or false")
	}
	return enabled, nil
}
