package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"

	"github.com/Parallels/pd-api-service/basecontext"
	"github.com/Parallels/pd-api-service/models"
)

const (
	DEFAULT_API_LOGIN_URL = "api/v1/auth/token"
)

type HttpClientService struct {
	context       context.Context
	ctx           basecontext.ApiContext
	headers       map[string]string
	authorization *HttpClientServiceAuthorization
	authorizer    *HttpClientServiceAuthorizer
}

type HttpClientServiceResponse struct {
	StatusCode int
	Data       interface{}
	ApiError   *models.ApiErrorResponse
}

func NewHttpClient(ctx basecontext.ApiContext) *HttpClientService {
	return &HttpClientService{
		ctx:           ctx,
		headers:       make(map[string]string, 0),
		authorizer:    nil,
		authorization: nil,
	}
}

func (c *HttpClientService) WithContext(ctx context.Context) *HttpClientService {
	c.context = ctx
	return c
}

func (c *HttpClientService) WithHeader(key, value string) *HttpClientService {
	c.headers[key] = value
	return c
}

func (c *HttpClientService) WithHeaders(headers map[string]string) *HttpClientService {
	for k, v := range headers {
		c.headers[k] = v
	}
	return c
}

func (c *HttpClientService) AuthorizeWithUsernameAndPassword(username, password string) *HttpClientService {
	c.authorization = &HttpClientServiceAuthorization{
		Username: username,
		Password: password,
	}

	return c
}

func (c *HttpClientService) AuthorizeWithApiKey(apiKey string) *HttpClientService {
	c.authorization = &HttpClientServiceAuthorization{
		ApiKey: apiKey,
	}

	return c
}

func (c *HttpClientService) SetAuthorization(authorization HttpClientServiceAuthorization) *HttpClientService {
	c.authorization = &authorization
	return c
}

func (c *HttpClientService) Get(url string, destination interface{}) (*HttpClientServiceResponse, error) {
	return c.RequestData(HttpClientServiceVerbGet, url, nil, destination)
}

func (c *HttpClientService) Post(url string, data interface{}, destination interface{}) (*HttpClientServiceResponse, error) {
	return c.RequestData(HttpClientServiceVerbPost, url, data, destination)
}

func (c *HttpClientService) Put(url string, data interface{}, destination interface{}) (*HttpClientServiceResponse, error) {
	return c.RequestData(HttpClientServiceVerbPut, url, data, destination)
}

func (c *HttpClientService) Delete(url string, destination interface{}) (*HttpClientServiceResponse, error) {
	return c.RequestData(HttpClientServiceVerbDelete, url, nil, destination)
}

