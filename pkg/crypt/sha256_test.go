package crypt_test

import (
	"testing"

	"github.com/devshark/wallet/pkg/crypt"
)

func TestComputeSHA256(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "hello world",
			input:    "hello world",
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name:     "numeric input",
			input:    "12345",
			expected: "5994471abb01112afcc18159f6cc74b4f511b99806da59b3caf5a9c173cacfc5",
		},
		{
			name:     "special characters",
			input:    "!@#$%^&*()",
			expected: "95ce789c5c9d18490972709838ca3a9719094bca3ac16332cfec0652b0236141",
		},
		{
			name:     "long input",
			input:    "This is a long input string that exceeds 64 characters to test the SHA256 function",
			expected: "8ac86a542607c3c1aab30942fe4dfaf5b39151272e9f873b4078b810fb70dc4e",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := crypt.ComputeSHA256(tt.input)
			if result != tt.expected {
				t.Errorf("ComputeSHA256(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestComputeSHA256_Consistency(t *testing.T) {
	input := "test input"
	firstResult := crypt.ComputeSHA256(input)
	secondResult := crypt.ComputeSHA256(input)

	if firstResult != secondResult {
		t.Errorf("ComputeSHA256 is not consistent: first result %v, second result %v", firstResult, secondResult)
	}
}

func TestComputeSHA256_DifferentInputs(t *testing.T) {
	input1 := "input1"
	input2 := "input2"

	result1 := crypt.ComputeSHA256(input1)
	result2 := crypt.ComputeSHA256(input2)

	if result1 == result2 {
		t.Errorf("ComputeSHA256 produced the same hash for different inputs: %v", result1)
	}
}
