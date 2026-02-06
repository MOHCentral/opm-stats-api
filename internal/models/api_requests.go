package models

import (
	"encoding/json"
	"fmt"
)

// FlexString unmarshals from both JSON string and number values into a Go string.
// Game scripts may send player_guid as int (0) or string ("0").
type FlexString string

func (f *FlexString) UnmarshalJSON(data []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexString(s)
		return nil
	}
	// Try number (int or float)
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*f = FlexString(n.String())
		return nil
	}
	return fmt.Errorf("FlexString: cannot unmarshal %s", string(data))
}

func (f FlexString) String() string {
	return string(f)
}

type RegisterServerRequest struct {
	Name      string     `json:"name"`
	IPAddress string     `json:"ip_address"`
	Port      FlexString `json:"port"`
}

type RegisterServerResponse struct {
	ServerID string `json:"server_id"`
	Token    string `json:"token"`
}

type DeviceAuthRequest struct {
	ForumUserID int    `json:"forum_user_id"`
	Regenerate  bool   `json:"regenerate"`
	ClientIP    string `json:"client_ip"`
}

type DeviceAuthResponse struct {
	UserCode  string `json:"user_code"`
	ExpiresIn int    `json:"expires_in"`
	ExpiresAt string `json:"expires_at"` // ISO8601
	IsNew     bool   `json:"is_new"`
}

type DevicePollRequest struct {
	DeviceCode string `json:"device_code"`
}

type VerifyTokenRequest struct {
	Token         string     `json:"token"`
	PlayerGUID    FlexString `json:"player_guid"`
	ServerName    string     `json:"server_name"`
	ServerAddress string     `json:"server_address"`
	PlayerIP      string     `json:"player_ip"`
	ServerID      string     `json:"server_id"`
	PlayerName    string     `json:"player_name"`
	ServerIP      string     `json:"server_ip"`
	ServerPort    FlexString `json:"server_port"`
}

type ResolveIPRequest struct {
	ForumUserID int    `json:"forum_user_id"`
	Action      string `json:"action"` // "approve" or "deny"
	Label       string `json:"label"`  // Optional label
}

type MarkNotifiedRequest struct {
	ForumUserID int      `json:"forum_user_id"`
	IDs         []string `json:"ids"`
}
