package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var myClient = &http.Client{Timeout: 10 * time.Second}

// ------------------------------------------

type authResponsePayload struct {
	AccessToken string `json:"access_token"`
}

type babyInfo struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
}

type babiesResponsePayload struct {
	Babies []babyInfo `json:"babies"`
}

// ------------------------------------------

type NanitClient struct {
	Email       string
	Password    string
	AccessToken string
}

func anonymizeToken(token string, clearLen int) string {
	if clearLen != 0 && (len(token)-2*clearLen) > 6 {
		runes := []rune(token)
		return string(runes[0:clearLen]) + strings.Repeat("*", len(token)-2*clearLen) + string(runes[len(token)-clearLen:])
	}

	return strings.Repeat("*", len(token))
}

func (c *NanitClient) Authorize() {
	log.Println(fmt.Sprintf("Authorizing. E-mail = %v, Password = %v", c.Email, anonymizeToken(c.Password, 0)))

	requestBody, requestBodyErr := json.Marshal(map[string]string{
		"email":    c.Email,
		"password": c.Password,
	})

	if requestBodyErr != nil {
		log.Fatal(fmt.Errorf("Unable to marshal auth body: %v", requestBodyErr))
	}

	r, clientErr := myClient.Post("https://api.nanit.com/login", "application/json", bytes.NewBuffer(requestBody))
	if clientErr != nil {
		log.Fatal(fmt.Errorf("Unable to fetch auth token: %v", clientErr))
	}

	defer r.Body.Close()

	if r.StatusCode != 201 {
		log.Fatal(fmt.Sprintf("Server responded with unexpected status code: %v", r.StatusCode))
	}

	authResponse := new(authResponsePayload)

	jsonErr := json.NewDecoder(r.Body).Decode(authResponse)
	if jsonErr != nil {
		log.Fatal(fmt.Errorf("Unable to decode response: %v", jsonErr))
	}

	log.Println(fmt.Sprintf("Authorized. Token = %v", anonymizeToken(authResponse.AccessToken, 4)))
	c.AccessToken = authResponse.AccessToken

}

func (c *NanitClient) FetchAuthorized(req *http.Request, data interface{}) {
	for i := 0; i < 2; i++ {
		if c.AccessToken != "" {
			req.Header.Set("Authorization", c.AccessToken)

			res, clientErr := myClient.Do(req)
			if clientErr != nil {
				log.Fatal(fmt.Errorf("HTTP request failed: %v", clientErr))
			}

			defer res.Body.Close()

			if res.StatusCode != 401 {
				if res.StatusCode != 200 {
					log.Fatal(fmt.Sprintf("Server responded with unexpected status code: %v", res.StatusCode))
				}

				jsonErr := json.NewDecoder(res.Body).Decode(data)
				if jsonErr != nil {
					log.Fatal(fmt.Errorf("Unable to decode response: %v", jsonErr))
				}

				return
			}

			log.Println("Token might be expired. Will try to re-authenticate.")
		}

		c.Authorize()
	}

	log.Fatal("Unable to make request due failed authorization (2 attempts).")
}

func (c *NanitClient) FetchBabies() []babyInfo {
	log.Println("Fetching babies list")
	req, reqErr := http.NewRequest("GET", "https://api.nanit.com/babies", nil)

	if reqErr != nil {
		log.Fatal(fmt.Errorf("Unable to create request: %v", reqErr))
	}

	data := new(babiesResponsePayload)
	c.FetchAuthorized(req, data)

	return data.Babies
}

func (c *NanitClient) GetStreamURL() string {
	babies := c.FetchBabies()
	if len(babies) < 1 {
		log.Fatal("No baby found")
	}

	baby := babies[0]
	baseURL := "rtmps://media-secured.nanit.com/nanit"
	token := fmt.Sprintf("%v.%v", baby.UID, c.AccessToken)
	log.Println(fmt.Sprintf("Stream URL: %v/%v", baseURL, anonymizeToken(token, 4)))

	return fmt.Sprintf("%v/%v", baseURL, token)
}

func startStream(url string, dataDir string) {
	args := []string{
		"-i",
		url,
		"-vcodec",
		"copy",
		"-acodec",
		"copy",
		"-hls_init_time",
		"0",
		"-hls_time",
		"1",
		"-hls_list_size",
		"6",
		"-hls_wrap",
		"10",
		"-start_number",
		"1",
		fmt.Sprintf("%v/stream.m3u8", dataDir),
	}

	log.Println(fmt.Sprintf("Starting FFMPEG with args: %v", strings.Join(args, " ")))

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err := cmd.Start()
	if err != nil {
		log.Fatal(fmt.Errorf("Run failed: %v", err))
	}
}

func main() {
	dataDir := "data"
	staticDir := "static"

	api := NanitClient{
		Email:    os.Getenv("EMAIL"),
		Password: os.Getenv("PASSWORD"),
	}

	startStream(api.GetStreamURL(), dataDir)

	http.Handle("/", http.FileServer(http.Dir(staticDir)))
	http.Handle("/stream/", http.StripPrefix("/stream/", http.FileServer(http.Dir(dataDir))))
	http.ListenAndServe(":8080", nil)
}
