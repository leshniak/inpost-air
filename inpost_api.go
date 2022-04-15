package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type InPostAPI struct {
	configFilePath string
	authToken      string
	refreshToken   string
	client         *http.Client
	baseURL        *url.URL
}

type APIError struct {
	Status      int
	Error       string
	Description string
}

type Pollutant struct {
	Value   float32
	Percent float32
}

type Point struct {
	Name          string
	AirSensor     bool
	AirSensorData struct {
		AirQuality string
		Weather    struct {
			Temperature float32
			Pressure    float32
			Humidity    float32
		}
		Pollutants struct {
			Pm10 Pollutant
			Pm25 Pollutant
		}
		UpdatedUntil time.Time
	}
}

type Config struct {
	RefreshToken string `json:"refreshToken"`
	AuthToken    string `json:"authToken"`
}

func NewInPostAPI(configFilePath string) *InPostAPI {
	inpost := new(InPostAPI)
	inpost.client = &http.Client{Timeout: 10 * time.Second}
	inpost.baseURL = &url.URL{
		Scheme: "https",
		Host:   "api-inmobile-pl.easypack24.net",
	}
	inpost.configFilePath = configFilePath
	inpost.ReadConfig()

	return inpost
}

func (inpost *InPostAPI) GetPoint(pointId string) (*Point, error) {
	if !inpost.isAuthTokenValid() && inpost.refreshToken != "" {
		err := inpost.Authenticate()
		if err != nil {
			return nil, err
		}
	}

	endpoint := url.URL{Path: fmt.Sprintf("/v2/points/%s", pointId)}
	response, body := inpost.request("GET", &endpoint, nil)

	if response.StatusCode != http.StatusOK {
		apiErr := APIError{}
		json.Unmarshal(body, &apiErr)

		if apiErr.Error == "tokenExpiredException" {
			err := inpost.Authenticate()
			if err != nil {
				return nil, err
			}

			return inpost.GetPoint(pointId)
		}

		return nil, errors.New(fmt.Sprintf("[%d] %s", response.StatusCode, apiErr.Error))
	}

	data := new(Point)
	json.Unmarshal(body, data)

	return data, nil
}

func (inpost *InPostAPI) Authenticate() error {
	if inpost.refreshToken == "" {
		return errors.New("Please log in again (--login).")
	}

	type RequestPayload struct {
		RefreshToken string `json:"refreshToken"`
		PhoneOS      string `json:"phoneOS"`
	}

	type Response struct {
		AuthToken string
	}

	endpoint := url.URL{Path: "/v1/authenticate"}
	payload := RequestPayload{inpost.refreshToken, "Apple"}
	jsonData, _ := json.Marshal(payload)
	response, body := inpost.request("POST", &endpoint, bytes.NewBuffer(jsonData))

	if response.StatusCode != http.StatusOK {
		apiErr := APIError{}
		json.Unmarshal(body, &apiErr)

		return errors.New(fmt.Sprintf("[%d] %s", response.StatusCode, apiErr.Error))
	}

	data := Response{}
	json.Unmarshal(body, &data)
	inpost.authToken = data.AuthToken
	inpost.SaveConfig()

	return nil
}

func (inpost *InPostAPI) SendSMSCode(phoneNumber string) error {
	type RequestPayload struct {
		PhoneNumber string `json:"phoneNumber"`
	}

	endpoint := url.URL{Path: "/v1/sendSMSCode"}
	payload := RequestPayload{phoneNumber}
	jsonData, _ := json.Marshal(payload)
	response, body := inpost.request("POST", &endpoint, bytes.NewBuffer(jsonData))

	if response.StatusCode != http.StatusOK {
		apiErr := APIError{}
		json.Unmarshal(body, &apiErr)

		return errors.New(fmt.Sprintf("[%d] %s", response.StatusCode, apiErr.Error))
	}

	return nil
}

func (inpost *InPostAPI) ConfirmSMSCode(phoneNumber string, smsCode string) error {
	type RequestPayload struct {
		PhoneNumber string `json:"phoneNumber"`
		SmsCode     string `json:"smsCode"`
		PhoneOS     string `json:"phoneOS"`
	}

	type Response struct {
		RefreshToken string
		AuthToken    string
	}

	endpoint := url.URL{Path: "/v1/confirmSMSCode"}
	payload := RequestPayload{phoneNumber, smsCode, "Apple"}
	jsonData, _ := json.Marshal(payload)
	response, body := inpost.request("POST", &endpoint, bytes.NewBuffer(jsonData))

	if response.StatusCode != http.StatusOK {
		apiErr := APIError{}
		json.Unmarshal(body, &apiErr)

		return errors.New(fmt.Sprintf("[%d] %s", response.StatusCode, apiErr.Error))
	}

	data := Response{}
	json.Unmarshal(body, &data)
	inpost.refreshToken = data.RefreshToken
	inpost.authToken = data.AuthToken
	inpost.SaveConfig()

	return nil
}

func (inpost *InPostAPI) SaveConfig() {
	text, _ := json.MarshalIndent(Config{inpost.refreshToken, inpost.authToken}, "", "  ")
	err := ioutil.WriteFile(inpost.configFilePath, text, 0644)
	if err != nil {
		log.Fatalf("Couldn't save config file: %+v", err)
	}
}

func (inpost *InPostAPI) ReadConfig() {
	text, _ := ioutil.ReadFile(inpost.configFilePath)
	config := Config{}
	json.Unmarshal(text, &config)
	inpost.refreshToken = config.RefreshToken
	inpost.authToken = config.AuthToken
}

func (inpost *InPostAPI) request(method string, apiURL *url.URL, requestBody io.Reader) (*http.Response, []byte) {
	resolvedURL := inpost.baseURL.ResolveReference(apiURL)
	req, err := http.NewRequest(method, resolvedURL.String(), requestBody)
	if err != nil {
		log.Fatalf("Error occurred: %+v", err)
	}

	req.Header.Set("User-Agent", "InPost-Mobile/3.7.2-release (iOS 15.1.1; iPhone14,2; pl)")
	req.Header.Add("Accept-Language", "en-US")

	if method == "POST" || method == "PUT" {
		req.Header.Add("Content-Type", "application/json")
	}

	if inpost.authToken != "" {
		req.Header.Add("Authorization", inpost.authToken)
	}

	response, err := inpost.client.Do(req)
	if err != nil {
		log.Fatalf("Error sending request to API endpoint: %+v", err)
	}

	defer response.Body.Close()

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("Couldn't parse response body: %+v", err)
	}

	return response, responseBody
}

func (inpost *InPostAPI) isAuthTokenValid() bool {
	type TokenPayload struct {
		Exp int64
	}

	if !strings.HasPrefix(inpost.authToken, "Bearer ") {
		return false
	}

	jwt := strings.Replace(inpost.authToken, "Bearer ", "", 1)
	encodedTokenPayload := strings.Split(jwt, ".")[1]
	jsonData, _ := base64.RawStdEncoding.DecodeString(encodedTokenPayload)
	decodedTokenPayload := TokenPayload{}
	err := json.Unmarshal(jsonData, &decodedTokenPayload)
	if err != nil {
		log.Fatalf("Couldn't validate auth token.")
	}

	tokenExpirationDate := time.Unix(decodedTokenPayload.Exp, 0)

	return time.Now().Before(tokenExpirationDate)
}
