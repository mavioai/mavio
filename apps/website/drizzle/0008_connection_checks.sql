ALTER TABLE "servers" ADD COLUMN "connection_checks" jsonb DEFAULT '[]'::jsonb NOT NULL;--> statement-breakpoint
ALTER TABLE "servers" ADD COLUMN "connections_checked_at" timestamp with time zone;