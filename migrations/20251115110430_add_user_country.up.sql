-- modify "events_user_signup" table
ALTER TABLE "public"."events_user_signup" ADD COLUMN "country" character varying(255) NULL;
