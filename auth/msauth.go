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

// StartDeviceAuth starts the Microsoft device code flow for the provided clientID, scope and tenant.
// It returns an access token string once the user completes the flow.
func StartDeviceAuth(clientID string, scope string, tenant string) (string, error) {
	if tenant == "" {
		tenant = "consumers"
	}

	deviceEndpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/devicecode", tenant)
	tokenEndpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenant)

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("scope", scope)

	resp, err := http.Post(deviceEndpoint, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("device code request failed: %s", string(body))
	}

	var dc DeviceCodeResponse
	err = json.Unmarshal(body, &dc)
	if err != nil {
		return "", err
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
			return "", errors.New("device code expired before verification")
		}

		post := url.Values{}
		post.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		post.Set("client_id", clientID)
		post.Set("device_code", dc.DeviceCode)

		resp, err := http.Post(tokenEndpoint, "application/x-www-form-urlencoded", strings.NewReader(post.Encode()))
		if err != nil {
			return "", err
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if resp.StatusCode == 200 {
			var tr TokenResponse
			err := json.Unmarshal(body, &tr)
			if err != nil {
				return "", err
			}

			return tr.AccessToken, nil
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
				return "", errors.New("authorization declined")
			}

			if errStr == "expired_token" {
				return "", errors.New("device code expired")
			}
		}

		return "", fmt.Errorf("token request failed: %s", string(body))
	}
}

// loadTokenFromFile helper: load token from file if present
func loadTokenFromFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	t := strings.TrimSpace(string(b))
	if t == "" {
		return "", nil
	}
	return t, nil
}

// saveTokenToFile helper: save token to file
func saveTokenToFile(path string, token string) error {
	if path == "" {
		return nil
	}
	// ensure directory exists
	if dir := filepathDir(path); dir != "" {
		_ = os.MkdirAll(dir, 0700)
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(token)+"\n"), 0600)
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
		return "", "", fmt.Errorf("xsts msauth failed: %s", string(body))
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
// It can use either device code flow or browser-based msauth code flow.
// authMode should be "device" or "browser".
// tokenFile is optional - if provided, the token will be cached to/from this file.
func GetMinecraftToken(clientID, scope, tenant, authMode, tokenFile string) (mcToken, profileID, profileName string, err error) {
	if tenant == "" {
		tenant = "consumers"
	}
	if scope == "" {
		scope = "XboxLive.signin offline_access"
	}
	if authMode == "" {
		authMode = "device"
	}

	// attempt to load token from file if configured
	if tokenFile != "" {
		if t, err := loadTokenFromFile(tokenFile); err == nil && t != "" {
			log.Println("Loaded Minecraft access token from file")
			// Verify it works by fetching profile
			id, name, err := fetchMinecraftProfile(t)
			if err == nil && id != "" && name != "" {
				return t, id, name, nil
			}
			log.Println("Cached token is invalid, will re-authenticate")
		}
	}

	log.Println("Starting Microsoft device msauth...")
	msToken, err := StartDeviceAuth(clientID, scope, tenant)
	if err != nil {
		return "", "", "", err
	}

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

	// persist token if file configured
	if tokenFile != "" {
		if err := saveTokenToFile(tokenFile, mcToken); err != nil {
			log.Printf("warning: failed to save token to file: %v", err)
		} else {
			log.Println("Saved Minecraft access token to file")
		}
	}

	log.Println("Minecraft access token obtained")

	profileID, profileName, err = fetchMinecraftProfile(mcToken)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch Minecraft profile: %w", err)
	}

	log.Printf("Retrieved Minecraft profile - Name: %q (len=%d), ID: %s", profileName, len(profileName), profileID)

	return mcToken, profileID, profileName, nil
}
