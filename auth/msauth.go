package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
	Message                 string `json:"message"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

type TokenCache struct {
	MinecraftAccessToken  string    `json:"minecraft_access_token"`
	MicrosoftRefreshToken string    `json:"microsoft_refresh_token"`
	ExpiresAt             time.Time `json:"expires_at"`
	ProfileID             string    `json:"profile_id"`
	ProfileName           string    `json:"profile_name"`
}

func StartDeviceAuth(clientID string) (string, string, error) {
	deviceEndpoint := "https://login.microsoftonline.com/consumers/oauth2/v2.0/devicecode"
	tokenEndpoint := "https://login.microsoftonline.com/consumers/oauth2/v2.0/token"

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("scope", "XboxLive.signin offline_access")

	resp, err := http.Post(deviceEndpoint, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return "", "", err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("device code request failed: %s", string(body))
	}

	var dc DeviceCodeResponse
	err = json.Unmarshal(body, &dc)
	if err != nil {
		return "", "", err
	}

	log.Println("To authenticate with Microsoft, follow these steps:")
	if dc.VerificationURIComplete != "" {
		log.Printf("Open: %s", dc.VerificationURIComplete)
	} else {
		log.Printf("Open: %s and enter code: %s", dc.VerificationURI, dc.UserCode)
	}

	log.Printf("%s", dc.Message)

	interval := time.Duration(dc.Interval) * time.Second
	if interval == 0 {
		interval = 5 * time.Second
	}

	expiresAt := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

	for {
		if time.Now().After(expiresAt) {
			return "", "", errors.New("device code expired before verification")
		}

		post := url.Values{}
		post.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		post.Set("client_id", clientID)
		post.Set("device_code", dc.DeviceCode)

		resp, err := http.Post(tokenEndpoint, "application/x-www-form-urlencoded", strings.NewReader(post.Encode()))
		if err != nil {
			return "", "", err
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode == 200 {
			var tr TokenResponse
			err := json.Unmarshal(body, &tr)
			if err != nil {
				return "", "", err
			}

			return tr.AccessToken, tr.RefreshToken, nil
		}

		var errObj map[string]any
		_ = json.Unmarshal(body, &errObj)
		errStr, ok := errObj["error"].(string)
		if ok {
			if errStr == "authorization_pending" {
				time.Sleep(interval)
				continue
			}

			if errStr == "authorization_declined" {
				return "", "", errors.New("authorization declined")
			}

			if errStr == "expired_token" {
				return "", "", errors.New("device code expired")
			}
		}

		return "", "", fmt.Errorf("token request failed: %s", string(body))
	}
}

func loadTokenCache(path string) (*TokenCache, error) {
	if path == "" {
		return nil, errors.New("empty path")
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cache TokenCache
	err = json.Unmarshal(b, &cache)

	return &cache, err
}

func saveTokenCache(path string, cache *TokenCache) error {
	if path == "" {
		return nil
	}

	dir := filepathDir(path)
	if dir != "" {
		_ = os.MkdirAll(dir, 0700)
	}

	b, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, b, 0600)
}

func filepathDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				return "/"
			}

			return path[:i]
		}
	}

	return ""
}

func refreshMicrosoftToken(clientID, refreshToken string) (string, string, error) {
	tokenEndpoint := "https://login.microsoftonline.com/consumers/oauth2/v2.0/token"

	post := url.Values{}
	post.Set("grant_type", "refresh_token")
	post.Set("client_id", clientID)
	post.Set("refresh_token", refreshToken)
	post.Set("scope", "XboxLive.signin offline_access")

	resp, err := http.Post(tokenEndpoint, "application/x-www-form-urlencoded", strings.NewReader(post.Encode()))
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("refresh token request failed: %s", string(body))
	}

	var tr TokenResponse
	err = json.Unmarshal(body, &tr)
	if err != nil {
		return "", "", err
	}

	return tr.AccessToken, tr.RefreshToken, nil
}

func xboxAuthenticate(msAccessToken string) (string, string, error) {
	requestBody, _ := json.Marshal(map[string]any{
		"Properties": map[string]string{
			"AuthMethod": "RPS",
			"SiteName":   "user.auth.xboxlive.com",
			"RpsTicket":  "d=" + msAccessToken,
		},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	})

	resp, err := http.Post("https://user.auth.xboxlive.com/user/authenticate", "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return "", "", err
	}

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("xbox auth failed: %s", string(body))
	}

	var responseBody map[string]any
	err = json.Unmarshal(body, &responseBody)
	if err != nil {
		return "", "", err
	}

	token, ok := responseBody["Token"].(string)
	if !ok || token == "" {
		return "", "", errors.New("no Token in xbox auth response")
	}

	displayClaims, ok := responseBody["DisplayClaims"].(map[string]any)
	if ok {
		xui, ok := displayClaims["xui"].([]any)
		if ok && len(xui) > 0 {
			first, ok := xui[0].(map[string]any)
			if ok {
				uhs, ok := first["uhs"].(string)
				if ok {
					return token, uhs, nil
				}
			}
		}
	}

	return token, "", nil
}

func xstsAuthorize(xblToken string) (string, string, error) {
	reqBody, _ := json.Marshal(map[string]any{
		"Properties": map[string]any{
			"SandboxId":  "RETAIL",
			"UserTokens": []string{xblToken},
		},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	})

	resp, err := http.Post("https://xsts.auth.xboxlive.com/xsts/authorize", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return "", "", err
	}

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("xsts auth failed: %s", string(body))
	}

	var responseObject map[string]any

	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return "", "", err
	}

	token, ok := responseObject["Token"].(string)
	if !ok || token == "" {
		return "", "", errors.New("no Token in xsts response")
	}

	displayClaims, ok := responseObject["DisplayClaims"].(map[string]any)
	if ok {
		xui, ok := displayClaims["xui"].([]any)
		if ok && len(xui) > 0 {
			first, ok := xui[0].(map[string]any)
			if ok {
				uhs, ok := first["uhs"].(string)
				if ok {
					return token, uhs, nil
				}
			}
		}
	}

	return token, "", nil
}

func minecraftLogin(xstsToken string, uhs string) (string, error) {
	requestBody, _ := json.Marshal(map[string]string{
		"identityToken": "XBL3.0 x=" + uhs + ";" + xstsToken,
	})

	resp, err := http.Post("https://api.minecraftservices.com/authentication/login_with_xbox", "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return "", err
	}

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("minecraft login failed: %s", string(body))
	}

	var responseObject map[string]any
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return "", err
	}

	token, ok := responseObject["access_token"].(string)
	if ok {
		return token, nil
	}

	return "", errors.New("no access_token in minecraft login response")
}

func checkEntitlements(mcToken string) (bool, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.minecraftservices.com/entitlements/mcstore", nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Authorization", "Bearer "+mcToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("entitlements check failed: %s", string(body))
	}

	var responseObject map[string]any
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return false, err
	}

	items, ok := responseObject["items"].([]any)
	if ok {
		return len(items) > 0, nil
	}

	return false, nil
}

func fetchMinecraftProfile(mcToken string) (string, string, error) {
	if mcToken == "" {
		return "", "", errors.New("empty minecraft token")
	}

	req, err := http.NewRequest(http.MethodGet, "https://api.minecraftservices.com/minecraft/profile", nil)
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Authorization", "Bearer "+mcToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("failed to fetch profile: %s", string(body))
	}

	var responseObject map[string]any
	err = json.Unmarshal(body, &responseObject)
	if err != nil {
		return "", "", err
	}

	id, _ := responseObject["id"].(string)
	name, _ := responseObject["name"].(string)
	if id == "" || name == "" {
		return "", "", errors.New("profile missing id or name")
	}

	return id, name, nil
}

func GetMinecraftToken(clientID, tokenFile string) (mcToken, profileID, profileName string, err error) {
	if tokenFile != "" {
		cache, err := loadTokenCache(tokenFile)
		if err == nil && cache != nil {
			log.Println("Loaded token cache from file")

			if cache.MinecraftAccessToken != "" && time.Now().Before(cache.ExpiresAt) {
				log.Println("Cached Minecraft token is still valid")

				id, name, err := fetchMinecraftProfile(cache.MinecraftAccessToken)
				if err == nil && id != "" && name != "" {
					log.Println("Using cached Minecraft access token")

					return cache.MinecraftAccessToken, id, name, nil
				}

				log.Println("Cached Minecraft token validation failed")
			}

			if cache.MicrosoftRefreshToken != "" {
				log.Println("Minecraft token expired, attempting refresh using Microsoft refresh token...")
				msToken, newRefreshToken, err := refreshMicrosoftToken(clientID, cache.MicrosoftRefreshToken)
				if err == nil {
					log.Println("Successfully refreshed Microsoft access token")

					return completeMicrosoftAuth(msToken, newRefreshToken, tokenFile)
				}

				log.Printf("Failed to refresh token: %v, will re-authenticate", err)
			}
		} else {
			log.Println("No valid token cache found, will authenticate")
		}
	}

	log.Println("Starting Microsoft device auth...")

	msToken, msRefreshToken, err := StartDeviceAuth(clientID)
	if err != nil {
		return "", "", "", err
	}

	return completeMicrosoftAuth(msToken, msRefreshToken, tokenFile)
}

func completeMicrosoftAuth(msToken, msRefreshToken, tokenFile string) (mcToken, profileID, profileName string, err error) {
	log.Println("Microsoft access token obtained, exchanging for Xbox Live token...")

	xblToken, uhs, err := xboxAuthenticate(msToken)
	if err != nil {
		return "", "", "", fmt.Errorf("xbox authenticate failed: %w", err)
	}

	log.Println("Xbox Live token obtained, exchanging for XSTS token...")

	xstsToken, uhs2, err := xstsAuthorize(xblToken)
	if err != nil {
		return "", "", "", fmt.Errorf("xsts authorize failed: %w", err)
	}
	if uhs == "" && uhs2 != "" {
		uhs = uhs2
	}

	log.Println("XSTS token obtained, logging into Minecraft services...")

	mcToken, err = minecraftLogin(xstsToken, uhs)
	if err != nil {
		return "", "", "", fmt.Errorf("minecraft login failed: %w", err)
	}

	owns, err := checkEntitlements(mcToken)
	if err != nil {
		log.Printf("warning: failed to check entitlements: %v", err)
	} else if !owns {
		log.Println("warning: account does not appear to own Minecraft (entitlements empty)")
	}

	log.Println("Minecraft access token obtained")

	profileID, profileName, err = fetchMinecraftProfile(mcToken)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch Minecraft profile: %w", err)
	}

	log.Printf("Retrieved Minecraft profile - Name: %q (len=%d), ID: %s", profileName, len(profileName), profileID)

	if tokenFile != "" {
		cache := &TokenCache{
			MinecraftAccessToken:  mcToken,
			MicrosoftRefreshToken: msRefreshToken,
			ExpiresAt:             time.Now().Add(6 * time.Hour),
			ProfileID:             profileID,
			ProfileName:           profileName,
		}
		err = saveTokenCache(tokenFile, cache)
		if err != nil {
			log.Printf("warning: failed to save token cache to file: %v", err)
		} else {
			log.Println("Saved token cache to file")
		}
	}

	return mcToken, profileID, profileName, nil
}
