#!/usr/bin/env php
<?php
/**
 * SMF Stats Audit Tool
 * 
 * Tests every stat endpoint, every tab, every chart to identify broken functionality
 * Usage: php tools/audit_smf_stats.php
 */

// Configuration
$SMF_BASE = 'http://localhost:8888';
$API_BASE = 'http://localhost:8080';
$TEST_USER_ID = 1; // SMF member ID to test
$OUTPUT_FILE = __DIR__ . '/../logs/smf_audit_' . date('Y-m-d_H-i-s') . '.log';

// ANSI colors
$RED = "\033[31m";
$GREEN = "\033[32m";
$YELLOW = "\033[33m";
$BLUE = "\033[34m";
$RESET = "\033[0m";

// Results tracking
$results = [
    'total' => 0,
    'passed' => 0,
    'failed' => 0,
    'warnings' => 0,
    'errors' => []
];

function log_output($message, $color = '') {
    global $RESET, $OUTPUT_FILE;
    echo $color . $message . $RESET . "\n";
    file_put_contents($OUTPUT_FILE, strip_tags($message) . "\n", FILE_APPEND);
}

function test_endpoint($name, $url, $checks = []) {
    global $results, $RED, $GREEN, $YELLOW, $BLUE;
    
    $results['total']++;
    log_output("\n[TEST] $name", $BLUE);
    log_output("URL: $url");
    
    $ch = curl_init($url);
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
    curl_setopt($ch, CURLOPT_TIMEOUT, 10);
    curl_setopt($ch, CURLOPT_FOLLOWLOCATION, true);
    
    $response = curl_exec($ch);
    $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    $error = curl_error($ch);
    curl_close($ch);
    
    if ($error) {
        $results['failed']++;
        $results['errors'][] = "$name: cURL Error - $error";
        log_output("❌ FAILED: $error", $RED);
        return false;
    }
    
    if ($httpCode !== 200) {
        $results['failed']++;
        $results['errors'][] = "$name: HTTP $httpCode";
        log_output("❌ FAILED: HTTP $httpCode", $RED);
        return false;
    }
    
    // Run custom checks
    $checksPassed = true;
    foreach ($checks as $checkName => $checkFunc) {
        $checkResult = $checkFunc($response);
        if ($checkResult === true) {
            log_output("  ✓ $checkName", $GREEN);
        } elseif ($checkResult === null) {
            $results['warnings']++;
            log_output("  ⚠ $checkName: Warning", $YELLOW);
        } else {
            $checksPassed = false;
            $results['errors'][] = "$name > $checkName: $checkResult";
            log_output("  ✗ $checkName: $checkResult", $RED);
        }
    }
    
    if ($checksPassed) {
        $results['passed']++;
        log_output("✅ PASSED", $GREEN);
        return true;
    } else {
        $results['failed']++;
        log_output("❌ FAILED", $RED);
        return false;
    }
}

function test_api_endpoint($name, $endpoint, $checks = []) {
    global $API_BASE;
    return test_endpoint($name, $API_BASE . $endpoint, $checks);
}

function test_smf_page($name, $action, $checks = []) {
    global $SMF_BASE;
    return test_endpoint($name, $SMF_BASE . '/index.php?action=' . $action, $checks);
}

// Common checks
$hasContent = function($html) {
    if (strlen($html) < 100) return "Response too short (". strlen($html) ." bytes)";
    if (stripos($html, 'error') !== false && stripos($html, 'An Error Has Occurred') !== false) return "SMF error page detected";
    return true;
};

$notEmpty = function($html) use ($hasContent) {
    $result = $hasContent($html);
    if ($result !== true) return $result;
    if (stripos($html, 'No data available') !== false) return null; // Warning
    if (preg_match('/>\s*0\s*</', $html)) return null; // Might be zero value
    return true;
};

$hasChart = function($html) {
    if (stripos($html, 'apexcharts') === false && stripos($html, 'chart') === false) {
        return "No chart elements found";
    }
    return true;
};

$hasValidJson = function($response) {
    $data = json_decode($response, true);
    if (json_last_error() !== JSON_ERROR_NONE) return "Invalid JSON: " . json_last_error_msg();
    return true;
};

// ===========================
// API HEALTH CHECKS
// ===========================
log_output("\n" . str_repeat("=", 60), $BLUE);
log_output("API HEALTH CHECKS", $BLUE);
log_output(str_repeat("=", 60), $BLUE);

test_api_endpoint('API Health', '/health', [
    'Valid JSON' => $hasValidJson,
    'Status OK' => function($r) {
        $data = json_decode($r, true);
        return isset($data['status']) && $data['status'] === 'healthy' ? true : "Status not healthy";
    }
]);

// ===========================
// SMF DASHBOARD CHECKS  
// ===========================
log_output("\n" . str_repeat("=", 60), $BLUE);
log_output("SMF DASHBOARD CHECKS", $BLUE);
log_output(str_repeat("=", 60), $BLUE);

