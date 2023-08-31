package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/rs/zerolog/log"
	"golang.org/x/term"

	"github.com/gregory-m/nanit/pkg/client"
)

func Login(email, password, mfaChannel string) (refreshToken string) {
	var err error
	if email == "" || password == "" {
		log.Info().Msg("Doing login")
		email, password, err = getCredentials()
		fmt.Print("\n")
		if err != nil {
			log.Fatal().Err(err).Msg("Can't get credentials")
			os.Exit(1)
		}
	}

	_, refreshToken, err = LoginMaybeMFA(email, password, mfaChannel)
	var mfaErr *client.MFARequiredError
	if errors.As(err, &mfaErr) {
		fmt.Printf("MFA is enabled\nPlease enter MFA code from %s\n", mfaChannel)
		if mfaChannel != "email" {
			fmt.Print("If you like to get code by email enter \"email\" as MFA code\n")
		}

		mfaCode, err := getMFACode()
		if err != nil {
			log.Fatal().Err(err).Msg("Can't get MFA code")
			os.Exit(1)
		}

		if mfaCode == "email" {
			return Login(email, password, "email")
		}
		_, refreshToken, err = LoginMFA(email, password, mfaErr.MFAToken, mfaCode)
		if err != nil {
			log.Fatal().Err(err).Msg("Can't get MFA code")
			os.Exit(1)
		}

	} else if err != nil {
		fmt.Printf("Can't login: %s\n", err)
		os.Exit(1)
	}

	return refreshToken
}

func getCredentials() (email string, password string, err error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter Email: ")
	email, err = reader.ReadString('\n')
	if err != nil {
		return "", "", err
	}
	email = strings.TrimSuffix(email, "\n")

	fmt.Print("Enter Password: ")
	bytePassword, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		return "", "", err
	}

	password = string(bytePassword)
	return email, password, nil
}

func getMFACode() (mfaCode string, err error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter MFA code: ")
	mfaCode, err = reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	mfaCode = strings.TrimSuffix(mfaCode, "\n")

	return mfaCode, nil
}

func LoginMaybeMFA(email, password, channel string) (accessToken string, refreshToken string, err error) {
	c := client.NanitClient{}
	req := &client.AuthRequestPayload{
		Email:    email,
		Password: password,
		Channel:  channel,
	}
	accessToken, refreshToken, err = c.Login(req)
	return accessToken, refreshToken, err
}

func LoginMFA(email, password, mfaToken, mfaCode string) (accessToken string, refreshToken string, err error) {
	c := client.NanitClient{}
	req := &client.AuthRequestPayload{
		Email:    email,
		Password: password,
		MFAToken: mfaToken,
		MFACode:  mfaCode,
	}
	accessToken, refreshToken, err = c.Login(req)
	return accessToken, refreshToken, err
}
