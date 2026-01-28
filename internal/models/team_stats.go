package models

// FactionStats comparison
type FactionStats struct {
	Axis   TeamMetrics `json:"axis"`
	Allies TeamMetrics `json:"allies"`
}

type TeamMetrics struct {
	Kills          uint64  `json:"kills"`
	Deaths         uint64  `json:"deaths"`
	Wins           uint64  `json:"wins"`
	Losses         uint64  `json:"losses"`
	KDRatio        float64 `json:"kd_ratio"`
	WinRate        float64 `json:"win_rate"`
	ObjectivesDone uint64  `json:"objectives_done"`
	TopWeapon      string  `json:"top_weapon"`
}