test_smf_page('MOHAA Dashboard', 'mohaadashboard', [
    'Has Content' => $hasContent,
    'Has Stats' => function($html) {
        if (stripos($html, 'KILLS') === false) return "Missing KILLS stat";
        if (stripos($html, 'DEATHS') === false) return "Missing DEATHS stat";
        if (stripos($html, 'K/D') === false) return "Missing K/D stat";
        return true;
    },
    'Has Wins' => function($html) {
        // Check if wins is actually populated (not just showing 0)
        if (preg_match('/WINS.*?>(\d+)</i', $html, $matches)) {
            $wins = (int)$matches[1];
            if ($wins === 0) return null; // Warning - might be legitimate zero
            return true;
        }
        return "Could not find WINS stat";
    }
]);

// ===========================
// PLAYER PROFILE TABS
// ===========================
log_output("\n" . str_repeat("=", 60), $BLUE);
log_output("PLAYER PROFILE TABS", $BLUE);
log_output(str_repeat("=", 60), $BLUE);

$tabs = [
    'combat' => ['Has Chart' => $hasChart, 'Has Stats' => $notEmpty],
    'signature' => ['Has Content' => $hasContent],
    'armoury' => ['Has Weapons' => $notEmpty],
    'movement' => ['Has Distance Stats' => $notEmpty],
    'game' => ['Has Game Stats' => $notEmpty],
    'gametypes' => ['Has Game Types' => $notEmpty],
    'interaction' => ['Has Interaction Stats' => $notEmpty],
    'maps' => ['Has Map Stats' => $notEmpty],
    'matches' => ['Has Match History' => $notEmpty],
    'medals' => ['Has Achievements' => $notEmpty]
];

foreach ($tabs as $tab => $checks) {
    test_smf_page("Profile Tab: $tab", "mohaaplayer;u=1;tab=$tab", $checks);
}

// ===========================
// LEADERBOARDS
// ===========================
log_output("\n" . str_repeat("=", 60), $BLUE);
log_output("LEADERBOARD CHECKS", $BLUE);
log_output(str_repeat("=", 60), $BLUE);

test_smf_page('Leaderboard: Overall', 'mohaaleaderboard', [
    'Has Content' => $hasContent,
    'Has Players' => function($html) {
        if (stripos($html, 'elgan') === false) return "Test user not in leaderboard";
        return true;
    }
]);

test_smf_page('Leaderboard: Weapons', 'mohaaleaderboard;type=weapons', ['Has Content' => $notEmpty]);
test_smf_page('Leaderboard: Maps', 'mohaaleaderboard;type=maps', ['Has Content' => $notEmpty]);
test_smf_page('Leaderboard: Achievements', 'mohaaleaderboard;type=achievements', ['Has Content' => $notEmpty]);

// ===========================
// WEAPONS & MAPS
// ===========================
log_output("\n" . str_repeat("=", 60), $BLUE);
log_output("WEAPONS & MAPS CHECKS", $BLUE);
log_output(str_repeat("=", 60), $BLUE);

test_smf_page('Weapons Overview', 'mohaaweapons', ['Has Content' => $notEmpty]);
test_smf_page('Maps Overview', 'mohaamaps', ['Has Content' => $notEmpty]);

// ===========================
// SERVERS
// ===========================
log_output("\n" . str_repeat("=", 60), $BLUE);
log_output("SERVER CHECKS", $BLUE);
log_output(str_repeat("=", 60), $BLUE);

test_smf_page('Server Dashboard', 'mohaaserverdashboard', ['Has Content' => $notEmpty]);

// ===========================
// ACHIEVEMENTS
// ===========================
log_output("\n" . str_repeat("=", 60), $BLUE);
log_output("ACHIEVEMENT CHECKS", $BLUE);
log_output(str_repeat("=", 60), $BLUE);

test_smf_page('Achievements List', 'mohaaachievements', [
    'Has Content' => $hasContent,
    'Has Achievements' => function($html) {
        if (stripos($html, 'achievement') === false) return "No achievements found";
        return true;
    }
]);

// ===========================
// SUMMARY REPORT
// ===========================
log_output("\n" . str_repeat("=", 60), $GREEN);
log_output("AUDIT SUMMARY", $GREEN);
log_output(str_repeat("=", 60), $GREEN);

$passRate = $results['total'] > 0 ? round(($results['passed'] / $results['total']) * 100, 1) : 0;

log_output("Total Tests: " . $results['total']);
log_output("Passed: " . $results['passed'], $GREEN);
log_output("Failed: " . $results['failed'], $results['failed'] > 0 ? $RED : $GREEN);
log_output("Warnings: " . $results['warnings'], $results['warnings'] > 0 ? $YELLOW : $GREEN);
log_output("Pass Rate: {$passRate}%", $passRate >= 80 ? $GREEN : ($passRate >= 50 ? $YELLOW : $RED));

if (!empty($results['errors'])) {
    log_output("\n" . str_repeat("=", 60), $RED);
    log_output("ERRORS FOUND", $RED);
    log_output(str_repeat("=", 60), $RED);
    foreach ($results['errors'] as $i => $error) {
        log_output(($i + 1) . ". $error", $RED);
    }
}

log_output("\nFull log saved to: $OUTPUT_FILE");

exit($results['failed'] > 0 ? 1 : 0);
