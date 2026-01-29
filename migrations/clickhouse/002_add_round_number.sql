-- Add round_number to raw_events table
ALTER TABLE mohaa_stats.raw_events ADD COLUMN IF NOT EXISTS round_number UInt16 DEFAULT 0 AFTER match_outcome;
