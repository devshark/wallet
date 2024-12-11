package env_test

import (
	"os"
	"testing"
	"time"

	"github.com/devshark/wallet/pkg/env"
	"github.com/stretchr/testify/assert"
)

func TestGetEnv(t *testing.T) {
	t.Run("existing environment variable", func(t *testing.T) {
		os.Setenv("TEST_VAR", "test_value")
		defer os.Unsetenv("TEST_VAR")

		value := env.GetEnv("TEST_VAR", "default")
		assert.Equal(t, "test_value", value)
	})

	t.Run("non-existing environment variable", func(t *testing.T) {
		value := env.GetEnv("NON_EXISTING_VAR", "default")
		assert.Equal(t, "default", value)
	})
}

func TestGetEnvBool(t *testing.T) {
	t.Run("existing boolean environment variable", func(t *testing.T) {
		os.Setenv("TEST_BOOL", "true")
		defer os.Unsetenv("TEST_BOOL")

		value := env.GetEnvBool("TEST_BOOL", false)
		assert.True(t, value)
	})

	t.Run("non-existing boolean environment variable", func(t *testing.T) {
		value := env.GetEnvBool("NON_EXISTING_BOOL", true)
		assert.True(t, value)
	})

	t.Run("invalid boolean environment variable", func(t *testing.T) {
		os.Setenv("INVALID_BOOL", "not_a_bool")
		defer os.Unsetenv("INVALID_BOOL")

		value := env.GetEnvBool("INVALID_BOOL", true)
		assert.True(t, value)
	})
}

func TestGetEnvInt64(t *testing.T) {
	t.Run("existing int64 environment variable", func(t *testing.T) {
		os.Setenv("TEST_INT", "42")
		defer os.Unsetenv("TEST_INT")

		value := env.GetEnvInt64("TEST_INT", 0)
		assert.Equal(t, int64(42), value)
	})

	t.Run("non-existing int64 environment variable", func(t *testing.T) {
		value := env.GetEnvInt64("NON_EXISTING_INT", 100)
		assert.Equal(t, int64(100), value)
	})

	t.Run("invalid int64 environment variable", func(t *testing.T) {
		os.Setenv("INVALID_INT", "not_an_int")
		defer os.Unsetenv("INVALID_INT")

		value := env.GetEnvInt64("INVALID_INT", 200)
		assert.Equal(t, int64(200), value)
	})
}

func TestGetEnvDuration(t *testing.T) {
	t.Run("existing duration environment variable", func(t *testing.T) {
		os.Setenv("TEST_DURATION", "5s")
		defer os.Unsetenv("TEST_DURATION")

		value := env.GetEnvDuration("TEST_DURATION", time.Second)
		assert.Equal(t, 5*time.Second, value)
	})

	t.Run("non-existing duration environment variable", func(t *testing.T) {
		value := env.GetEnvDuration("NON_EXISTING_DURATION", 10*time.Minute)
		assert.Equal(t, 10*time.Minute, value)
	})

	t.Run("invalid duration environment variable", func(t *testing.T) {
		os.Setenv("INVALID_DURATION", "not_a_duration")
		defer os.Unsetenv("INVALID_DURATION")

		value := env.GetEnvDuration("INVALID_DURATION", 15*time.Second)
		assert.Equal(t, 15*time.Second, value)
	})
}

func TestGetEnvValues(t *testing.T) {
	t.Run("existing comma-separated environment variable", func(t *testing.T) {
		os.Setenv("TEST_VALUES", "value1,value2,value3")
		defer os.Unsetenv("TEST_VALUES")

		values := env.GetEnvValues("TEST_VALUES")
		assert.Equal(t, []string{"value1", "value2", "value3"}, values)
	})

	t.Run("non-existing environment variable", func(t *testing.T) {
		values := env.GetEnvValues("NON_EXISTING_VALUES")
		assert.Empty(t, values)
	})
}

func TestRequireEnv(t *testing.T) {
	t.Run("existing required environment variable", func(t *testing.T) {
		os.Setenv("REQUIRED_VAR", "required_value")
		defer os.Unsetenv("REQUIRED_VAR")

		value := env.RequireEnv("REQUIRED_VAR")
		assert.Equal(t, "required_value", value)
	})

	t.Run("non-existing required environment variable", func(t *testing.T) {
		assert.Panics(t, func() {
			env.RequireEnv("NON_EXISTING_REQUIRED_VAR")
		})
	})
}

func TestRequireEnvInt64(t *testing.T) {
	t.Run("existing required int64 environment variable", func(t *testing.T) {
		os.Setenv("REQUIRED_INT", "42")
		defer os.Unsetenv("REQUIRED_INT")

		value := env.RequireEnvInt64("REQUIRED_INT")
		assert.Equal(t, int64(42), value)
	})

	t.Run("non-existing required int64 environment variable", func(t *testing.T) {
		assert.Panics(t, func() {
			env.RequireEnvInt64("NON_EXISTING_REQUIRED_INT")
		})
	})

	t.Run("invalid required int64 environment variable", func(t *testing.T) {
		os.Setenv("INVALID_REQUIRED_INT", "not_an_int")
		defer os.Unsetenv("INVALID_REQUIRED_INT")

		assert.Panics(t, func() {
			env.RequireEnvInt64("INVALID_REQUIRED_INT")
		})
	})
}

func TestRequireEnvBool(t *testing.T) {
	t.Run("existing required boolean environment variable", func(t *testing.T) {
		os.Setenv("REQUIRED_BOOL", "true")
		defer os.Unsetenv("REQUIRED_BOOL")

		value := env.RequireEnvBool("REQUIRED_BOOL")
		assert.True(t, value)
	})

	t.Run("non-existing required boolean environment variable", func(t *testing.T) {
		assert.Panics(t, func() {
			env.RequireEnvBool("NON_EXISTING_REQUIRED_BOOL")
		})
	})

	t.Run("invalid required boolean environment variable", func(t *testing.T) {
		os.Setenv("INVALID_REQUIRED_BOOL", "not_a_bool")
		defer os.Unsetenv("INVALID_REQUIRED_BOOL")

		assert.Panics(t, func() {
			env.RequireEnvBool("INVALID_REQUIRED_BOOL")
		})
	})
}
