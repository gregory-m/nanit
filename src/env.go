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

func EnvVarBool(varName string, defaultValue bool) bool {
	value := EnvVarStr(varName, "")
	if value == "true" {
		return true
	} else if value == "false" {
		return false
	} else if value == "" {
		return defaultValue
	}

	log.Fatal().Msgf("Unexpected value for boolean environment variable %v (allowed values true, false)", varName)
	return false
}
