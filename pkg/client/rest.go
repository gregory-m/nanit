package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/gregory-m/nanit/pkg/baby"
	"github.com/gregory-m/nanit/pkg/message"
	"github.com/gregory-m/nanit/pkg/session"
	"github.com/gregory-m/nanit/pkg/utils"
)

var myClient = &http.Client{Timeout: 10 * time.Second}
var ErrExpiredRefreshToken = errors.New("Refresh token has expired. Relogin required.")

type MFARequiredError struct {
	MFAToken string
}

func (e *MFARequiredError) Error() string {
	return "MFA authentication enabled for user account"
}

// ------------------------------------------

type AuthRequestPayload struct {
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	Channel  string `json:"channel,omitempty"`
	MFAToken string `json:"mfa_token,omitempty"`
	MFACode  string `json:"mfa_code,omitempty"`
}

type authResponsePayload struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"` // We can store this to renew a session, avoiding the need to re-auth with MFA
}

type authMFAEnabledResponsePayload struct {
	MFAToken    string `json:"mfa_token,omitempty"`
	PhoneSuffix string `json:"phone_suffix,omitempty"`
	Channel     string `json:"channel,omitempty"`
}

type babiesResponsePayload struct {
	Babies []baby.Baby `json:"babies"`
}

type messagesResponsePayload struct {
	Messages []message.Message `json:"messages"`
}

// ------------------------------------------

// NanitClient - client context
type NanitClient struct {
	RefreshToken string
	SessionStore *session.Store
}

// MaybeAuthorize - Performs authorization if we don't have token or we assume it is expired
func (c *NanitClient) MaybeAuthorize(force bool) {
	if force || c.SessionStore.Session.AuthToken == "" || time.Since(c.SessionStore.Session.AuthTime) > AuthTokenTimelife {
		c.Authorize()
	}
}

// Authorize - performs authorization attempt, panics if it fails
func (c *NanitClient) Authorize() {
	if len(c.SessionStore.Session.RefreshToken) == 0 {
		c.SessionStore.Session.RefreshToken = c.RefreshToken
	}

	if len(c.SessionStore.Session.RefreshToken) > 0 {
		err := c.RenewSession() // We have a refresh token, so we'll use that to extend our session
		if err != nil {
			log.Fatal().Err(err).Msg("Error occurred while trying to refresh the session")
		}
	}
}

// Renews an existing session using a valid refresh token
// If the refresh token has also expired, we need to perform a full re-login
func (c *NanitClient) RenewSession() error {
	log.Debug().Str("refresh_token", utils.AnonymizeToken(c.SessionStore.Session.RefreshToken, 4)).Msg("Renewing Session")
	requestBody, requestBodyErr := json.Marshal(map[string]string{
		"refresh_token": c.SessionStore.Session.RefreshToken,
	})

	if requestBodyErr != nil {
		log.Fatal().Err(requestBodyErr).Msg("Unable to marshal auth body")
	}

	r, clientErr := myClient.Post("https://api.nanit.com/tokens/refresh", "application/json", bytes.NewBuffer(requestBody))
	if clientErr != nil {
		log.Fatal().Err(clientErr).Msg("Unable to renew session")
	}

	defer r.Body.Close()
	if r.StatusCode == 404 {
		log.Warn().Msg("Server responded with code 404. This typically means your refresh token has expired.")
		return ErrExpiredRefreshToken
	} else if r.StatusCode > 299 || r.StatusCode < 200 {
		log.Fatal().Int("code", r.StatusCode).Msg("Server responded with an error")
	}

	authResponse := new(authResponsePayload)

	jsonErr := json.NewDecoder(r.Body).Decode(authResponse)
	if jsonErr != nil {
		log.Fatal().Err(jsonErr).Msg("Unable to decode response")
	}

	log.Info().Str("token", utils.AnonymizeToken(authResponse.AccessToken, 4)).Msg("Authorized")
	log.Info().Str("refresh_token", utils.AnonymizeToken(authResponse.RefreshToken, 4)).Msg("Retreived")
	c.SessionStore.Session.AuthToken = authResponse.AccessToken
	c.SessionStore.Session.RefreshToken = authResponse.RefreshToken
	c.SessionStore.Session.AuthTime = time.Now()
	c.SessionStore.Save()

	return nil
}