func (c *HttpClientService) RequestData(verb HttpClientServiceVerb, url string, data interface{}, destination interface{}) (*HttpClientServiceResponse, error) {
	c.ctx.LogInfo("[Api Client] %v data from %s", verb, url)
	var err error
	apiResponse := HttpClientServiceResponse{
		StatusCode: 0,
		Data:       nil,
	}

	if destination != nil {
		var destType = reflect.TypeOf(destination)
		if destType.Kind() != reflect.Ptr {
			return &apiResponse, errors.New("dest must be a pointer type")
		}
	}

	if url == "" {
		return &apiResponse, errors.New("url cannot be empty")
	}

	client := http.DefaultClient
	var req *http.Request

	if data != nil {
		reqBody, err := json.MarshalIndent(data, "", "  ")
		c.ctx.LogInfo("[Api Client] Request body: \n%s", string(reqBody))
		if err != nil {
			return &apiResponse, fmt.Errorf("error marshalling data, err: %v", err)
		}
		if c.context != nil {
			req, err = http.NewRequestWithContext(c.context, verb.String(), url, bytes.NewBuffer(reqBody))
			if err != nil {
				return &apiResponse, fmt.Errorf("error creating request, err: %v", err)
			}
		} else {
			req, err = http.NewRequest(verb.String(), url, bytes.NewBuffer(reqBody))
			if err != nil {
				return &apiResponse, fmt.Errorf("error creating request, err: %v", err)
			}
		}
	} else {
		if c.context != nil {
			req, err = http.NewRequestWithContext(c.context, verb.String(), url, nil)
			if err != nil {
				return &apiResponse, fmt.Errorf("error creating request, err: %v", err)
			}
		} else {
			req, err = http.NewRequest(verb.String(), url, nil)
			if err != nil {
				return &apiResponse, fmt.Errorf("error creating request, err: %v", err)
			}
		}
	}

	if req == nil {
		return &apiResponse, fmt.Errorf("request is nil")
	}

	if c.authorization != nil {
		c.authorizer = nil
		if c.authorization.ApiKey != "" {
			c.authorizer = &HttpClientServiceAuthorizer{
				ApiKey: c.authorization.ApiKey,
			}
		}
		if c.authorization.Username != "" && c.authorization.Password != "" {
			c.ctx.LogInfo("[Api Client] Getting Client Authorization with username %s ", c.authorization.Username)
			token, err := getJwtToken(c.ctx, url, c.authorization.Username, c.authorization.Password)
			if err != nil {
				apiResponse.StatusCode = 401
				return &apiResponse, err
			}
			c.authorizer = &HttpClientServiceAuthorizer{
				BearerToken: token,
			}
		}
	}

	if c.authorizer != nil {
		if c.authorizer.BearerToken != "" {
			c.ctx.LogDebug("[Api Client] Setting Authorization header to Bearer %s", c.authorizer.BearerToken)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.authorizer.BearerToken))
		} else if c.authorizer.ApiKey != "" {
			c.ctx.LogDebug("[Api Client] Setting Authorization header to X-Api-Key %s", c.authorizer.ApiKey)
			req.Header.Set("X-Api-Key", c.authorizer.ApiKey)
		}
	}

	if req.Header.Get("Content-Type") == "" && data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.headers != nil && len(c.headers) > 0 {
		for k, v := range c.headers {
			req.Header.Set(k, v)
		}
	}

	response, err := client.Do(req)
	if err != nil {
		return &apiResponse, fmt.Errorf("error %s data on %s, err: %v", verb, url, err)
	}

	apiResponse.StatusCode = response.StatusCode
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var errMsg models.ApiErrorResponse
		body, bodyErr := io.ReadAll(response.Body)
		if bodyErr == nil {
			if err := json.Unmarshal(body, &errMsg); err == nil {
				apiResponse.ApiError = &errMsg
			}
		}

		if apiResponse.ApiError != nil && apiResponse.ApiError.Message != "" {
			return &apiResponse, fmt.Errorf("error on %s data from %s, err: %v message: %v", verb, url, apiResponse.ApiError.Code, apiResponse.ApiError.Message)
		} else {
			return &apiResponse, fmt.Errorf("error on %s data from %s, status code: %d", verb, url, response.StatusCode)
		}
	}

	if response.Body != nil {
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return &apiResponse, fmt.Errorf("error reading response body from %s, err: %v", url, err)
		}
		if destination != nil {

			err = json.Unmarshal(body, destination)
			if err != nil {
				return &apiResponse, fmt.Errorf("error unmarshalling body from %s, err: %v ", url, err)
			}

			c.ctx.LogDebug("[Api Client] Response body: \n%s", string(body))
			apiResponse.Data = destination
		} else {
			var bodyData map[string]interface{}

			err = json.Unmarshal(body, &bodyData)
			if err != nil {
				return &apiResponse, fmt.Errorf("error unmarshalling body from %s, err: %v ", url, err)
			}

			apiResponse.Data = bodyData
		}
	}

	return &apiResponse, nil
}

func (c *HttpClientService) GetFileFromUrl(fileUrl string, destinationPath string) error {
	// Create the file in the tmp folder
	file, err := os.Create(destinationPath)
	if err != nil {
		return err
	}

	defer file.Close()

	// Download the file from the URL
	resp, err := http.Get(fileUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the file to disk
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func getJwtToken(ctx basecontext.ApiContext, baseUrl, username, password string) (string, error) {
	if username == "" {
		return "", errors.New("username cannot be empty")
	}

	if password == "" {
		return "", errors.New("password cannot be empty")
	}

	tokenRequest := models.LoginRequest{
		Email:    username,
		Password: password,
	}

	h, err := url.Parse(baseUrl)
	if err != nil {
		return "", err
	}

	hostAndPath := fmt.Sprintf("%s://%s/%s", h.Scheme, h.Host, DEFAULT_API_LOGIN_URL)

	c := NewHttpClient(ctx)
	c.ctx.LogDebug("[Api Client] Getting token from %s with username and password", hostAndPath, username, password)

	var tokenResponse models.LoginResponse
	if _, err := c.Post(hostAndPath, tokenRequest, &tokenResponse); err != nil {
		return "", err
	}
	return tokenResponse.Token, nil
}
