#!/usr/bin/env php
<?php
/**
 * Link elgan's SMF account to their game GUID
 */

// SMF Database connection (using same credentials as SMF forum)
$db_server = 'localhost';
$db_name = 'smf';
$db_user = 'root';  // Adjust if needed
$db_passwd = '';    // Adjust if needed

$smf_member_id = 1;  // elgan's SMF account ID
$player_guid = '72750883-29ae-4377-85c4-9367f1f89d1a';  // elgan's game GUID
$player_name = 'elgan';

try {
    $db = new mysqli($db_server, $db_user, $db_passwd, $db_name);
    
    if ($db->connect_error) {
        die("Connection failed: " . $db->connect_error . "\n");
    }
    
    // Check if table exists
    $result = $db->query("SHOW TABLES LIKE 'smf_mohaa_identities'");
    if ($result->num_rows == 0) {
        echo "Table smf_mohaa_identities does not exist. Creating it...\n";
        
        $createTable = "
        CREATE TABLE IF NOT EXISTS smf_mohaa_identities (
            id INT AUTO_INCREMENT PRIMARY KEY,
            id_member INT NOT NULL,
            player_guid VARCHAR(36) NOT NULL,
            player_name VARCHAR(100) NOT NULL,
            linked_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            verified TINYINT(1) DEFAULT 1,
            UNIQUE KEY unique_member (id_member),
            UNIQUE KEY unique_guid (player_guid),
            KEY idx_member (id_member)
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
        ";
        
        if (!$db->query($createTable)) {
            die("Failed to create table: " . $db->error . "\n");
        }
        
        echo "✓ Table created\n";
    }
    
    // Check if already linked
    $check = $db->prepare("SELECT * FROM smf_mohaa_identities WHERE id_member = ? OR player_guid = ?");
    $check->bind_param('is', $smf_member_id, $player_guid);
    $check->execute();
    $existing = $check->get_result()->fetch_assoc();
    
    if ($existing) {
        echo "Already linked:\n";
        print_r($existing);
        echo "\nUpdating...\n";
        
        $stmt = $db->prepare("
            UPDATE smf_mohaa_identities 
            SET player_guid = ?, player_name = ?, linked_date = NOW() 
            WHERE id_member = ?
        ");
        $stmt->bind_param('ssi', $player_guid, $player_name, $smf_member_id);
    } else {
        echo "Creating new link...\n";
        $stmt = $db->prepare("
            INSERT INTO smf_mohaa_identities (id_member, player_guid, player_name, verified)
            VALUES (?, ?, ?, 1)
        ");
        $stmt->bind_param('iss', $smf_member_id, $player_guid, $player_name);
    }
    
    if ($stmt->execute()) {
        echo "✓ Successfully linked SMF member {$smf_member_id} to GUID {$player_guid}\n";
    } else {
        echo "✗ Failed: " . $stmt->error . "\n";
    }
    
    $db->close();
    
} catch (Exception $e) {
    die("Error: " . $e->getMessage() . "\n");
}
