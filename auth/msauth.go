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

// DeviceCodeResponse is the response from the device code endpoint.
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
	Message                 string `json:"message"`
}

// TokenResponse contains a simple token response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}

// TokenCache stores both Minecraft access token and Microsoft refresh token
type TokenCache struct {
	MinecraftAccessToken  string    `json:"minecraft_access_token"`
	MicrosoftRefreshToken string    `json:"microsoft_refresh_token"`
	ExpiresAt             time.Time `json:"expires_at"`
	ProfileID             string    `json:"profile_id"`
	ProfileName           string    `json:"profile_name"`
}

// StartDeviceAuth starts the Microsoft device code flow for the provided clientID, using the
// default consumer tenant and a fixed Xbox Live scope.
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

		var errObj map[string]interface{}
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

// loadTokenCache loads the token cache from file
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
	if err != nil {
		return nil, err
	}

	return &cache, nil
}

// saveTokenCache saves the token cache to file
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

// small helper for directory extraction (avoid importing path/filepath multiple times)
func filepathDir(path string) string {
	// mimic filepath.Dir but avoid import if not strictly necessary
	// use os package to check for last slash
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

// refreshMicrosoftToken uses a refresh token to get a new Microsoft access token
func refreshMicrosoftToken(clientID, refreshToken string) (string, string, error) {
	tokenEndpoint := fmt.Sprintf("https://login.microsoftonline.com/consumers/oauth2/v2.0/token")

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

// xboxAuthenticate exchanges a Microsoft access token for an Xbox Live token and returns the XBL token and user hash (uhs).
func xboxAuthenticate(msAccessToken string) (string, string, error) {
	endpoint := "https://user.auth.xboxlive.com/user/authenticate"
	reqBody := map[string]interface{}{
		"Properties": map[string]string{
			"AuthMethod": "RPS",
			"SiteName":   "user.auth.xboxlive.com",
			"RpsTicket":  "d=" + msAccessToken,
		},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}
	b, _ := json.Marshal(reqBody)
	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("xbox auth failed: %s", string(body))
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return "", "", err
	}
	token, ok := obj["Token"].(string)
	if !ok || token == "" {
		return "", "", errors.New("no Token in xbox auth response")
	}
	// extract uhs
	if dc, ok := obj["DisplayClaims"].(map[string]interface{}); ok {
		if xui, ok := dc["xui"].([]interface{}); ok && len(xui) > 0 {
			if first, ok := xui[0].(map[string]interface{}); ok {
				if uhs, ok := first["uhs"].(string); ok {
					return token, uhs, nil
				}
			}
		}
	}
	return token, "", nil
}

// xstsAuthorize exchanges an XBL token for an XSTS token. It returns the XSTS token and user hash.
func xstsAuthorize(xblToken string) (string, string, error) {
	endpoint := "https://xsts.auth.xboxlive.com/xsts/authorize"
	reqBody := map[string]interface{}{
		"Properties": map[string]interface{}{
			"SandboxId":  "RETAIL",
			"UserTokens": []string{xblToken},
		},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	}
	b, _ := json.Marshal(reqBody)
	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("xsts auth failed: %s", string(body))
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return "", "", err
	}
	token, ok := obj["Token"].(string)
	if !ok || token == "" {
		return "", "", errors.New("no Token in xsts response")
	}
	// extract uhs
	if dc, ok := obj["DisplayClaims"].(map[string]interface{}); ok {
		if xui, ok := dc["xui"].([]interface{}); ok && len(xui) > 0 {
			if first, ok := xui[0].(map[string]interface{}); ok {
				if uhs, ok := first["uhs"].(string); ok {
					return token, uhs, nil
				}
			}
		}
	}
	return token, "", nil
}

// minecraftLogin uses XSTS token and user hash to obtain a Minecraft access token.
func minecraftLogin(xstsToken string, uhs string) (string, error) {
	endpoint := "https://api.minecraftservices.com/authentication/login_with_xbox"
	identity := "XBL3.0 x=" + uhs + ";" + xstsToken
	reqBody := map[string]string{
		"identityToken": identity,
	}
	b, _ := json.Marshal(reqBody)
	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("minecraft login failed: %s", string(body))
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return "", err
	}
	if token, ok := obj["access_token"].(string); ok {
		return token, nil
	}
	return "", errors.New("no access_token in minecraft login response")
}

