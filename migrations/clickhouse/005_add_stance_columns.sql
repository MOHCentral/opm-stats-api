ALTER TABLE mohaa_stats.raw_events ADD COLUMN IF NOT EXISTS actor_stance LowCardinality(String) AFTER actor_yaw;
ALTER TABLE mohaa_stats.raw_events ADD COLUMN IF NOT EXISTS target_stance LowCardinality(String) AFTER target_pos_z;
