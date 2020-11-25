package main

import (
	"os"

	"github.com/rs/zerolog/log"
)

func EnvVarStr(varName string, defaultValue string) string {
	value := os.Getenv(varName)

	if value == "" {
		return defaultValue
	}

	return value
}

func EnvVarReqStr(varName string) string {
	value := EnvVarStr(varName, "")

	if value == "" {
		log.Fatal().Msgf("Missing environment variable %v", varName)
	}

	return value
}
