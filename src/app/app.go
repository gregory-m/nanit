package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func loadDotEnvFile() {
	absFilepath, filePathErr := filepath.Abs(".env")
	if filePathErr != nil {
		log.Fatal().Str("path", absFilepath).Err(filePathErr).Msg("Unable to retrieve absolute file path")
	}

	// loads values from .env into the system
	if err := godotenv.Load(absFilepath); err != nil {
		log.Info().Str("path", absFilepath).Msg("No .env file found. Using only environment variables")
	} else {
		log.Info().Str("path", absFilepath).Msg("Additional environment variables loaded from .env file")
	}
}

// Set log level after env. initialization
func setLogLevel() {
	// Try to read log level from env. variable
	logLevelStr := EnvVarStr("NANIT_LOG_LEVEL", "info")
	logLevel, _ := zerolog.ParseLevel(logLevelStr)
	if logLevel == zerolog.NoLevel {
		log.Fatal().Str("value", logLevelStr).Msg("Unknown log level specified")
	}

	log.Info().Msgf("Setting log level to %v", logLevel)
	zerolog.SetGlobalLevel(logLevel)
}

// Set logger for application bootstrap
func initLogger() {
	// Initial log level, overridden later by setLogLevel
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC822}
	log.Logger = log.Output(consoleWriter)
}

type DataDirectories struct {
	BaseDir  string
	VideoDir string
	LogDir   string
}

func ensureDataDirectories() DataDirectories {
	relDataDir := EnvVarStr("NANIT_DATA_DIR", "data")

	absDataDir, filePathErr := filepath.Abs(relDataDir)
	if filePathErr != nil {
		log.Fatal().Str("path", relDataDir).Err(filePathErr).Msg("Unable to retrieve absolute file path")
	}

	// Create base data directory if it does not exist
	if _, err := os.Stat(absDataDir); os.IsNotExist(err) {
		log.Warn().Str("dir", absDataDir).Msg("Data directory does not exist, creating")
		mkdirErr := os.MkdirAll(absDataDir, 0755)
		if mkdirErr != nil {
			log.Fatal().Str("path", absDataDir).Err(mkdirErr).Msg("Unable to create a directory")
		}
	}

	// Create data dir skeleton
	for _, subdirName := range []string{"video", "log"} {
		absSubdir := filepath.Join(absDataDir, subdirName)

		if _, err := os.Stat(absSubdir); os.IsNotExist(err) {
			mkdirErr := os.Mkdir(absSubdir, 0755)
			if mkdirErr != nil {
				log.Fatal().Str("path", absDataDir).Err(mkdirErr).Msg("Unable to create a directory")
			} else {
				log.Info().Str("dir", absSubdir).Msgf("Directory created ./%v", subdirName)
			}
		}
	}

	return DataDirectories{
		BaseDir:  absDataDir,
		VideoDir: filepath.Join(absDataDir, "video"),
		LogDir:   filepath.Join(absDataDir, "log"),
	}
}

var validUID = regexp.MustCompile(`^[a-z0-9_-]+$`)

// Checks that Baby UID does not contain any bad characters
// This is necessary because we use it as part of file paths
func ensureValidBabyUID(babyUID string) {
	if !validUID.MatchString(babyUID) {
		log.Fatal().Str("uid", babyUID).Msg("Baby UID contains unsafe characters")
	}
}

func initSessionStore() *AppSessionStore {
	sessionStore := new(AppSessionStore)

	// Load previous state of the application from session file
	sessionFile := EnvVarStr("NANIT_SESSION_FILE", "")
	if sessionFile != "" {

		absFileName, filePathErr := filepath.Abs(sessionFile)
		if filePathErr != nil {
			log.Fatal().Str("path", sessionFile).Err(filePathErr).Msg("Unable to retrieve absolute file path")
		}

		sessionStore.Filename = absFileName
		sessionStore.Load()
	}

	return sessionStore
}

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	initLogger()
	loadDotEnvFile()
	setLogLevel()
	dataDirectories := ensureDataDirectories()
	sessionStore := initSessionStore()

	api := NanitClient{
		Email:        EnvVarReqStr("NANIT_EMAIL"),
		Password:     EnvVarReqStr("NANIT_PASSWORD"),
		SessionStore: sessionStore,
	}

	// Fetches babies info if they are not present in session
	api.EnsureBabies()

	babiesStreamClosers := make([]func(), len(sessionStore.Session.Babies))

	// Start reading the data from the stream
	for i, baby := range sessionStore.Session.Babies {
		babiesStreamClosers[i] = startStream(baby.UID, sessionStore.Session.AuthToken, dataDirectories)
	}

	// Start serving content over HTTP
	go serve(sessionStore.Session.Babies, dataDirectories)

	for {
		select {
		case <-interrupt:
			log.Warn().Msg("Received interrupt signal, terminating")
			for _, closeBabyStream := range babiesStreamClosers {
				closeBabyStream()
			}

			return
		}
	}
}