// checkEntitlements verifies whether the Minecraft token has the 'minecraft' entitlement (owns game).
func checkEntitlements(mcToken string) (bool, error) {
	endpoint := "https://api.minecraftservices.com/entitlements/mcstore"
	req, err := http.NewRequest("GET", endpoint, nil)
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
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return false, err
	}
	if items, ok := obj["items"].([]interface{}); ok {
		return len(items) > 0, nil
	}
	return false, nil
}

// fetchMinecraftProfile retrieves the Minecraft profile (id and name) for the provided Minecraft access token.
func fetchMinecraftProfile(mcToken string) (string, string, error) {
	if mcToken == "" {
		return "", "", errors.New("empty minecraft token")
	}
	endpoint := "https://api.minecraftservices.com/minecraft/profile"
	req, err := http.NewRequest("GET", endpoint, nil)
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
	var obj map[string]interface{}
	if err := json.Unmarshal(body, &obj); err != nil {
		return "", "", err
	}
	id, _ := obj["id"].(string)
	name, _ := obj["name"].(string)
	if id == "" || name == "" {
		return "", "", errors.New("profile missing id or name")
	}
	return id, name, nil
}

// GetMinecraftToken performs the full OAuth flow to get a Minecraft access token.
// tokenFile is optional - if provided, the token will be cached to/from this file.
func GetMinecraftToken(clientID, tokenFile string) (mcToken, profileID, profileName string, err error) {
	// attempt to load token cache from file if configured
	if tokenFile != "" {
		cache, err := loadTokenCache(tokenFile)
		if err == nil && cache != nil {
			log.Println("Loaded token cache from file")

			// Check if Minecraft token is still valid
			if cache.MinecraftAccessToken != "" && time.Now().Before(cache.ExpiresAt) {
				log.Println("Cached Minecraft token is still valid")
				// Verify it works by fetching profile
				id, name, err := fetchMinecraftProfile(cache.MinecraftAccessToken)
				if err == nil && id != "" && name != "" {
					log.Println("Using cached Minecraft access token")
					return cache.MinecraftAccessToken, id, name, nil
				}
				log.Println("Cached Minecraft token validation failed")
			}

			// Token expired or invalid, try to refresh using Microsoft refresh token
			if cache.MicrosoftRefreshToken != "" {
				log.Println("Minecraft token expired, attempting refresh using Microsoft refresh token...")
				msToken, newRefreshToken, err := refreshMicrosoftToken(clientID, cache.MicrosoftRefreshToken)
				if err == nil {
					log.Println("Successfully refreshed Microsoft access token")
					// Continue with the refreshed token to get new Minecraft token
					return completeMicrosoftAuth(msToken, newRefreshToken, tokenFile)
				}
				log.Printf("Failed to refresh token: %v, will re-authenticate", err)
			}
		} else {
			log.Println("No valid token cache found, will authenticate")
		}
	}

	// No cached token or refresh failed, perform full OAuth flow
	log.Println("Starting Microsoft device auth...")
	msToken, msRefreshToken, err := StartDeviceAuth(clientID)
	if err != nil {
		return "", "", "", err
	}

	return completeMicrosoftAuth(msToken, msRefreshToken, tokenFile)
}

// completeMicrosoftAuth completes the authentication flow from Microsoft token to Minecraft token
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

	// Optionally verify entitlement (owning the game)
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

	// persist token cache if file configured
	if tokenFile != "" {
		cache := &TokenCache{
			MinecraftAccessToken:  mcToken,
			MicrosoftRefreshToken: msRefreshToken,
			ExpiresAt:             time.Now().Add(6 * time.Hour),
			ProfileID:             profileID,
			ProfileName:           profileName,
		}
		if err := saveTokenCache(tokenFile, cache); err != nil {
			log.Printf("warning: failed to save token cache to file: %v", err)
		} else {
			log.Println("Saved token cache to file")
		}
	}

	return mcToken, profileID, profileName, nil
}
