package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

var myClient = &http.Client{Timeout: 10 * time.Second}

// ------------------------------------------

type authResponsePayload struct {
	AccessToken string `json:"access_token"`
}

type Baby struct {
	UID       string `json:"uid"`
	Name      string `json:"name"`
	CameraUID string `json:"camera_uid"`
}

type babiesResponsePayload struct {
	Babies []Baby `json:"babies"`
}

// ------------------------------------------

type NanitClient struct {
	Email        string
	Password     string
	SessionStore *AppSessionStore
}

func anonymizeToken(token string, clearLen int) string {
	if clearLen != 0 && (len(token)-2*clearLen) > 6 {
		runes := []rune(token)
		return string(runes[0:clearLen]) + strings.Repeat("*", len(token)-2*clearLen) + string(runes[len(token)-clearLen:])
	}

	return strings.Repeat("*", len(token))
}

func (c *NanitClient) EnsureToken() string {
	if c.SessionStore.Session.AuthToken == "" {
		c.Authorize()
	}

	return c.SessionStore.Session.AuthToken
}

func (c *NanitClient) Authorize() {
	log.Info().Str("email", c.Email).Str("password", anonymizeToken(c.Password, 0)).Msg("Authorizing using user credentials")

	requestBody, requestBodyErr := json.Marshal(map[string]string{
		"email":    c.Email,
		"password": c.Password,
	})

	if requestBodyErr != nil {
		log.Fatal().Err(requestBodyErr).Msg("Unable to marshal auth body")
	}

	r, clientErr := myClient.Post("https://api.nanit.com/login", "application/json", bytes.NewBuffer(requestBody))
	if clientErr != nil {
		log.Fatal().Err(clientErr).Msg("Unable to fetch auth token")
	}

	defer r.Body.Close()

	if r.StatusCode == 401 {
		log.Fatal().Msg("Server responded with code 401. Provided credentials has not been accepted by the server. Please check if your e-mail address and password is entered correctly and that 2FA is disabled on your account.")
	} else if r.StatusCode != 201 {
		log.Fatal().Int("code", r.StatusCode).Msg("Server responded with unexpected status code")
	}

	authResponse := new(authResponsePayload)

	jsonErr := json.NewDecoder(r.Body).Decode(authResponse)
	if jsonErr != nil {
		log.Fatal().Err(jsonErr).Msg("Unable to decode response")
	}

	log.Info().Str("token", anonymizeToken(authResponse.AccessToken, 4)).Msg("Authorized")
	c.SessionStore.Session.AuthToken = authResponse.AccessToken
	c.SessionStore.Session.AuthTime = time.Now()
	c.SessionStore.Save()
}

func (c *NanitClient) FetchAuthorized(req *http.Request, data interface{}) {
	for i := 0; i < 2; i++ {
		if c.SessionStore.Session.AuthToken != "" {
			req.Header.Set("Authorization", c.SessionStore.Session.AuthToken)

			res, clientErr := myClient.Do(req)
			if clientErr != nil {
				log.Fatal().Err(clientErr).Msg("HTTP request failed")
			}

			defer res.Body.Close()

			if res.StatusCode != 401 {
				if res.StatusCode != 200 {
					log.Fatal().Int("code", res.StatusCode).Msg("Server responded with unexpected status code")
				}

				jsonErr := json.NewDecoder(res.Body).Decode(data)
				if jsonErr != nil {
					log.Fatal().Err(jsonErr).Msg("Unable to decode response")
				}

				return
			}

			log.Info().Msg("Token might be expired. Will try to re-authenticate.")
		}

		c.Authorize()
	}

	log.Fatal().Msg("Unable to make request due failed authorization (2 attempts).")
}

func (c *NanitClient) FetchBabies() []Baby {
	log.Info().Msg("Fetching babies list")
	req, reqErr := http.NewRequest("GET", "https://api.nanit.com/babies", nil)

	if reqErr != nil {
		log.Fatal().Err(reqErr).Msg("Unable to create request")
	}

	data := new(babiesResponsePayload)
	c.FetchAuthorized(req, data)

	c.SessionStore.Session.Babies = data.Babies
	c.SessionStore.Save()
	return data.Babies
}

func (c *NanitClient) EnsureBabies() []Baby {
	if len(c.SessionStore.Session.Babies) == 0 {
		return c.FetchBabies()
	}

	return c.SessionStore.Session.Babies
}
