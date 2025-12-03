package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	authData   *AuthData
}

func (c *Client) setCookies() error {
	baseURL, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}

	var cookies []*http.Cookie
	for name, value := range c.authData.Cookies {
		cookies = append(cookies, &http.Cookie{
			Name:  name,
			Value: value,
			Path:  "/",
		})
	}

	c.httpClient.Jar.SetCookies(baseURL, cookies)
	return nil
}

func (c *Client) CallAPI(method, endpoint string, body interface{}) ([]byte, error) {
	url := c.baseURL + endpoint

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("atermes_ea_api_key", c.apiKey)
	}
	if c.authData.JWT != "" {
		req.Header.Set("Authorization", "Bearer "+c.authData.JWT)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// Helper methods for common API calls
func (c *Client) GetClientOverview(take, skip int) ([]byte, error) {
	body := map[string]interface{}{
		"take": take,
		"skip": skip,
	}
	return c.CallAPI("POST", "/api/v1/Client/overzicht", body)
}

func (c *Client) GetDossierOverview(take, skip int) ([]byte, error) {
	body := map[string]interface{}{
		"take": take,
		"skip": skip,
	}
	return c.CallAPI("POST", "/api/v1/Dossier/overzicht", body)
}

func (c *Client) GetClient(clientID string) ([]byte, error) {
	return c.CallAPI("GET", fmt.Sprintf("/api/v1/Client/%s", clientID), nil)
}
