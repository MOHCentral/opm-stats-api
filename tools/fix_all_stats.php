#!/usr/bin/env php
<?php
/**
 * SMF Stats Comprehensive Fix Script
 * 
 * Identifies and fixes all broken stats:
 * 1. Checks if wins/losses are populated
 * 2. Tests all API endpoints
 * 3. Verifies data seeding
 * 4. Fixes empty stats
 */

define('SMF', 1);
require_once('/var/www/smf/SSI.php');

echo "==================================================\n";
echo "SMF Stats Comprehensive Fix\n";
echo "==================================================\n\n";

// Database connections
$smfDb = $smcFunc['db_connection'];
$apiBase = 'http://localhost:8080';

// Test user
$testUserId = 1; // SMF member ID
$testGuid = null;

// Get linked GUID
echo "[1] Checking user identity linking...\n";
$guidResult = $smcFunc['db_query']('', '
    SELECT guid
    FROM {db_prefix}mohaa_identities
    WHERE id_member = {int:member}
    LIMIT 1',
    ['member' => $testUserId]
);

if ($smcFunc['db_num_rows']($guidResult) > 0) {
    $row = $smcFunc['db_fetch_assoc']($guidResult);
    $testGuid = $row['guid'];
    echo "✓ User $testUserId linked to GUID: $testGuid\n";
} else {
    echo "✗ User $testUserId has NO linked GUID!\n";
    echo "  Run: php smf-mohaa/link_identity_manual.php\n";
    $testGuid = 'test_guid_placeholder';
}
$smcFunc['db_free_result']($guidResult);

// Check PostgreSQL player_guid_registry
echo "\n[2] Checking PostgreSQL identity registry...\n";
$pgConn = pg_connect("host=localhost port=5432 dbname=mohaa_stats user=mohaa password=admin123");
if (!$pgConn) {
    die("✗ Cannot connect to PostgreSQL\n");
}

$pgResult = pg_query_params($pgConn, 'SELECT smf_member_id FROM player_guid_registry WHERE guid = $1', [$testGuid]);
if ($pgResult && pg_num_rows($pgResult) > 0) {
    $pgRow = pg_fetch_assoc($pgResult);
    echo "✓ GUID found in PostgreSQL: SMF ID = {$pgRow['smf_member_id']}\n";
} else {
    echo "⚠ GUID not in PostgreSQL player_guid_registry\n";
}

// Check ClickHouse raw_events
echo "\n[3] Checking ClickHouse events...\n";
$ch = curl_init('http://localhost:9000/?query=' . urlencode("SELECT COUNT(*) as count FROM mohaa_stats.raw_events WHERE actor_guid = '$testGuid'"));
curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
$chCount = curl_exec($ch);
$httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

if ($httpCode === 200) {
    echo "✓ ClickHouse events for GUID: " . trim($chCount) . "\n";
} else {
    echo "✗ ClickHouse query failed (HTTP $httpCode)\n";
}

// Check wins/losses calculation
echo "\n[4] Checking wins/losses data...\n";

// Query raw_events for team_win events where actor participated
$killsQuery = curl_init('http://localhost:9000/?query=' . urlencode("
    SELECT COUNT(*) as kills
    FROM mohaa_stats.raw_events
    WHERE actor_guid = '$testGuid'
    AND event_type = 'kill'
"));
curl_setopt($killsQuery, CURLOPT_RETURNTRANSFER, true);
$killsCount = trim(curl_exec($killsQuery));
curl_close($killsQuery);

echo "  Total kills in ClickHouse: $killsCount\n";

// Check if team_win events exist
$winsQuery = curl_init('http://localhost:9000/?query=' . urlencode("
    SELECT event_type, COUNT(*) as count
    FROM mohaa_stats.raw_events
    WHERE event_type IN ('team_win', 'round_end', 'match_end')
    GROUP BY event_type
"));
curl_setopt($winsQuery, CURLOPT_RETURNTRANSFER, true);
$winsData = curl_exec($winsQuery);
curl_close($winsQuery);

echo "  Win/Loss events:\n";
echo $winsData ? "    " . str_replace("\n", "\n    ", trim($winsData)) . "\n" : "    No win/loss events found\n";

// Test API endpoints
echo "\n[5] Testing API endpoints...\n";

$endpoints = [
    '/stats/player/' . urlencode($testGuid),
    '/stats/player/' . urlencode($testGuid) . '/weapons',
    '/stats/player/' . urlencode($testGuid) . '/maps',
    '/stats/player/' . urlencode($testGuid) . '/gametypes',
    '/stats/global',
    '/achievements/player/' . urlencode($testGuid),
];

foreach ($endpoints as $endpoint) {
    $ch = curl_init($apiBase . $endpoint);
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
    curl_setopt($ch, CURLOPT_TIMEOUT, 5);
    
    $response = curl_exec($ch);
    $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    curl_close($ch);
    
    if ($httpCode === 200) {
        $data = json_decode($response, true);
        if ($data) {
            echo "  ✓ $endpoint (HTTP $httpCode, " . strlen($response) . " bytes)\n";
        } else {
            echo "  ⚠ $endpoint (HTTP $httpCode, invalid JSON)\n";
        }
    } else {
        echo "  ✗ $endpoint (HTTP $httpCode)\n";
    }
}

// Check achievement unlocks
echo "\n[6] Checking achievements...\n";
$achQuery = $smcFunc['db_query']('', '
    SELECT COUNT(*) as unlocked
    FROM {db_prefix}mohaa_player_achievements
    WHERE smf_member_id = {int:member}
    AND unlocked = 1',
    ['member' => $testUserId]
);

if ($achQuery) {
    $achRow = $smcFunc['db_fetch_assoc']($achQuery);
    echo "  Achievements unlocked: {$achRow['unlocked']}\n";
    $smcFunc['db_free_result']($achQuery);
}

// Summary
echo "\n==================================================\n";
echo "FIX RECOMMENDATIONS\n";
echo "==================================================\n";

if (empty($testGuid) || $testGuid === 'test_guid_placeholder') {
    echo "1. Link user identity:\n";
    echo "   cd /home/elgan/dev/opm-stats-system\n";
    echo "   php smf-mohaa/link_identity_manual.php\n\n";
}

if (trim($killsCount) === '0') {
    echo "2. Seed event data:\n";
    echo "   cd /home/elgan/dev/opm-stats-system\n";
    echo "   ./bin/seeder -player-guid=\"$testGuid\" -events=2000\n\n";
}

if (strpos($winsData, 'team_win') === false) {
    echo "3. Add win/loss events:\n";
    echo "   Missing team_win, round_end, or match_end events\n";
    echo "   These are needed to calculate wins/losses\n\n";
}

echo "4. Test achievement system:\n";
echo "   curl -H 'Authorization: dev-seed-token' -X POST http://localhost:8080/api/v1/ingest/events \\\n";
echo "     -d 'event_type=kill&attacker_guid=$testGuid&attacker_smf_id=$testUserId&timestamp=" . time() . "'\n\n";

pg_close($pgConn);
?>
