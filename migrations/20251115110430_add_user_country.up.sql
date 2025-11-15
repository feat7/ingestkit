-- modify "events_user_signup" table
ALTER TABLE IF EXISTS "public"."events_user_signup" ADD COLUMN "country" character varying(255) NULL;
