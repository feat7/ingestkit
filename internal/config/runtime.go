package config

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type APIEnv struct {
	ListenAddress  string
	Brokers        []string
	EventTopic     string
	APIKeys        map[string]string
	RateLimitRPS   int
	PublishTimeout time.Duration
}
type ConsumerEnv struct {
	ListenAddress                              string
	Brokers                                    []string
	EventTopic, DLQTopic, GroupID, DatabaseURL string
}

func value(get func(string) string, k, d string) string {
	v := get(k)
	if v == "" {
		return d
	}
	return v
}
func brokers(get func(string) string) []string {
	return strings.Split(value(get, "BROKERS", "localhost:19092"), ",")
}
func LoadAPIEnv(get func(string) string) (APIEnv, error) {
	c := APIEnv{ListenAddress: value(get, "HTTP_ADDR", ":8080"), Brokers: brokers(get), EventTopic: value(get, "EVENT_TOPIC", "ingestkit.events"), RateLimitRPS: 1000, PublishTimeout: 5 * time.Second}
	var err error
	c.APIKeys, err = ParseAPIKeys(get("API_KEYS"))
	if err != nil || len(c.APIKeys) == 0 {
		return c, errors.New("API_KEYS must be a nonempty JSON key-to-tenant map")
	}
	for key, tenant := range c.APIKeys {
		if key == "" || len(tenant) < 1 || len(tenant) > 128 {
			return c, errors.New("invalid API_KEYS mapping")
		}
	}
	if raw := get("RATE_LIMIT_RPS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return c, errors.New("RATE_LIMIT_RPS must be positive")
		}
		c.RateLimitRPS = n
	}
	return c, nil
}
func LoadConsumerEnv(get func(string) string) (ConsumerEnv, error) {
	c := ConsumerEnv{ListenAddress: value(get, "METRICS_ADDR", ":8081"), Brokers: brokers(get), EventTopic: value(get, "EVENT_TOPIC", "ingestkit.events"), DLQTopic: value(get, "DLQ_TOPIC", "ingestkit.dlq"), GroupID: value(get, "CONSUMER_GROUP", "ingestkit-consumer"), DatabaseURL: get("DATABASE_URL")}
	u, err := url.Parse(c.DatabaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return c, errors.New("DATABASE_URL must be a PostgreSQL URL")
	}
	return c, nil
}
