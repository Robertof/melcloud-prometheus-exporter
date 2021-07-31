package melcloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	loginUrl 	  = "https://app.melcloud.com/Mitsubishi.Wifi.Client/Login/ClientLogin"
	deviceInfoUrl = "https://app.melcloud.com/Mitsubishi.Wifi.Client/Device/Get"
)

type MelcloudRequestor struct {
	client *http.Client
	contextKey string
	reauthenticate func() (string, error)
}

func Authenticate(Email, Password string) (*MelcloudRequestor, error) {
	client := http.DefaultClient

	log.Info().Msg("Authenticating with MELCloud...")

	request := loginRequest{
		AppVersion:      "1.21.6.0",
		CaptchaResponse: nil,
		Email:           Email,
		Password:        Password,
		Language:        19,
		Persist:         true,
	}

	serializedRequest, _ := json.Marshal(request)
	rawResponse, err := client.Post(loginUrl, "application/json", bytes.NewBuffer(serializedRequest))

	if err != nil {
		return nil, fmt.Errorf("Unable to authenticate with MELCloud: %w", err)
	}

	defer rawResponse.Body.Close()

	log.Trace().
		Int("statusCode", rawResponse.StatusCode).
		Func(func(e *zerolog.Event) {
			res, _ := httputil.DumpResponse(rawResponse, true)
			e.Str("response", string(res))
		}).
		Msg("Received MELCloud login response")

	response := loginResponse{}
	if err = json.NewDecoder(rawResponse.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("Unable to decode MELCloud login response: %w", err)
	}

	if response.ErrorId != nil {
		return nil, fmt.Errorf("Unable to sign in to MELCloud, maybe your credentials are incorrect? (err: %v)", *response.ErrorId)
	}

	log.Info().Msg("Successfully authenticated with MELCloud")

	return &MelcloudRequestor{
		client:     client,
		contextKey: response.LoginData.ContextKey,
		reauthenticate: func() (string, error) {
			result, err := Authenticate(Email, Password)
			if err != nil {
				return "", fmt.Errorf("Reauthentication failed: %w", err)
			}
			return result.contextKey, nil
		},
	}, nil
}

func (r *MelcloudRequestor) GetDeviceInformation(DeviceId, BuildingId string) (io.ReadCloser, error) {
	url, err := url.Parse(deviceInfoUrl)
	if err != nil {
		panic(err)
	}

	q := url.Query()
	q.Set("id", DeviceId)
	q.Set("buildingID", BuildingId)
	url.RawQuery = q.Encode()

	log.Debug().
		Str("url", url.String()).
		Str("deviceID", DeviceId).
		Str("buildingID", BuildingId).
		Msg("Requesting device info from MELCloud")

	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to get device info from MELCloud: %w", err)
	}

	res, err := r.makeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("Unable to get device info from MELCloud: %w", err)
	}

	log.Trace().
		Int("statusCode", res.StatusCode).
		Func(func(e *zerolog.Event) {
			res, _ := httputil.DumpResponse(res, true)
			e.Str("response", string(res))
		}).
		Msg("Received MELCloud device information response")

	return res.Body, nil
}

func (r *MelcloudRequestor) makeRequest(req *http.Request) (*http.Response, error) {
	req.Header.Add("X-MitsContextKey", r.contextKey)
	req.Header.Add("Accept", "application/json")

	res, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("MELCloud request failed: %w", err)
	}

	if res.StatusCode == http.StatusUnauthorized {
		res.Body.Close()
		log.Warn().Msg("Performing MELCloud reauthentication")
		// Try to reauthenticate...
		contextKey, err := r.reauthenticate()
		if err != nil {
			return nil, err
		}
		r.contextKey = contextKey
		return r.makeRequest(req)
	}

	return res, nil
}
