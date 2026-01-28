package models

type RegisterServerRequest struct {
	Name      string `json:"name"`
	IPAddress string `json:"ip_address"`
	Port      int    `json:"port"`
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
	Token         string `json:"token"`
	PlayerGUID    string `json:"player_guid"`
	ServerName    string `json:"server_name"`
	ServerAddress string `json:"server_address"`
	PlayerIP      string `json:"player_ip"`
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
