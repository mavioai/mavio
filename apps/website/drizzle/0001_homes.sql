CREATE TABLE "friendships" (
	"requester_id" text NOT NULL,
	"addressee_id" text NOT NULL,
	"accepted" boolean NOT NULL,
	"created_at" timestamp with time zone NOT NULL,
	CONSTRAINT "friendships_requester_id_addressee_id_pk" PRIMARY KEY("requester_id","addressee_id")
);
--> statement-breakpoint
CREATE TABLE "home_invitations" (
	"home_id" text NOT NULL,
	"account_id" text NOT NULL,
	"offers" jsonb NOT NULL,
	"created_at" timestamp with time zone NOT NULL,
	CONSTRAINT "home_invitations_home_id_account_id_pk" PRIMARY KEY("home_id","account_id")
);
--> statement-breakpoint
CREATE TABLE "home_members" (
	"account_id" text PRIMARY KEY NOT NULL,
	"home_id" text NOT NULL,
	"joined_at" timestamp with time zone NOT NULL
);
--> statement-breakpoint
CREATE TABLE "homes" (
	"id" text PRIMARY KEY NOT NULL,
	"name" text NOT NULL,
	"administrator_id" text NOT NULL,
	"created_at" timestamp with time zone NOT NULL,
	CONSTRAINT "homes_administrator_id_unique" UNIQUE("administrator_id")
);
--> statement-breakpoint
CREATE TABLE "passes" (
	"account_id" text PRIMARY KEY NOT NULL,
	"expires_at" timestamp with time zone NOT NULL
);
--> statement-breakpoint
CREATE TABLE "pins" (
	"account_id" text PRIMARY KEY NOT NULL,
	"hash" text NOT NULL,
	"failures" integer DEFAULT 0 NOT NULL,
	"retry_at" timestamp with time zone
);
--> statement-breakpoint
ALTER TABLE "user" ADD COLUMN "kind" text DEFAULT 'full';--> statement-breakpoint
ALTER TABLE "user" ADD COLUMN "restriction" integer DEFAULT 0;--> statement-breakpoint
ALTER TABLE "friendships" ADD CONSTRAINT "friendships_requester_id_user_id_fk" FOREIGN KEY ("requester_id") REFERENCES "public"."user"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "friendships" ADD CONSTRAINT "friendships_addressee_id_user_id_fk" FOREIGN KEY ("addressee_id") REFERENCES "public"."user"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "home_invitations" ADD CONSTRAINT "home_invitations_home_id_homes_id_fk" FOREIGN KEY ("home_id") REFERENCES "public"."homes"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "home_invitations" ADD CONSTRAINT "home_invitations_account_id_user_id_fk" FOREIGN KEY ("account_id") REFERENCES "public"."user"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "home_members" ADD CONSTRAINT "home_members_account_id_user_id_fk" FOREIGN KEY ("account_id") REFERENCES "public"."user"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "home_members" ADD CONSTRAINT "home_members_home_id_homes_id_fk" FOREIGN KEY ("home_id") REFERENCES "public"."homes"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "homes" ADD CONSTRAINT "homes_administrator_id_user_id_fk" FOREIGN KEY ("administrator_id") REFERENCES "public"."user"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "passes" ADD CONSTRAINT "passes_account_id_user_id_fk" FOREIGN KEY ("account_id") REFERENCES "public"."user"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "pins" ADD CONSTRAINT "pins_account_id_user_id_fk" FOREIGN KEY ("account_id") REFERENCES "public"."user"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "friendships_addressee_id_idx" ON "friendships" USING btree ("addressee_id");--> statement-breakpoint
CREATE INDEX "home_invitations_account_id_idx" ON "home_invitations" USING btree ("account_id");--> statement-breakpoint
CREATE INDEX "home_members_home_id_idx" ON "home_members" USING btree ("home_id");