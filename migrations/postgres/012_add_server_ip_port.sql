ALTER TABLE servers ADD COLUMN IF NOT EXISTS ip_address VARCHAR(45);
ALTER TABLE servers ADD COLUMN IF NOT EXISTS port INTEGER;

-- Add unique constraint to prevent duplicate servers on same IP:Port
-- (Ignore if it already exists or handle conflicts gracefully in application)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'unique_server_address') THEN
        ALTER TABLE servers ADD CONSTRAINT unique_server_address UNIQUE (ip_address, port);
    END IF;
END $$;
