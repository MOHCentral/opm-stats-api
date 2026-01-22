-- Add max_players column to servers table
ALTER TABLE servers ADD COLUMN IF NOT EXISTS max_players INTEGER DEFAULT 32;
