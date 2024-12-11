package env

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func GetEnv(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}

func GetEnvBool(key string, defaultValue bool) bool {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	parse, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parse
}

func GetEnvInt64(key string, defaultValue int64) int64 {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	parse, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return defaultValue
	}
	return parse
}

func GetEnvDuration(key string, defaultValue time.Duration) time.Duration {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	parse, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return parse
}

func GetEnvValues(key string) []string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return []string{}
	}

	return strings.Split(value, ",")
}

func RequireEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		panic(fmt.Sprintf("required env variable %s not found", key))
	}
	return value
}

func RequireEnvInt64(key string) int64 {
	value, exists := os.LookupEnv(key)
	if !exists {
		panic(fmt.Sprintf("required env variable %s not found", key))
	}

	parse, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		panic(fmt.Sprintf("failed to parse env variable %s: %v", key, err))
	}

	return parse
}

func RequireEnvBool(key string) bool {
	value, exists := os.LookupEnv(key)
	if !exists {
		panic(fmt.Sprintf("required env variable %s not found", key))
	}

	parse, err := strconv.ParseBool(value)
	if err != nil {
		panic(fmt.Sprintf("failed to parse env variable %s: %v", key, err))
	}

	return parse
}
