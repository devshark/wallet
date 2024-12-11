package crypt

import (
	"crypto/sha256"
	"encoding/hex"
)

func ComputeSHA256(input string) string {
	// Create a new SHA256 hash
	hasher := sha256.New()

	// Write the input string to the hasher
	hasher.Write([]byte(input))

	// Get the final hash and convert it to a byte slice
	hashBytes := hasher.Sum(nil)

	// Convert the byte slice to a hexadecimal string
	hashString := hex.EncodeToString(hashBytes)

	return hashString
}
