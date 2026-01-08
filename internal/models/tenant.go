package models

import (
	"encoding/json"
	"fmt"
)

type TenantServiceKey struct {
	OAuth TenantOAuth `json:"oauth"`
}

type TenantOAuth struct {
	CreateDate   string `json:"createdate"`
	ClientID     string `json:"clientid"`
	ClientSecret string `json:"clientsecret"`
	TokenURL     string `json:"tokenurl"`
	URL          string `json:"url"`
}

func (t TenantServiceKey) PrettyJSON() ([]byte, error) {
	return json.MarshalIndent(t, "", "  ")
}

func (t TenantServiceKey) ValidateRequired() error {
	if t.OAuth.URL == "" {
		return fmt.Errorf("tenant service key missing required field: oauth.url")
	}
	if t.OAuth.TokenURL == "" {
		return fmt.Errorf("tenant service key missing required field: oauth.tokenurl")
	}
	if t.OAuth.ClientID == "" {
		return fmt.Errorf("tenant service key missing required field: oauth.clientid")
	}
	if t.OAuth.ClientSecret == "" {
		return fmt.Errorf("tenant service key missing required field: oauth.clientsecret")
	}
	if t.OAuth.CreateDate == "" {
		return fmt.Errorf("tenant service key missing required field: oauth.createdate")
	}
	return nil
}
