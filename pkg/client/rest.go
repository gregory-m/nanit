package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/rs/zerolog/log"
	"gitlab.com/adam.stanek/nanit/pkg/baby"
	"gitlab.com/adam.stanek/nanit/pkg/message"
	"gitlab.com/adam.stanek/nanit/pkg/session"
	"gitlab.com/adam.stanek/nanit/pkg/utils"
)

var myClient = &http.Client{Timeout: 10 * time.Second}

// ------------------------------------------

type authResponsePayload struct {
	AccessToken string `json:"access_token"`
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
	Email        string
	Password     string
	SessionStore *session.Store
}

// MaybeAuthorize - Performs authorizaiton if we don't have token or we assume it is expired
func (c *NanitClient) MaybeAuthorize(force bool) {
	if force || c.SessionStore.Session.AuthToken == "" || time.Since(c.SessionStore.Session.AuthTime) > AuthTokenTimelife {
		c.Authorize()
	}
}

// Authorize - performs authorization attempt, panics if it fails
func (c *NanitClient) Authorize() {
	log.Info().Str("email", c.Email).Str("password", utils.AnonymizeToken(c.Password, 0)).Msg("Authorizing using user credentials")

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

	log.Info().Str("token", utils.AnonymizeToken(authResponse.AccessToken, 4)).Msg("Authorized")
	c.SessionStore.Session.AuthToken = authResponse.AccessToken
	c.SessionStore.Session.AuthTime = time.Now()
	c.SessionStore.Save()
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
