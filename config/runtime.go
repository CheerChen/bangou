package config

import (
	"os"
	"strings"
)

const (
	EnvInputDir    = "BANGOU_INPUT_DIR"
	EnvOutputDir   = "BANGOU_OUTPUT_DIR"
	EnvAria2RPCURL = "BANGOU_ARIA2_RPC_URL"
	EnvAria2Token  = "BANGOU_ARIA2_TOKEN"
)

// ResolveSetting picks DB value first, then env var, then fallback.
func ResolveSetting(dbValue, envKey, fallback string) string {
	if v := strings.TrimSpace(dbValue); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	return fallback
}
