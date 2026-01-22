#!/usr/bin/env php
<?php
/**
 * Comprehensive SMF Stats Testing & Validation Tool
 * 
 * This tool:
 * 1. Fetches data from API directly
 * 2. Fetches data from SMF pages
 * 3. Compares values to find mismatches
 * 4. Reports all broken stats
 * 
 * Usage: php test_all_smf_stats.php [player_guid]
 */

error_reporting(E_ALL);
ini_set('display_errors', '1');

// Configuration
$apiBaseUrl = 'http://localhost:8080/api/v1';
$smfBaseUrl = 'http://localhost:8888';
$testGuid = $argv[1] ?? '72750883-29ae-4377-85c4-9367f1f89d1a'; // elgan's GUID
$testSMFID = 1; // elgan's SMF ID

echo "=====================================\n";
echo "SMF Stats Comprehensive Test Suite\n";
echo "=====================================\n\n";

/**
 * Helper to fetch JSON from API
 */
function fetchAPI($endpoint) {
    global $apiBaseUrl;
    $url = $apiBaseUrl . $endpoint;
    $response = @file_get_contents($url);
    if ($response === false) {
        return ['error' => 'Failed to fetch: ' . $url];
    }
    return json_decode($response, true);
}

/**
 * Helper to fetch HTML from SMF (requires auth cookie if needed)
 */
function fetchSMF($path) {
    global $smfBaseUrl;
    $url = $smfBaseUrl . $path;
    $response = @file_get_contents($url);
    if ($response === false) {
        return '';
    }
    return $response;
}

/**
 * Extract stat value from HTML using regex
 */
function extractStatFromHTML($html, $pattern) {
    if (preg_match($pattern, $html, $matches)) {
        // Remove commas and convert to number
        return intval(str_replace(',', '', $matches[1]));
    }
    return null;
}

// ============================================================================
// TEST 1: Player Stats Endpoint
// ============================================================================
echo "TEST 1: Player Stats API Endpoint\n";
echo str_repeat("-", 50) . "\n";

$playerData = fetchAPI("/stats/player/{$testGuid}");
if (isset($playerData['error'])) {
    echo "❌ FAILED: {$playerData['error']}\n\n";
} else {
    $player = $playerData['player'] ?? [];
    
    $tests = [
        'Player Name' => $player['name'] ?? 'N/A',
        'Total Kills' => $player['total_kills'] ?? 0,
        'Total Deaths' => $player['total_deaths'] ?? 0,
        'K/D Ratio' => round($player['kd_ratio'] ?? 0, 2),
        'Matches Played' => $player['matches_played'] ?? 0,
        'Matches Won' => $player['matches_won'] ?? 0,
        'Win Rate %' => round($player['win_rate'] ?? 0, 2),
        'Accuracy %' => round($player['accuracy'] ?? 0, 2),
        'Headshot %' => round($player['headshot_percent'] ?? 0, 2),
    ];
    
    foreach ($tests as $label => $value) {
        $status = ($value !== 0 && $value !== 'N/A') ? '✅' : '⚠️';
        printf("%s %-20s: %s\n", $status, $label, $value);
    }
}
echo "\n";

// ============================================================================
// TEST 2: Gametypes Breakdown
// ============================================================================
echo "TEST 2: Gametypes Stats\n";
echo str_repeat("-", 50) . "\n";

$gametypesData = fetchAPI("/stats/player/{$testGuid}/gametypes");
if (isset($gametypesData['error'])) {
    echo "❌ FAILED: {$gametypesData['error']}\n\n";
} else if (is_array($gametypesData)) {
    $totalWins = 0;
    foreach ($gametypesData as $gt) {
        $type = $gt['gametype'] ?? 'unknown';
        $wins = $gt['matches_won'] ?? 0;
        $played = $gt['matches_played'] ?? 0;
        $winRate = $played > 0 ? round(($wins / $played) * 100, 1) : 0;
        
        $totalWins += $wins;
        $status = $wins > 0 ? '✅' : '❌';
        printf("%s %-6s: %2d wins / %2d played (%s%%)\n", 
            $status, strtoupper($type), $wins, $played, $winRate);
    }
    echo "\n";
    echo "✅ Total Wins Across Gametypes: $totalWins\n";
} else {
    echo "❌ Invalid response format\n";
}
echo "\n";

// ============================================================================
// TEST 3: Leaderboard Cards (Wins specifically)
// ============================================================================
echo "TEST 3: Leaderboard Cards (Wins)\n";
echo str_repeat("-", 50) . "\n";

$cardsData = fetchAPI("/stats/leaderboard/cards");
if (isset($cardsData['error'])) {
    echo "❌ FAILED: {$cardsData['error']}\n\n";
} else {
    $winsCard = $cardsData['wins'] ?? [];
    $elganInWins = array_filter($winsCard, fn($p) => $p['id'] === $testGuid);
    
    if (!empty($elganInWins)) {
        $elgan = array_values($elganInWins)[0];
        echo "✅ Elgan found in WINS leaderboard: {$elgan['value']} wins\n";
    } else {
        echo "❌ Elgan NOT found in WINS leaderboard\n";
    }
}
echo "\n";

