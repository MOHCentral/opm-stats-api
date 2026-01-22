package main

import (
	"bytes"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

const (
	APIURL        = "http://localhost:8080/api/v1/ingest/events"
	ServerToken   = "test-token"
	PostgresURL   = "postgres://mohaa:admin123@localhost:5432/mohaa_stats?sslmode=disable"
	ClickHouseURL = "http://localhost:8123"
	MySQLDSN      = "smf:smf_password@tcp(localhost:3306)/smf?charset=utf8mb4"
)

var Concurrency = 2

// Event types covering all 92 events
const (
	// Game Flow (11)
	EventGameInit          = "game_init"
	EventGameStart         = "game_start"
	EventGameEnd           = "game_end"
	EventMatchStart        = "match_start"
	EventMatchEnd          = "match_end"
	EventRoundStart        = "round_start"
	EventRoundEnd          = "round_end"
	EventWarmupStart       = "warmup_start"
	EventWarmupEnd         = "warmup_end"
	EventIntermissionStart = "intermission_start"
	EventTeamWin           = "team_win"

	// Combat (23)
	EventKill              = "kill"
	EventDeath             = "death"
	EventDamage            = "damage"
	EventPlayerPain        = "player_pain"
	EventHeadshot          = "headshot"
	EventPlayerSuicide     = "player_suicide"
	EventPlayerCrushed     = "player_crushed"
	EventPlayerTelefragged = "player_telefragged"
	EventPlayerRoadkill    = "player_roadkill"
	EventPlayerBash        = "player_bash"
	EventPlayerTeamkill    = "player_teamkill"
	EventWeaponFire        = "weapon_fire"
	EventWeaponHit         = "weapon_hit"
	EventWeaponChange      = "weapon_change"
	EventWeaponReload      = "weapon_reload"
	EventWeaponReloadDone  = "weapon_reload_done"
	EventWeaponReady       = "weapon_ready"
	EventWeaponNoAmmo      = "weapon_no_ammo"
	EventWeaponHolster     = "weapon_holster"
	EventWeaponRaise       = "weapon_raise"
	EventWeaponDrop        = "weapon_drop"
	EventGrenadeThrow      = "grenade_throw"
	EventGrenadeExplode    = "grenade_explode"

	// Movement (10)
	EventJump           = "jump"
	EventLand           = "land"
	EventCrouch         = "crouch"
	EventProne          = "prone"
	EventPlayerStand    = "player_stand"
	EventPlayerSpawn    = "player_spawn"
	EventPlayerRespawn  = "player_respawn"
	EventDistance       = "distance"
	EventLadderMount    = "ladder_mount"
	EventLadderDismount = "ladder_dismount"

	// Interaction (6)
	EventPlayerUse             = "player_use"
	EventPlayerUseObjectStart  = "player_use_object_start"
	EventPlayerUseObjectFinish = "player_use_object_finish"
	EventPlayerSpectate        = "player_spectate"
	EventPlayerFreeze          = "player_freeze"
	EventPlayerSay             = "player_say"

	// Item (5)
	EventItemPickup   = "item_pickup"
	EventItemDrop     = "item_drop"
	EventItemRespawn  = "item_respawn"
	EventHealthPickup = "health_pickup"
	EventAmmoPickup   = "ammo_pickup"

	// Vehicle/Turret (6)
	EventVehicleEnter     = "vehicle_enter"
	EventVehicleExit      = "vehicle_exit"
	EventVehicleDeath     = "vehicle_death"
	EventVehicleCollision = "vehicle_collision"
	EventTurretEnter      = "turret_enter"
	EventTurretExit       = "turret_exit"

	// Server (5)
	EventServerInit           = "server_init"
	EventServerStart          = "server_start"
	EventServerShutdown       = "server_shutdown"
	EventServerSpawned        = "server_spawned"
	EventServerConsoleCommand = "server_console_command"
	EventHeartbeat            = "heartbeat"

	// Map (4)
	EventMapLoadStart   = "map_load_start"
	EventMapLoadEnd     = "map_load_end"
	EventMapChangeStart = "map_change_start"
	EventMapRestart     = "map_restart"

	// Team/Vote (5)
	EventTeamJoin   = "team_join"
	EventTeamChange = "team_change"
	EventVoteStart  = "vote_start"
	EventVotePassed = "vote_passed"
	EventVoteFailed = "vote_failed"

	// Client (5)
	EventClientConnect         = "client_connect"
	EventClientDisconnect      = "client_disconnect"
	EventClientBegin           = "client_begin"
	EventClientUserinfoChanged = "client_userinfo_changed"
	EventPlayerInactivityDrop  = "player_inactivity_drop"

	// World (3)
	EventDoorOpen  = "door_open"
	EventDoorClose = "door_close"
	EventExplosion = "explosion"

	// AI/Actor/Bot (7)
	EventActorSpawn  = "actor_spawn"
	EventActorKilled = "actor_killed"
	EventBotSpawn    = "bot_spawn"
	EventBotKilled   = "bot_killed"
	EventBotRoam     = "bot_roam"
	EventBotCurious  = "bot_curious"
	EventBotAttack   = "bot_attack"

	// Objectives (2)
	EventObjectiveUpdate  = "objective_update"
	EventObjectiveCapture = "objective_capture"

	// Score/Admin (2)
	EventScoreChange  = "score_change"
	EventTeamkillKick = "teamkill_kick"

	// Legacy
	EventConnect    = "connect"
	EventDisconnect = "disconnect"
)

// Player represents a game player with SMF link
type Player struct {
	Name      string
	GUID      string
	Team      string
	Skill     float32
	Style     string
	Favorite  string
	SMFUserID int
	Token     string
	Weapon    string
	Ammo      int
	HP        int
	Alive     bool
	Stance    string
	OnTurret  bool
	InVehicle bool
}

var weapons = []string{
	"Thompson", "MP40", "M1 Garand", "Kar98k", "BAR", "STG44",
	"Springfield", "Kar98k Sniper", "Bazooka", "Panzerschreck",
	"Colt .45", "Walther P38", "Grenade", "Stielhandgranate", "Shotgun", "Fists",
}

var hitlocs = []string{
	"head", "neck", "torso_upper", "torso_lower",
	"left_arm_upper", "left_arm_lower", "right_arm_upper", "right_arm_lower",
	"left_leg_upper", "left_leg_lower", "right_leg_upper", "right_leg_lower",
}

var maps = []string{
	"dm/mohdm1", "dm/mohdm2", "dm/mohdm3", "dm/mohdm4", "dm/mohdm5", "dm/mohdm6", "dm/mohdm7",
	"obj/obj_team1", "obj/obj_team2", "obj/obj_team3", "obj/obj_team4",
	"lib/stalingrad", "lib/hunt", "lib/flughafen",
	"tdm/v2rocket", "tdm/destroyed_village", "tdm/bridge",
}

var gametypes = []string{"dm", "tdm", "obj", "lib", "ctf"}

var servers = []struct {
	ID   string
	Name string
}{
	{"srv-us-east-1", "US East #1"},
	{"srv-us-west-1", "US West #1"},
	{"srv-eu-central-1", "EU Central #1"},
	{"srv-eu-west-1", "EU West #1"},
	{"srv-asia-1", "Asia Pacific #1"},
}

var adjectives = []string{"Silent", "Swift", "Dark", "Iron", "Shadow", "Steel", "Ghost", "Thunder", "Storm", "Frost", "Fire", "Deadly", "Wild", "Savage", "Grim", "Crazy", "Sneaky", "Fast", "Brave", "Bold"}
var nouns = []string{"Wolf", "Eagle", "Hawk", "Sniper", "Soldier", "Warrior", "Hunter", "Ranger", "Scout", "Trooper", "Veteran", "Killer", "Slayer", "Striker", "Gunner", "Runner", "Fighter", "Shooter", "Blade", "Knight"}

type Stats struct {
	eventsSent   int64
	eventsErrors int64
	matchesRun   int64
	usersCreated int64
	startTime    time.Time
}

func (s *Stats) AddEvents(n int) { atomic.AddInt64(&s.eventsSent, int64(n)) }
func (s *Stats) AddError()       { atomic.AddInt64(&s.eventsErrors, 1) }
func (s *Stats) AddMatch()       { atomic.AddInt64(&s.matchesRun, 1) }
func (s *Stats) AddUser()        { atomic.AddInt64(&s.usersCreated, 1) }

func (s *Stats) Print() {
	elapsed := time.Since(s.startTime).Seconds()
	events := atomic.LoadInt64(&s.eventsSent)
	errors := atomic.LoadInt64(&s.eventsErrors)
	matches := atomic.LoadInt64(&s.matchesRun)
	users := atomic.LoadInt64(&s.usersCreated)
	fmt.Printf("\n=== Seeding Complete ===\n")
	fmt.Printf("SMF Users:    %d\n", users)
	fmt.Printf("Matches:      %d\n", matches)
	fmt.Printf("Events sent:  %d\n", events)
	fmt.Printf("Errors:       %d\n", errors)
	fmt.Printf("Duration:     %.2fs\n", elapsed)
	if elapsed > 0 {
		fmt.Printf("Rate:         %.0f events/sec\n", float64(events)/elapsed)
	}
}

func generatePlayerName(i int) string {
	adj := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]
	return fmt.Sprintf("%s%s%d", adj, noun, i)
}

