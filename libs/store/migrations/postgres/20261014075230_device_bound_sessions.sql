-- Sessions without a device key are dropped: their devices sign in again.
DELETE FROM "auth_sessions";
-- Modify "auth_sessions" table
ALTER TABLE "auth_sessions" DROP COLUMN "token_hash", ADD COLUMN "device_key" bytea NOT NULL;