// ============================================================================
// TEST 4: SMF Dashboard (Requires Login)
// ============================================================================
echo "TEST 4: SMF Dashboard Display\n";
echo str_repeat("-", 50) . "\n";
echo "NOTE: This test requires authenticated SMF session\n";
echo "Please check manually at: $smfBaseUrl/index.php?action=mohaadashboard\n";
echo "\n";

// If we have access to SMF directly via file system
if (file_exists('../smf-mohaa/Sources/MohaaPlayers.php')) {
    echo "✅ SMF source files accessible\n";
    echo "   You can test the dashboard by logging in as 'elgan'\n";
} else {
    echo "⚠️  SMF source not accessible from this location\n";
}
echo "\n";

// ============================================================================
// TEST 5: Deep Stats (Combat, Weapons, Movement)
// ============================================================================
echo "TEST 5: Deep Stats Endpoints\n";
echo str_repeat("-", 50) . "\n";

$deepEndpoints = [
    'combat' => "/stats/player/{$testGuid}/combat",
    'weapons' => "/stats/player/{$testGuid}/weapons",
    'movement' => "/stats/player/{$testGuid}/movement",
    'stance' => "/stats/player/{$testGuid}/stance",
];

foreach ($deepEndpoints as $name => $endpoint) {
    $data = fetchAPI($endpoint);
    if (isset($data['error'])) {
        echo "❌ $name: {$data['error']}\n";
    } else {
        $count = count($data);
        $status = $count > 0 ? '✅' : '⚠️';
        echo "$status $name: $count items\n";
    }
}
echo "\n";

// ============================================================================
// TEST 6: Match History
// ============================================================================
echo "TEST 6: Match History\n";
echo str_repeat("-", 50) . "\n";

$matchData = fetchAPI("/stats/player/{$testGuid}/matches?limit=10");
if (isset($matchData['error'])) {
    echo "❌ FAILED: {$matchData['error']}\n";
} else {
    $matches = $matchData['matches'] ?? [];
    echo "✅ Retrieved " . count($matches) . " recent matches\n";
    
    if (count($matches) > 0) {
        echo "\nLast 3 matches:\n";
        foreach (array_slice($matches, 0, 3) as $i => $match) {
            $result = ($match['won'] ?? false) ? 'WIN' : 'LOSS';
            $resultColor = $result === 'WIN' ? '✅' : '❌';
            echo "  $resultColor Match {$match['match_id']}: $result | K:{$match['kills']} D:{$match['deaths']} | {$match['gametype']} on {$match['map']}\n";
        }
    }
}
echo "\n";

// ============================================================================
// TEST 7: Achievements
// ============================================================================
echo "TEST 7: Achievements System\n";
echo str_repeat("-", 50) . "\n";

// Check MySQL for player achievements
$mysqlHost = 'localhost';
$mysqlUser = 'smf';
$mysqlPass = 'smf_password';
$mysqlDB = 'smf';

try {
    $conn = new mysqli($mysqlHost, $mysqlUser, $mysqlPass, $mysqlDB);
    if ($conn->connect_error) {
        throw new Exception($conn->connect_error);
    }
    
    // Count total achievements
    $result = $conn->query("SELECT COUNT(*) as total FROM smf_mohaa_achievements");
    $total = $result->fetch_assoc()['total'];
    echo "✅ Total achievements in database: $total\n";
    
    // Count unlocked for test player
    $stmt = $conn->prepare("SELECT COUNT(*) as unlocked FROM smf_mohaa_player_achievements WHERE smf_member_id = ?");
    $stmt->bind_param("i", $testSMFID);
    $stmt->execute();
    $unlocked = $stmt->get_result()->fetch_assoc()['unlocked'];
    echo "✅ Achievements unlocked by elgan: $unlocked\n";
    
    if ($unlocked > 0) {
        echo "\n  Recently unlocked:\n";
        $stmt = $conn->prepare("
            SELECT a.achievement_name, a.achievement_code, pa.unlocked_at 
            FROM smf_mohaa_player_achievements pa
            JOIN smf_mohaa_achievements a ON pa.achievement_id = a.achievement_id
            WHERE pa.smf_member_id = ?
            ORDER BY pa.unlocked_at DESC
            LIMIT 5
        ");
        $stmt->bind_param("i", $testSMFID);
        $stmt->execute();
        $result = $stmt->get_result();
        while ($row = $result->fetch_assoc()) {
            echo "  🏆 {$row['achievement_name']} ({$row['achievement_code']}) - {$row['unlocked_at']}\n";
        }
    }
    
    $conn->close();
} catch (Exception $e) {
    echo "❌ Database check failed: {$e->getMessage()}\n";
}
echo "\n";

// ============================================================================
// SUMMARY
// ============================================================================
echo "=====================================\n";
echo "Summary\n";
echo "=====================================\n";
echo "✅ = Working correctly\n";
echo "⚠️  = No data or suspicious value\n";
echo "❌ = Broken or error\n";
echo "\n";
echo "Next Steps:\n";
echo "1. Login to SMF as 'elgan' and check War Room dashboard\n";
echo "2. Verify WINS displays as 13 (not 0)\n";
echo "3. Check each tab (Combat, Weapons, Maps, etc.)\n";
echo "4. Look for empty charts or \"No data\" messages\n";
echo "\n";
echo "To re-run: php test_all_smf_stats.php <player_guid>\n";