func hashSMFPassword(username, password string) string {
	h := sha1.New()
	h.Write([]byte(strings.ToLower(username) + password))
	return hex.EncodeToString(h.Sum(nil))
}

func main() {
	rand.Seed(time.Now().UnixNano())

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "setup":
		numUsers := 1000
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &numUsers)
		}
		setupUsers(numUsers)
	case "seed":
		numMatches := 500
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &numMatches)
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%d", &Concurrency)
		}
		fmt.Printf("Seeding %d matches with %d workers...\n", numMatches, Concurrency)
		seedEvents(numMatches)
	case "full":
		numUsers := 1000
		numMatches := 500
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &numUsers)
		}
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%d", &numMatches)
		}
		if len(os.Args) > 4 {
			fmt.Sscanf(os.Args[4], "%d", &Concurrency
			fmt.Sscanf(os.Args[3], "%d", &numMatches)
		}
		players := setupUsers(numUsers)
		if len(players) > 0 {
			seedEventsWithPlayers(numMatches, players)
		}
	case "seed-player":
		if len(os.Args) < 3 {
			fmt.Println("Usage: statscli seed-player <username|token> [token|matches] [matches]")
			return
		}
		
		arg2 := ""
		if len(os.Args) >= 4 {
			arg2 = os.Args[3]
		}
		
		arg3 := 0
		if len(os.Args) >= 5 {
			fmt.Sscanf(os.Args[4], "%d", &arg3)
		}
		
		seedPlayer(os.Args[2], arg2, arg3)
	case "view":
		if len(os.Args) < 3 {
			fmt.Println("Usage: statscli view <username> or <guid>")
			return
		}
		viewUser(os.Args[2])
	case "seed-tournaments":
		seedTournaments()
	case "clear":
		clearData()
	case "status":
		showStatus()OPM CLI - OpenMOHAA Plugin Manager & Seeder

Usage:
  opm setup [users]                 Create SMF users (default: 1000)
  opm seed [matches] [workers]      Seed events with existing players
  opm seed-player <u...>            Seed events for specific user & token
  opm seed-tournaments              Seed Teams and Tournaments
  opm full [users] [matches] [wrk]  Full setup: create users + seed events
  opm view <user>                   View stats for a specific user
  opm clear                         Clear all test data
  opm status                        Show database status

Examples:
  opm setup 500
  opm seed 1000 10                  (Run 1000 matches with 10 concurrent workers)
  opm seed-player elgan MOH-123 10
  opm full [users] [matches]  Full setup: create users + seed events
  statscli view <user>       View stats for a specific user
  statscli clear             Clear all test data
  statscli status            Show database status

Examples:
  statscli seed-player elgan MOH-123 10
  statscli view PlayerOne`)
}

// ============================================================================
// VIEW COMMAND
// ============================================================================

func viewUser(identifier string) {
	fmt.Printf("Searching for user: %s...\n", identifier)

	// Postgres connection for detailed stats
	pg, err := sql.Open("postgres", PostgresURL)
	if err != nil {
		fmt.Printf("Error connecting to Postgres: %v\n", err)
		return
	}
	defer pg.Close()

	// MySQL for profile info
	mysql, err := sql.Open("mysql", MySQLDSN)
	if err != nil {
		fmt.Printf("Error connecting to MySQL: %v\n", err)
		return
	}
	defer mysql.Close()

	var memberID int
	var name, email string
	var guid string

	// Try to find by name first
	err = mysql.QueryRow("SELECT id_member, member_name, email_address FROM smf_members WHERE member_name = ?", identifier).Scan(&memberID, &name, &email)
	if err != nil {
		// Try by ID (if numeric)
		if idVal, errAtoi := strconv.Atoi(identifier); errAtoi == nil {
			err = mysql.QueryRow("SELECT id_member, member_name, email_address FROM smf_members WHERE id_member = ?", idVal).Scan(&memberID, &name, &email)
		}
		
		// If still not found, try by GUID
		if err != nil {
			err = mysql.QueryRow("SELECT m.id_member, m.member_name, m.email_address FROM smf_members m JOIN smf_mohaa_identities i ON m.id_member = i.id_member WHERE i.player_guid = ?", identifier).Scan(&memberID, &name, &email)
		}

		if err != nil {
			fmt.Printf("User not found in SMF database (searched Name, ID, GUID). Error: %v\n", err)
			return
		}
	}

	// Get GUID if we found by name
	mysql.QueryRow("SELECT player_guid FROM smf_mohaa_identities WHERE id_member = ?", memberID).Scan(&guid)
	if guid == "" {
		guid = "N/A"
	}

	fmt.Printf("\n=== User Profile ===\n")
	fmt.Printf("ID:       %d\n", memberID)
	fmt.Printf("Name:     %s\n", name)
	fmt.Printf("Email:    %s\n", email)
	fmt.Printf("GUID:     %s\n", guid)

	// Get Token Status
	var tokenCount int
	pg.QueryRow("SELECT COUNT(*) FROM player_tokens WHERE smf_user_id = $1", memberID).Scan(&tokenCount)
	fmt.Printf("Tokens:   %d active tokens\n", tokenCount)

	// Fetch Stats from ClickHouse via API (simulated or direct query if possible)
	// Since we are CLI, querying ClickHouse directly is better for "View"
	viewClickHouseStats(guid)
}

func viewClickHouseStats(guid string) {
	if guid == "N/A" {
		return
	}

	url := fmt.Sprintf("%s/?query=SELECT+countIf(event_type='kill'),+countIf(event_type='death')+FROM+raw_events+WHERE+actor_id='%s'", ClickHouseURL, guid)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error fetching stats: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var kills, deaths int
	fmt.Fscan(resp.Body, &kills, &deaths)

	kd := 0.0
	if deaths > 0 {
		kd = float64(kills) / float64(deaths)
	}

	fmt.Printf("\n=== Combat Stats ===\n")
	fmt.Printf("Kills:    %d\n", kills)
	fmt.Printf("Deaths:   %d\n", deaths)
	fmt.Printf("K/D:      %.2f\n", kd)
}

// ============================================================================
// TOURNAMENT SEEDER
// ============================================================================

func seedTournaments() {
	fmt.Println("Seeding Teams & Tournaments...")

	mysql, err := sql.Open("mysql", MySQLDSN)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer mysql.Close()

	// Ensure Tables Exist
	createTournamentTables(mysql)

	// 1. Create Teams
	teamNames := []string{"Bravo Six", "Delta Force", "Alpha Squad", "Omega", "Titans", "Rangers", "Ninjas", "Vikings", "Spartans", "Reapers"}

	var teamIDs []int
	for _, tName := range teamNames {
		// Try insert
		res, err := mysql.Exec("INSERT IGNORE INTO smf_mohaa_teams (team_name, status, created_at) VALUES (?, 'active', ?)", tName, time.Now().Unix())
		if err != nil {
			fmt.Printf("Error creating team %s: %v\n", tName, err)
			continue
		}

		id, err := res.LastInsertId()

		// If ID is 0, it means the row already existed (IGNORE)
		if id == 0 {
			err = mysql.QueryRow("SELECT id_team FROM smf_mohaa_teams WHERE team_name = ?", tName).Scan(&id)
			if err != nil {
				fmt.Printf("Error fetching existing team %s: %v\n", tName, err)
				continue
			}
		}

		if id != 0 {
			teamIDs = append(teamIDs, int(id))
		}
	}
	fmt.Printf("✓ Created/Loaded %d Teams\n", len(teamIDs))

	if len(teamIDs) == 0 {
		fmt.Println("⚠ No teams available. Aborting tournament seed.")
		return
	}

	// 2. Assign Users to Teams (Randomly)
	rows, _ := mysql.Query("SELECT id_member FROM smf_members LIMIT 100")
	var memberIDs []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		memberIDs = append(memberIDs, id)
	}
	rows.Close()

	if len(memberIDs) > 0 {
		for i, id := range memberIDs {
			teamID := teamIDs[i%len(teamIDs)]
			role := "member"
			if i < len(teamIDs) {
				role = "captain" // First user assigned to each team is captain
			}
			mysql.Exec("INSERT IGNORE INTO smf_mohaa_team_members (id_team, id_member, role, status) VALUES (?, ?, ?, 'active')", teamID, id, role)
		}
		fmt.Printf("✓ Assigned %d users to teams\n", len(memberIDs))
	} else {
		fmt.Println("⚠ No users found to assign to teams.")
	}

	// 3. Create Tournaments
	tournaments := []string{"Winter Championship 2026", "Summer Frag Fest"}
	for _, tName := range tournaments {
		res, err := mysql.Exec("INSERT INTO smf_mohaa_tournaments (name, status, format, max_teams, tournament_start) VALUES (?, 'active', 'single_elim', 16, ?)", tName, time.Now().Unix())
		if err != nil {
			fmt.Printf("Failed to create tournament: %v\n", err)
			continue
		}
		tID, _ := res.LastInsertId()

		// Register random teams
		for i, teamID := range teamIDs {
			if i >= 8 {
				break
			} // Register 8 teams
			mysql.Exec("INSERT INTO smf_mohaa_tournament_registrations (id_tournament, id_team, status) VALUES (?, ?, 'approved')", tID, teamID)
		}
		fmt.Printf("✓ Created Tournament: %s (ID: %d)\n", tName, tID)
	}
}

func createTournamentTables(db *sql.DB) {
	// Minimal schema for Teams if missing
	db.Exec(`CREATE TABLE IF NOT EXISTS smf_mohaa_teams (
		id_team INT AUTO_INCREMENT PRIMARY KEY,
		team_name VARCHAR(255) NOT NULL UNIQUE,
		logo_url VARCHAR(255),
		status VARCHAR(20) DEFAULT 'active',
		created_at INT
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS smf_mohaa_team_members (
		id_team INT,
		id_member INT,
		role VARCHAR(20),
		status VARCHAR(20),
		PRIMARY KEY (id_team, id_member)
	)`)

	// Tournaments (as seen in PHP file)
	db.Exec(`CREATE TABLE IF NOT EXISTS smf_mohaa_tournaments (
		id_tournament INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(255),
		status VARCHAR(20) DEFAULT 'open',
		format VARCHAR(20),
		max_teams INT,
		tournament_start INT,
		id_winner_team INT DEFAULT 0
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS smf_mohaa_tournament_registrations (
		id_tournament INT,
		id_team INT,
		seed INT DEFAULT 0,
		status VARCHAR(20),
		PRIMARY KEY (id_tournament, id_team)
	)`)
}

func setupUsers(numUsers int) []Player {
	fmt.Printf("Setting up %d SMF users...\n", numUsers)
	stats := &Stats{startTime: time.Now()}

	mysql, err := sql.Open("mysql", MySQLDSN)
	if err != nil {
		fmt.Printf("ERROR: Cannot connect to MySQL: %v\n", err)
		return nil
	}
	defer mysql.Close()

	if err := mysql.Ping(); err != nil {
		fmt.Printf("ERROR: MySQL ping failed: %v\n", err)
		return nil
	}
	fmt.Println("✓ MySQL connected")

	pg, err := sql.Open("postgres", PostgresURL)
	if err != nil {
		fmt.Printf("ERROR: Cannot connect to PostgreSQL: %v\n", err)
		return nil
	}
	defer pg.Close()

	if err := pg.Ping(); err != nil {
		fmt.Printf("ERROR: PostgreSQL ping failed: %v\n", err)
		return nil
	}
	fmt.Println("✓ PostgreSQL connected")

	_, _ = mysql.Exec(`
		CREATE TABLE IF NOT EXISTS smf_mohaa_identities (
			id INT AUTO_INCREMENT PRIMARY KEY,
			member_id INT NOT NULL,
			player_guid VARCHAR(64) NOT NULL UNIQUE,
			player_name VARCHAR(64),
			verified TINYINT DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_member (member_id),
			INDEX idx_guid (player_guid)
		)
	`)

	_, _ = pg.Exec(`
		CREATE TABLE IF NOT EXISTS player_tokens (
			id SERIAL PRIMARY KEY,
			smf_user_id INT NOT NULL,
			player_guid VARCHAR(64) NOT NULL,
			token VARCHAR(128) NOT NULL UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			expires_at TIMESTAMP,
			last_used TIMESTAMP
		)
	`)

	var maxID int
	mysql.QueryRow("SELECT COALESCE(MAX(id_member), 0) FROM smf_members").Scan(&maxID)
	startID := maxID + 1

	players := make([]Player, 0, numUsers)
	password := "admin123"
	now := time.Now().Unix()

	fmt.Printf("Creating users from ID %d...\n", startID)

	batchSize := 100
	for batch := 0; batch < numUsers; batch += batchSize {
		end := batch + batchSize
		if end > numUsers {
			end = numUsers
		}

		tx, _ := mysql.Begin()
		pgTx, _ := pg.Begin()

		for i := batch; i < end; i++ {
			userID := startID + i
			username := generatePlayerName(userID)
			email := fmt.Sprintf("%s@test.local", strings.ToLower(username))
			passwdHash := hashSMFPassword(username, password)
			guid := fmt.Sprintf("GUID_%05d", userID)
			token := uuid.New().String()

			_, err := tx.Exec(`
				INSERT INTO smf_members 
				(member_name, real_name, email_address, passwd, password_salt, 
				 date_registered, last_login, posts, id_group, lngfile, 
				 instant_messages, unread_messages, new_pm, alerts, buddy_list, 
				 pm_prefs, mod_prefs, personal_text, birthdate, website_title, 
				 website_url, show_online, time_format, signature, time_offset, 
				 avatar, usertitle, secret_question, secret_answer, id_theme, 
				 is_activated, validation_code, id_msg_last_visit, additional_groups, 
				 smiley_set, id_post_group, total_time_logged_in, ignore_boards, 
				 warning, passwd_flood, pm_receive_from, timezone, tfa_secret, tfa_backup)
				VALUES (?, ?, ?, ?, '', 
				        ?, ?, 0, 0, '', 
				        0, 0, 0, 0, '', 
				        0, '', '', '1004-01-01', '', 
				        '', 1, '', '', 0, 
				        '', '', '', '', 0, 
				        1, '', 0, '', 
				        '', 0, 0, '', 
				        0, '', 1, '', '', '')
			`, username, username, email, passwdHash, now, now)
			if err != nil {
				fmt.Printf("  INSERT error: %v\n", err)
				continue
			}

			var insertedID int
			tx.QueryRow("SELECT LAST_INSERT_ID()").Scan(&insertedID)

			tx.Exec(`
				INSERT INTO smf_mohaa_identities (id_member, player_guid, player_name, linked_date)
				VALUES (?, ?, ?, ?)
			`, insertedID, guid, username, now)

			pgTx.Exec(`
				INSERT INTO player_tokens (smf_user_id, player_guid, token, expires_at)
				VALUES ($1, $2, $3, NOW() + INTERVAL '1 year')
			`, insertedID, guid, token)

			skill := 0.3 + rand.Float32()*0.6
			player := Player{
				Name:      username,
				GUID:      guid,
				Team:      []string{"allies", "axis"}[rand.Intn(2)],
				Skill:     skill,
				Style:     []string{"aggressive", "defensive", "sniper", "rusher"}[rand.Intn(4)],
				Favorite:  weapons[rand.Intn(len(weapons))],
				SMFUserID: insertedID,
				Token:     token,
			}
			players = append(players, player)
			stats.AddUser()
		}

		tx.Commit()
		pgTx.Commit()
		fmt.Printf("  Created %d/%d users\n", len(players), numUsers)
	}

	fmt.Printf("\n✓ Created %d SMF users with tokens\n", len(players))
	stats.Print()
	return players
}

func seedEvents(numMatches int) {
	mysql, err := sql.Open("mysql", MySQLDSN)
	if err != nil {
		fmt.Printf("ERROR: Cannot connect to MySQL: %v\n", err)
		return
	}
	defer mysql.Close()

	rows, err := mysql.Query(`
		SELECT m.id_member, m.member_name, i.player_guid 
		FROM smf_members m 
		JOIN smf_mohaa_identities i ON m.id_member = i.id_member 
		LIMIT 2000
	`)
	if err != nil {
		fmt.Printf("ERROR: Cannot load players: %v\n", err)
		return
	}

	var players []Player
	for rows.Next() {
		var p Player
		var userID int
		rows.Scan(&userID, &p.Name, &p.GUID)
		p.SMFUserID = userID
		p.Team = []string{"allies", "axis"}[rand.Intn(2)]
		p.Skill = 0.3 + rand.Float32()*0.6
		p.Style = []string{"aggressive", "defensive", "sniper", "rusher"}[rand.Intn(4)]
		p.Favorite = weapons[rand.Intn(len(weapons))]
		players = append(players, p)
	}
	rows.Close()

	if len(players) == 0 {
		fmt.Println("No players found. Run 'statscli setup' first.")
		return
	}

	// Load tokens for these players from Postgres
	pg, err := sql.Open("postgres", PostgresURL)
	if err == nil {
		defer pg.Close()
		fmt.Println("Loading player tokens...")
		for i := range players {
			var token string
			// Query for the *latest* valid token
			err := pg.QueryRow("SELECT token FROM player_tokens WHERE smf_user_id = $1 ORDER BY expires_at DESC LIMIT 1", players[i].SMFUserID).Scan(&token)
			if err == nil {
				players[i].Token = token
			}
		}
	} else {
		fmt.Printf("Warning: Could not connect to Postgres to load tokens: %v. Login simulation may fail.\n", err)
	}

	fmt.Printf("Loaded %d players from database\n", len(players))
	seedEventsWithPlayers(numMatches, players)
}

func seedPlayer(identifier string, tokenOrMatches string, matchesInput int) {
	// Logic to parse arguments:
	// Usage 1: seed-player <name> <token> [matches]
	// Usage 2: seed-player <name> [matches]  (Auto-lookup token)
	// Usage 3: seed-player <token> [matches] (Lookup user by token)

	var token string
	var numMatches = 5

	// Heuristic to check if 'tokenOrMatches' is actually the match count (numeric)
	if val, err := strconv.Atoi(tokenOrMatches); err == nil && val > 0 {
		numMatches = val
		// If 2nd arg is number, then token is implicit (or identifier IS the token)
	} else if tokenOrMatches != "" {
		token = tokenOrMatches
		if matchesInput > 0 {
			numMatches = matchesInput
		}
	} else if matchesInput > 0 {
		numMatches = matchesInput
	}

	fmt.Printf("Seeding Player: Identifier='%s', Token='%s', Matches=%d\n", identifier, token, numMatches)

	// DB Connections
	mysql, err := sql.Open("mysql", MySQLDSN)
	if err != nil {
		fmt.Printf("Error connecting to MySQL: %v\n", err)
		return
	}
	defer mysql.Close()

	pg, err := sql.Open("postgres", PostgresURL)
	if err != nil {
		fmt.Printf("Error connecting to Postgres: %v\n", err)
		return
	}
	defer pg.Close()

	var target Player
	var memberID int

	// Case A: Identifier is likely a Token/GUID (UUID or starts with MOH-)
	isTokenLike := strings.Contains(identifier, "-") || strings.HasPrefix(identifier, "MOH")
	
	found := false
	if isTokenLike {
		// Try to find user by Token in Postgres FIRST
		var smfID int
		err := pg.QueryRow("SELECT smf_user_id FROM player_tokens WHERE token = $1", identifier).Scan(&smfID)
		if err == nil {
			// Found user ID by token
			token = identifier
			identifier = strconv.Itoa(smfID) // treat as ID for next lookup
			fmt.Printf("✓ Found User ID %d from token '%s'\n", smfID, token)
		} else {
			// Maybe identifier is GUID?
			err = mysql.QueryRow("SELECT m.id_member, m.member_name, i.player_guid FROM smf_members m JOIN smf_mohaa_identities i ON m.id_member = i.id_member WHERE i.player_guid = ?", identifier).Scan(&memberID, &target.Name, &target.GUID)
			if err == nil {
				found = true
				fmt.Printf("✓ Found User '%s' from GUID '%s'\n", target.Name, identifier)
			}
		}
	}

	// Case B: Identifier is Username (or ID from Case A)
	if !found {
		// Try by Name
		err = mysql.QueryRow("SELECT m.id_member, m.member_name, i.player_guid FROM smf_members m JOIN smf_mohaa_identities i ON m.id_member = i.id_member WHERE m.member_name = ?", identifier).Scan(&memberID, &target.Name, &target.GUID)
		if err != nil {
			// Try by ID
			if idVal, errAtoi := strconv.Atoi(identifier); errAtoi == nil {
				err = mysql.QueryRow("SELECT m.id_member, m.member_name, i.player_guid FROM smf_members m JOIN smf_mohaa_identities i ON m.id_member = i.id_member WHERE m.id_member = ?", idVal).Scan(&memberID, &target.Name, &target.GUID)
			}
		}

		if err == nil {
			found = true
		}
	}

	if !found {
		fmt.Printf("User '%s' not found. Ensure they exist and have a GUID (run setup if needed).\n", identifier)
		return
	}

	target.SMFUserID = memberID

	// Token Handling
	if token == "" {
		// If token wasn't provided or found, look it up
		err := pg.QueryRow("SELECT token FROM player_tokens WHERE smf_user_id = $1 ORDER BY expires_at DESC LIMIT 1", memberID).Scan(&token)
		if err != nil {
			// No token? Generate one.
			token = uuid.New().String()
			fmt.Println("⚠ No token found. Generated new one.")
		} else {
			fmt.Println("✓ Loaded existing token")
		}
	}
	target.Token = token
	
	// Ensure token is saved/updated
	pg.Exec(`
		INSERT INTO player_tokens (smf_user_id, player_guid, token, expires_at)
		VALUES ($1, $2, $3, NOW() + INTERVAL '1 year')
		ON CONFLICT (token) DO UPDATE SET expires_at = NOW() + INTERVAL '1 year'
	`, target.SMFUserID, target.GUID, target.Token)


	// Set Defaults
	target.Team = "allies"
	target.Skill = 0.9
	target.Style = "aggressive"
	target.Favorite = "Thompson"

	// FIX: If GUID is legacy/invalid "GUID_ELGAN", update it to a valid UUID
	if target.GUID == "GUID_ELGAN" || !strings.Contains(target.GUID, "-") {
		newGUID := uuid.New().String()
		fmt.Printf("⚠ Found invalid GUID '%s' for user '%s'. Updating to '%s'...\n", target.GUID, target.Name, newGUID)
		
		_, err := mysql.Exec("UPDATE smf_mohaa_identities SET player_guid = ? WHERE id_member = ?", newGUID, target.SMFUserID)
		if err != nil {
			fmt.Printf("Error updating GUID in MySQL: %v\n", err)
			return
		}
		target.GUID = newGUID
		fmt.Println("✓ GUID updated in MySQL")
	}

	fmt.Printf("Target Player: %s (ID: %d, GUID: %s)\n", target.Name, target.SMFUserID, target.GUID)

	// 3. Load ~9 Opponents to ensure a full match
	rows, err := mysql.Query(`
		SELECT m.id_member, m.member_name, i.player_guid 
		FROM smf_members m 
		JOIN smf_mohaa_identities i ON m.id_member = i.id_member 
		WHERE m.id_member != ? 
		LIMIT 9
	`, memberID)
	
	var opponents []Player
	if err == nil {
		for rows.Next() {
			var p Player
			var uid int
			rows.Scan(&uid, &p.Name, &p.GUID)
			p.SMFUserID = uid
			
			// Fetch their token too
			pg.QueryRow("SELECT token FROM player_tokens WHERE smf_user_id = $1 ORDER BY expires_at DESC LIMIT 1", uid).Scan(&p.Token)
			
			// Random attributes
			p.Team = []string{"allies", "axis"}[rand.Intn(2)]
			p.Skill = 0.5
			p.Style = "defensive"
			p.Favorite = weapons[rand.Intn(len(weapons))]
			
			opponents = append(opponents, p)
		}
		rows.Close()
	}

	players := append([]Player{target}, opponents...)
	fmt.Printf("assembled match pool of %d players (Target + %d Opponents)\n", len(players), len(opponents))

	seedEventsWithPlayers(numMatches, players)
}

func seedEventsWithPlayers(numMatches int, players []Player) {
	fmt.Printf("Seeding %d matches with %d players...\n", numMatches, len(players))
	stats := &Stats{startTime: time.Now()}

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	var wg sync.WaitGroup
	matchChan := make(chan int, numMatches)

	for i := 0; i < numMatches; i++ {
		matchChan <- i
	}
	close(matchChan)

	for w := 0; w < Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range matchChan {
				sim := NewMatchSimulator(client, stats, players)
				sim.Run()
			}
		}()
	}

	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				events := atomic.LoadInt64(&stats.eventsSent)
				matches := atomic.LoadInt64(&stats.matchesRun)
				elapsed := time.Since(stats.startTime).Seconds()
				rate := float64(events) / elapsed
				fmt.Printf("\r  Matches: %d/%d | Events: %d | Rate: %.0f/s     ", matches, numMatches, events, rate)
			case <-done:
				return
			}
		}
	}()

	wg.Wait()
	close(done)
	fmt.Println()
	stats.Print()
}

func clearData() {
	fmt.Println("Clearing all test data...")

	resp, err := http.Post(ClickHouseURL+"/?query=TRUNCATE+TABLE+IF+EXISTS+raw_events", "", nil)
	if err == nil {
		resp.Body.Close()
		fmt.Println("✓ ClickHouse cleared")
	}

	pg, err := sql.Open("postgres", PostgresURL)
	if err == nil {
		pg.Exec("TRUNCATE player_tokens, match_reports, achievements_earned RESTART IDENTITY CASCADE")
		pg.Close()
		fmt.Println("✓ PostgreSQL cleared")
	}

	mysql, err := sql.Open("mysql", MySQLDSN)
	if err == nil {
		mysql.Exec("DELETE FROM smf_mohaa_identities WHERE member_id > 1")
		mysql.Exec("DELETE FROM smf_members WHERE id_member > 1")

		// Clear Tournaments & Teams (DROP to ensure schema updates)
		mysql.Exec("DROP TABLE IF EXISTS smf_mohaa_tournaments")
		mysql.Exec("DROP TABLE IF EXISTS smf_mohaa_tournament_registrations")
		mysql.Exec("DROP TABLE IF EXISTS smf_mohaa_team_members")
		mysql.Exec("DROP TABLE IF EXISTS smf_mohaa_teams")


		mysql.Close()
		fmt.Println("✓ SMF test users & tournaments cleared")
	}

	fmt.Println("\nAll test data cleared.")
}

func showStatus() {
	fmt.Println("=== Database Status ===")

	resp, err := http.Get(ClickHouseURL + "/?query=SELECT+count()+FROM+raw_events")
	if err == nil {
		defer resp.Body.Close()
		var count string
		fmt.Fscan(resp.Body, &count)
		fmt.Printf("ClickHouse raw_events: %s rows\n", strings.TrimSpace(count))
	} else {
		fmt.Printf("ClickHouse: ERROR - %v\n", err)
	}

	pg, err := sql.Open("postgres", PostgresURL)
	if err == nil {
		var count int
		pg.QueryRow("SELECT COUNT(*) FROM player_tokens").Scan(&count)
		fmt.Printf("PostgreSQL tokens: %d\n", count)
		pg.Close()
	}

	mysql, err := sql.Open("mysql", MySQLDSN)
	if err == nil {
		var users, identities int
		mysql.QueryRow("SELECT COUNT(*) FROM smf_members").Scan(&users)
		mysql.QueryRow("SELECT COUNT(*) FROM smf_mohaa_identities").Scan(&identities)
		fmt.Printf("SMF users: %d\n", users)
		fmt.Printf("SMF identities: %d\n", identities)
		mysql.Close()
	}

	resp, err = http.Get("http://localhost:8080/health")
	if err == nil {
		resp.Body.Close()
		fmt.Printf("API: Healthy (status %d)\n", resp.StatusCode)
	} else {
		fmt.Printf("API: ERROR - %v\n", err)
	}
}

type MatchSimulator struct {
	client  *http.Client
	stats   *Stats
	matchID string
	server  struct{ ID, Name string }
	mapName string
	players []Player
	baseTS  float64
}

func NewMatchSimulator(client *http.Client, stats *Stats, allPlayers []Player) *MatchSimulator {
	server := servers[rand.Intn(len(servers))]
	mapName := maps[rand.Intn(len(maps))]

	numPlayers := 8 + rand.Intn(9)
	if numPlayers > len(allPlayers) {
		numPlayers = len(allPlayers)
	}

	shuffled := make([]Player, len(allPlayers))
	copy(shuffled, allPlayers)
	rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	matchPlayers := shuffled[:numPlayers]

	for i := range matchPlayers {
		if i%2 == 0 {
			matchPlayers[i].Team = "allies"
		} else {
			matchPlayers[i].Team = "axis"
		}
	}

	return &MatchSimulator{
		client:  client,
		stats:   stats,
		matchID: uuid.New().String(),
		server:  struct{ ID, Name string }{server.ID, server.Name},
		mapName: mapName,
		players: matchPlayers,
		baseTS:  float64(time.Now().Add(-time.Duration(rand.Intn(720)) * time.Hour).Unix()),
	}
}

func (m *MatchSimulator) Run() {
	// 1. Simulate players logging in (Device Auth / Token Verify)
	for i := range m.players {
		// Initialize state
		m.players[i].Alive = true
		m.players[i].HP = 100
		m.players[i].Ammo = 30
		m.players[i].Weapon = m.players[i].Favorite
		m.loginPlayer(m.players[i])
	}

	// 2. Start Match
	m.sendEvent(map[string]interface{}{
		"type": EventMatchStart, "match_id": m.matchID, "server_id": m.server.ID,
		"server_token": ServerToken, "map_name": m.mapName,
		"gametype": gametypes[rand.Intn(len(gametypes))], "timestamp": m.baseTS,
	})

	for _, p := range m.players {
		m.baseTS += float64(rand.Intn(3) + 1)
		m.sendEvent(map[string]interface{}{
			"type": EventClientConnect, "match_id": m.matchID, "timestamp": m.baseTS,
			"player_name": p.Name, "player_guid": p.GUID, "player_team": p.Team, "player_smf_id": p.SMFUserID,
		})
		x, y, z := m.generatePosition()
		m.sendEvent(map[string]interface{}{
			"type": EventPlayerSpawn, "match_id": m.matchID, "timestamp": m.baseTS + 0.5,
			"player_name": p.Name, "player_guid": p.GUID, "player_team": p.Team, "player_smf_id": p.SMFUserID,
			"pos_x": x, "pos_y": y, "pos_z": z,
		})
	}

	numRounds := 2 + rand.Intn(4)
	for round := 1; round <= numRounds; round++ {
		m.simulateRound(round)
	}

	for _, p := range m.players {
		m.baseTS += float64(rand.Intn(3))
		m.sendEvent(map[string]interface{}{
			"type": EventClientDisconnect, "match_id": m.matchID, "timestamp": m.baseTS,
			"player_name": p.Name, "player_guid": p.GUID,
		})
	}

	m.sendEvent(map[string]interface{}{
		"type": EventMatchEnd, "match_id": m.matchID, "server_id": m.server.ID,
		"timestamp": m.baseTS, "allies_score": rand.Intn(50), "axis_score": rand.Intn(50),
	})
	m.stats.AddMatch()
}

func (m *MatchSimulator) loginPlayer(p Player) {
	// Simulate the Game Server verifying the player's token
	payload := map[string]string{
		"token":          p.Token,
		"player_guid":    p.GUID,
		"server_name":    m.server.Name,
		"server_address": m.server.ID,
		"player_ip":      fmt.Sprintf("192.168.%d.%d", rand.Intn(255), rand.Intn(255)),
	}

	data, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "http://localhost:8080/api/v1/auth/verify", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ServerToken)

	resp, err := m.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func (m *MatchSimulator) simulateRound(roundNum int) {
	m.baseTS += float64(rand.Intn(5) + 1)
	m.sendEvent(map[string]interface{}{
		"type": EventRoundStart, "match_id": m.matchID, "timestamp": m.baseTS, "round": roundNum,
	})

	numEvents := 100 + rand.Intn(200)
	for i := 0; i < numEvents; i++ {
		m.baseTS += float64(rand.Intn(3)) + rand.Float64()
		// Select random player
		pIdx := rand.Intn(len(m.players))
		// We use a pointer so updates (Ammo, HP) persist
		actor := &m.players[pIdx]

		if !actor.Alive {
			// Respawn logic
			m.baseTS += 2.0
			x, y, z := m.generatePosition()
			m.sendEvent(map[string]interface{}{
				"type": EventPlayerRespawn, "match_id": m.matchID, "timestamp": m.baseTS,
				"player_guid": actor.GUID, "player_name": actor.Name, "pos_x": x, "pos_y": y, "pos_z": z,
			})
			actor.Alive = true
			actor.HP = 100
			actor.Ammo = 30
			continue
		}

		r := rand.Float32()
		// Enhanced State Machine aligned with Seeder
		if r < 0.05 {
			m.simulateObjective(actor)
		} else if r < 0.10 {
			m.simulateVehicle(actor)
		} else if r < 0.40 {
			m.simulateCombat(actor)
		} else if r < 0.55 {
			m.simulateMovement(actor)
		} else if r < 0.65 {
			m.simulateInteraction(actor)
		} else if r < 0.75 {
			m.simulateWeaponHandling(actor) 
		} else if r < 0.80 {
			if rand.Float32() < 0.5 {
				m.simulateTurret(actor)
			} else {
				m.simulateLadder(actor)
			}
		} else if r < 0.85 {
			m.simulateChat(actor)
		} else if r < 0.90 {
			m.simulateVote(actor)
		} else {
			m.simulateMisc(actor)
		}
	}

	m.sendEvent(map[string]interface{}{
		"type": EventRoundEnd, "match_id": m.matchID, "timestamp": m.baseTS, "round": roundNum,
	})
}

func (m *MatchSimulator) simulateObjective(p *Player) {
	m.sendEvent(map[string]interface{}{
		"type":             EventObjectiveCapture,
		"match_id":         m.matchID,
		"timestamp":        m.baseTS,
		"player_guid":      p.GUID,
		"player_name":      p.Name,
		"player_team":      p.Team,
		"objective":        fmt.Sprintf("obj_%d", rand.Intn(3)+1),
		"objective_status": "complete",
	})
}

func (m *MatchSimulator) simulateVehicle(p *Player) {
	vehicleName := "Sherman Tank"
	if p.Team == "axis" {
		vehicleName = "Tiger Tank"
	}
	m.sendEvent(map[string]interface{}{
		"type":        EventVehicleEnter,
		"match_id":    m.matchID,
		"timestamp":   m.baseTS,
		"player_guid": p.GUID,
		"entity":      vehicleName,
		"seat":        "driver",
	})
	p.InVehicle = true

	// Drive
	m.baseTS += 2.5
	
	// Exit or Die
	if rand.Float32() < 0.2 {
		m.sendEvent(map[string]interface{}{
			"type":        EventVehicleDeath,
			"match_id":    m.matchID,
			"timestamp":   m.baseTS,
			"player_guid": p.GUID,
			"entity":      vehicleName,
		})
	} else {
		m.sendEvent(map[string]interface{}{
			"type":        EventVehicleExit,
			"match_id":    m.matchID,
			"timestamp":   m.baseTS,
			"player_guid": p.GUID,
			"entity":      vehicleName,
		})
	}
	p.InVehicle = false
}

func (m *MatchSimulator) simulateCombat(attacker *Player) {
	// Find victim
	var victim *Player
	attempts := 0
	for {
		attempts++
		if attempts > 10 { return } // No target found
		vIdx := rand.Intn(len(m.players))
		victim = &m.players[vIdx]
		if victim.GUID != attacker.GUID && victim.Alive {
			break
		}
	}

	weapon := attacker.Weapon
	if weapon == "" { weapon = "Thompson" }
	
	hitloc := m.selectHitloc(attacker.Skill)
	aX, aY, aZ := m.generatePosition()
	vX, vY, vZ := m.generatePosition()

	// Fire
	m.sendEvent(map[string]interface{}{
		"type": EventWeaponFire, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_name": attacker.Name, "player_guid": attacker.GUID, "player_team": attacker.Team,
		"weapon": weapon, "pos_x": aX, "pos_y": aY, "pos_z": aZ,
	})
	attacker.Ammo--

	// Relad if needed
	if attacker.Ammo <= 0 {
		m.baseTS += 1.0
		m.sendEvent(map[string]interface{}{
			"type": EventWeaponReload, "match_id": m.matchID, "timestamp": m.baseTS,
			"player_guid": attacker.GUID, "weapon": weapon,
		})
		attacker.Ammo = 30
		return
	}

	if rand.Float32() < attacker.Skill*0.9 {
		// Hit
		m.sendEvent(map[string]interface{}{
			"type": EventWeaponHit, "match_id": m.matchID, "timestamp": m.baseTS + 0.05,
			"player_name": attacker.Name, "player_guid": attacker.GUID,
			"target_name": victim.Name, "target_guid": victim.GUID,
			"weapon": weapon, "hitloc": hitloc,
		})

		damage := 25 + rand.Intn(20)
		if hitloc == "head" {
			damage = 100
		}

		m.sendEvent(map[string]interface{}{
			"type": EventPlayerPain, "match_id": m.matchID, "timestamp": m.baseTS + 0.1,
			"player_name": victim.Name, "player_guid": victim.GUID,
			"attacker_name": attacker.Name, "attacker_guid": attacker.GUID,
			"damage": damage, "hitloc": hitloc,
		})
		victim.HP -= damage
		
		eventsDmg := map[string]interface{}{
			"type": EventDamage, "match_id": m.matchID, "timestamp": m.baseTS + 0.1,
			"attacker_name": attacker.Name, "attacker_guid": attacker.GUID, "attacker_team": attacker.Team,
			"victim_name": victim.Name, "victim_guid": victim.GUID,
			"damage": damage, "weapon": weapon, "hitloc": hitloc,
		}
		m.sendEvent(eventsDmg)

		// Kill logic
		if victim.HP <= 0 {
			killType := EventKill
			weaponName := weapon
			
			// Special Kills
			specialRoll := rand.Float32()
			if specialRoll < 0.05 {
				killType = EventPlayerBash
				weaponName = "MoH::Bash"
			} else if specialRoll < 0.10 {
				killType = EventGrenadeExplode
				weaponName = "Grenade"
			}

			if hitloc == "head" {
				m.sendEvent(map[string]interface{}{
					"type": EventHeadshot, "match_id": m.matchID, "timestamp": m.baseTS + 0.15,
					"attacker_name": attacker.Name, "attacker_guid": attacker.GUID,
					"victim_name": victim.Name, "victim_guid": victim.GUID,
					"weapon": weapon,
				})
			}

			m.sendEvent(map[string]interface{}{
				"type":          killType,
				"match_id":      m.matchID,
				"timestamp":     m.baseTS + 0.16,
				"attacker_name": attacker.Name, "attacker_guid": attacker.GUID, "attacker_team": attacker.Team,
				"victim_name":   victim.Name, "victim_guid": victim.GUID, "victim_team": victim.Team,
				"weapon":        weaponName, "hitloc": hitloc,
				"attacker_x": aX, "attacker_y": aY, "attacker_z": aZ,
				"victim_x": vX, "victim_y": vY, "victim_z": vZ,
			})

			m.sendEvent(map[string]interface{}{
				"type": EventDeath, "match_id": m.matchID, "timestamp": m.baseTS + 0.17,
				"player_name": victim.Name, "player_guid": victim.GUID, "player_team": victim.Team,
				"attacker_name": attacker.Name, "attacker_guid": attacker.GUID,
				"weapon": weaponName, "pos_x": vX, "pos_y": vY, "pos_z": vZ,
			})
			
			victim.Alive = false
		}
	}
}

func (m *MatchSimulator) simulateMovement(actor *Player) {
	x, y, z := m.generatePosition()
	eventType := []string{EventJump, EventCrouch, EventProne, EventLand, EventDistance}[rand.Intn(5)]
	
	if eventType == EventCrouch { actor.Stance = "crouch" }
	if eventType == EventProne { actor.Stance = "prone" }

	event := map[string]interface{}{
		"type": eventType, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_name": actor.Name, "player_guid": actor.GUID, "player_team": actor.Team,
		"pos_x": x, "pos_y": y, "pos_z": z,
	}
	if eventType == EventDistance {
		event["distance"] = rand.Float32() * 100
	}
	m.sendEvent(event)
}

func (m *MatchSimulator) simulateInteraction(actor *Player) {
	x, y, z := m.generatePosition()
	eventType := []string{EventItemPickup, EventDoorOpen}[rand.Intn(2)]
	event := map[string]interface{}{
		"type": eventType, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_name": actor.Name, "player_guid": actor.GUID,
		"pos_x": x, "pos_y": y, "pos_z": z,
	}
	if eventType == EventItemPickup {
		items := []string{"health_25", "health_50", "ammo_pistol", "ammo_rifle"}
		event["item"] = items[rand.Intn(len(items))]
	}
	m.sendEvent(event)
}

func (m *MatchSimulator) simulateChat(actor *Player) {
	messages := []string{"nice shot!", "gg", "lol", "brb", "good game", "haha", "wow", "rekt", "ez", "wp"}
	m.sendEvent(map[string]interface{}{
		"type": EventPlayerSay, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_name": actor.Name, "player_guid": actor.GUID,
		"message": messages[rand.Intn(len(messages))],
	})
}

func (m *MatchSimulator) simulateWeaponHandling(actor *Player) {
	// Holster
	m.sendEvent(map[string]interface{}{
		"type": EventWeaponHolster, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_guid": actor.GUID, "weapon": actor.Weapon,
	})
	m.baseTS += 0.5
	
	// Change
	newWep := weapons[rand.Intn(len(weapons))]
	m.sendEvent(map[string]interface{}{
		"type": EventWeaponChange, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_guid": actor.GUID, "old_weapon": actor.Weapon, "new_weapon": newWep,
	})
	actor.Weapon = newWep
	m.baseTS += 0.5

	// Raise
	m.sendEvent(map[string]interface{}{
		"type": EventWeaponRaise, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_guid": actor.GUID, "weapon": actor.Weapon,
	})
}

func (m *MatchSimulator) simulateTurret(actor *Player) {
	m.sendEvent(map[string]interface{}{
		"type": EventTurretEnter, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_guid": actor.GUID, "turret": "mg42_nest",
	})
	actor.OnTurret = true
	m.baseTS += 2.0
	m.sendEvent(map[string]interface{}{
		"type": EventWeaponFire, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_guid": actor.GUID, "weapon": "Turret MG42",
	})
	m.baseTS += 2.0
	m.sendEvent(map[string]interface{}{
		"type": EventTurretExit, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_guid": actor.GUID, "turret": "mg42_nest",
	})
	actor.OnTurret = false
}

func (m *MatchSimulator) simulateLadder(actor *Player) {
	m.sendEvent(map[string]interface{}{
		"type": EventLadderMount, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_guid": actor.GUID, "ladder_id": "ladder_1",
	})
	m.baseTS += 2.0
	m.sendEvent(map[string]interface{}{
		"type": EventLadderDismount, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_guid": actor.GUID, "ladder_id": "ladder_1",
	})
}

func (m *MatchSimulator) simulateVote(actor *Player) {
	m.sendEvent(map[string]interface{}{
		"type": EventVoteStart, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_guid": actor.GUID, "vote_string": "map restart",
	})
	m.baseTS += 10.0
	m.sendEvent(map[string]interface{}{
		"type": EventVoteFailed, "match_id": m.matchID, "timestamp": m.baseTS,
		"fail_reason": "timeout",
	})
}

func (m *MatchSimulator) simulateMisc(actor *Player) {
	m.sendEvent(map[string]interface{}{
		"type": EventPlayerSpectate, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_guid": actor.GUID, "is_spectator": true,
	})
	m.baseTS += 5.0
	m.sendEvent(map[string]interface{}{
		"type": EventPlayerSpectate, "match_id": m.matchID, "timestamp": m.baseTS,
		"player_guid": actor.GUID, "is_spectator": false,
	})
}

func (m *MatchSimulator) generatePosition() (x, y, z float32) {
	return rand.Float32()*4000 - 2000, rand.Float32()*4000 - 2000, rand.Float32()*200 + 10
}

func (m *MatchSimulator) selectWeapon(p Player) string {
	if rand.Float32() < 0.6 {
		return p.Favorite
	}
	return weapons[rand.Intn(len(weapons))]
}

func (m *MatchSimulator) selectHitloc(skill float32) string {
	if rand.Float32() < skill*0.3 {
		return "head"
	}
	return hitlocs[rand.Intn(len(hitlocs))]
}

func (m *MatchSimulator) sendEvent(event map[string]interface{}) {
	time.Sleep(5 * time.Millisecond)
	data, _ := json.Marshal(event)

	req, _ := http.NewRequest("POST", APIURL, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ServerToken)

	resp, err := m.client.Do(req)
	if err != nil {
		m.stats.AddError()
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		m.stats.AddEvents(1)
	} else {
		m.stats.AddError()
	}
}