// Login - performs login with MFA support
func (c *NanitClient) Login(authReq *AuthRequestPayload) (accessToken string, refreshToken string, err error) {
	log.Debug().
		Str("email", authReq.Email).
		Str("password", utils.AnonymizeToken(authReq.Password, 0)).
		Str("channel", authReq.Channel).
		Str("mfa_token", utils.AnonymizeToken(authReq.MFAToken, 4)).
		Str("mfa_code", utils.AnonymizeToken(authReq.MFAToken, 1)).
		Msg("Authorizing")

	requestBody, err := json.Marshal(authReq)
	if err != nil {
		return "", "", fmt.Errorf("unable to marshal auth body: %q", err)
	}

	req, err := http.NewRequest("POST", "https://api.nanit.com/login", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", "", fmt.Errorf("unable to create request: %q", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("nanit-api-version", "2")
	r, err := myClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("unable to fetch auth token: %q", err)
	}

	defer r.Body.Close()

	if r.StatusCode == 401 {
		return "", "", fmt.Errorf("server responded with code 401, provided credentials has not been accepted by the server")
	} else if r.StatusCode == 482 {
		mfaResponse := new(authMFAEnabledResponsePayload)

		err = json.NewDecoder(r.Body).Decode(mfaResponse)
		if err != nil {
			return "", "", fmt.Errorf("unable to decode MFA response: %q", err)
		}

		log.Debug().Str("mfa_token", utils.AnonymizeToken(mfaResponse.MFAToken, 4)).Msg("MFA Required")
		return "", "", &MFARequiredError{MFAToken: mfaResponse.MFAToken}
	} else if r.StatusCode != 201 {
		return "", "", fmt.Errorf("server responded with unexpected status code: %d", r.StatusCode)
	}

	authResponse := new(authResponsePayload)

	jsonErr := json.NewDecoder(r.Body).Decode(authResponse)
	if jsonErr != nil {
		return "", "", fmt.Errorf("unable to decode auth response: %q", err)
	}

	log.Debug().Str("access_token", utils.AnonymizeToken(authResponse.AccessToken, 4)).
		Str("refresh_token", utils.AnonymizeToken(authResponse.RefreshToken, 4)).
		Msg("Authorized")

	return authResponse.AccessToken, authResponse.RefreshToken, nil
}

// FetchAuthorized - makes authorized http request
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

// FetchBabies - fetches baby list
func (c *NanitClient) FetchBabies() []baby.Baby {
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

// FetchMessages - fetches message list
func (c *NanitClient) FetchMessages(babyUID string, limit int) []message.Message {
	req, reqErr := http.NewRequest("GET", fmt.Sprintf("https://api.nanit.com/babies/%s/messages?limit=%d", babyUID, limit), nil)

	if reqErr != nil {
		log.Fatal().Err(reqErr).Msg("Unable to create request")
	}

	data := new(messagesResponsePayload)
	c.FetchAuthorized(req, data)

	return data.Messages
}

// EnsureBabies - fetches baby list if not fetched already
func (c *NanitClient) EnsureBabies() []baby.Baby {
	if len(c.SessionStore.Session.Babies) == 0 {
		return c.FetchBabies()
	}

	return c.SessionStore.Session.Babies
}

// FetchNewMessages - fetches 10 newest messages, ignores any messages which were already fetched or which are older than 5 minutes
func (c *NanitClient) FetchNewMessages(babyUID string, defaultMessageTimeout time.Duration) []message.Message {
	fetchedMessages := c.FetchMessages(babyUID, 10)
	newMessages := make([]message.Message, 0)

	// return empty [] if there are no fetchedMessages
	if len(fetchedMessages) == 0 {
		log.Debug().Msg("No messages fetched")
		return newMessages
	}

	// sort fetechedMessages starting with most recent
	sort.Slice(fetchedMessages, func(i, j int) bool {
		return fetchedMessages[i].Time.Time().After(fetchedMessages[j].Time.Time())
	})

	lastSeenMessageTime := c.SessionStore.Session.LastSeenMessageTime
	messageTimeoutTime := lastSeenMessageTime
	log.Debug().Msgf("Last seen message time was %s", lastSeenMessageTime)

	// Don't know when last message was, set messageTimeout to default
	if lastSeenMessageTime.IsZero() {
		messageTimeoutTime = time.Now().UTC().Add(-defaultMessageTimeout)
	}

	// lastSeenMessageTime is older than most recent fetchedMessage, or is unset
	if lastSeenMessageTime.Before(fetchedMessages[0].Time.Time()) {
		lastSeenMessageTime = fetchedMessages[0].Time.Time()
		c.SessionStore.Session.LastSeenMessageTime = lastSeenMessageTime
		c.SessionStore.Save()
	}

	// Only keep messages that are more recent than messageTimeoutTime
	filteredMessages := message.FilterMessages(fetchedMessages, func(message message.Message) bool {
		return message.Time.Time().After(messageTimeoutTime)
	})

	log.Debug().Msgf("Found %d new messages", len(filteredMessages))
	log.Debug().Msgf("%+v\n", filteredMessages)

	return filteredMessages
}
