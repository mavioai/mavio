CREATE TABLE "authorization_requests" (
	"id" text PRIMARY KEY NOT NULL,
	"server_id" text NOT NULL,
	"public_key" "bytea" NOT NULL,
	"name" text DEFAULT '' NOT NULL,
	"version" text DEFAULT '' NOT NULL,
	"redirect_uri" text NOT NULL,
	"code_challenge" text NOT NULL,
	"device_key" "bytea" NOT NULL,
	"state" text DEFAULT '' NOT NULL,
	"claim" boolean DEFAULT false NOT NULL,
	"account_id" text,
	"declined" boolean DEFAULT false NOT NULL,
	"expires_at" timestamp with time zone NOT NULL
);
--> statement-breakpoint
CREATE TABLE "device_sessions" (
	"id" text PRIMARY KEY NOT NULL,
	"account_id" text NOT NULL,
	"client_id" text NOT NULL,
	"device_key" "bytea" NOT NULL,
	"renewed_at" bigint DEFAULT 0 NOT NULL,
	"created_at" timestamp with time zone NOT NULL,
	"last_seen_at" timestamp with time zone NOT NULL
);
--> statement-breakpoint
CREATE TABLE "link_codes" (
	"device_code_hash" text PRIMARY KEY NOT NULL,
	"user_code" text NOT NULL,
	"client_id" text NOT NULL,
	"device_key" "bytea" NOT NULL,
	"account_id" text,
	"denied" boolean DEFAULT false NOT NULL,
	"polled_at" timestamp with time zone,
	"expires_at" timestamp with time zone NOT NULL,
	CONSTRAINT "link_codes_user_code_unique" UNIQUE("user_code")
);
--> statement-breakpoint
ALTER TABLE "device_code" DISABLE ROW LEVEL SECURITY;--> statement-breakpoint
DROP TABLE "device_code" CASCADE;--> statement-breakpoint
ALTER TABLE "authorization_codes" ALTER COLUMN "server_id" DROP NOT NULL;--> statement-breakpoint
ALTER TABLE "servers" ADD COLUMN "connections" jsonb DEFAULT '[]'::jsonb NOT NULL;--> statement-breakpoint
ALTER TABLE "authorization_requests" ADD CONSTRAINT "authorization_requests_account_id_user_id_fk" FOREIGN KEY ("account_id") REFERENCES "public"."user"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "device_sessions" ADD CONSTRAINT "device_sessions_account_id_user_id_fk" FOREIGN KEY ("account_id") REFERENCES "public"."user"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "link_codes" ADD CONSTRAINT "link_codes_account_id_user_id_fk" FOREIGN KEY ("account_id") REFERENCES "public"."user"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "authorization_requests_expires_at_idx" ON "authorization_requests" USING btree ("expires_at");--> statement-breakpoint
CREATE INDEX "device_sessions_account_id_idx" ON "device_sessions" USING btree ("account_id");--> statement-breakpoint
CREATE INDEX "link_codes_expires_at_idx" ON "link_codes" USING btree ("expires_at");