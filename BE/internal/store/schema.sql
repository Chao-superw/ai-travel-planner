CREATE TABLE IF NOT EXISTS users (id text PRIMARY KEY, username text UNIQUE NOT NULL, password_hash text NOT NULL, role text NOT NULL CHECK(role IN ('user','admin')), active boolean NOT NULL DEFAULT true, created_at timestamptz NOT NULL DEFAULT now());

-- Additive email-auth migration. Existing identity, password and trip rows survive.
ALTER TABLE users ADD COLUMN IF NOT EXISTS email text;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_key text;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at timestamptz;
CREATE UNIQUE INDEX IF NOT EXISTS users_email_key_unique ON users(email_key) WHERE email_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS sessions (token_hash text PRIMARY KEY, user_id text NOT NULL REFERENCES users(id), expires_at timestamptz NOT NULL, revoked_at timestamptz);
CREATE TABLE IF NOT EXISTS places (id text PRIMARY KEY, provider text NOT NULL, provider_poi_id text NOT NULL, data jsonb NOT NULL, curated boolean NOT NULL DEFAULT false, version bigint NOT NULL DEFAULT 1, UNIQUE(provider,provider_poi_id));
CREATE TABLE IF NOT EXISTS trips (id text PRIMARY KEY, owner_id text NOT NULL REFERENCES users(id), current_version bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS planning_jobs (id text PRIMARY KEY, owner_id text NOT NULL REFERENCES users(id), kind text NOT NULL CHECK(kind IN ('generate','replan','manual_edit')), operation text NOT NULL, idem_key text NOT NULL, request_hash text NOT NULL, input jsonb NOT NULL, base_snapshot jsonb, candidates jsonb NOT NULL DEFAULT '[]', public jsonb NOT NULL, status text NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','running','succeeded','failed','conflicted','interrupted')), stage text NOT NULL DEFAULT 'queued', attempt_id text NOT NULL DEFAULT '', lease_expires_at timestamptz, deadline_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), trip_id text NOT NULL DEFAULT '', result_version bigint NOT NULL DEFAULT 0, error jsonb, model_runs jsonb NOT NULL DEFAULT '[]', UNIQUE(owner_id,operation,idem_key));
CREATE UNIQUE INDEX IF NOT EXISTS one_active_ai_per_user ON planning_jobs(owner_id) WHERE status IN ('queued','running') AND kind IN ('generate','replan');
CREATE UNIQUE INDEX IF NOT EXISTS one_active_edit_per_user ON planning_jobs(owner_id) WHERE status IN ('queued','running') AND kind='manual_edit';
CREATE INDEX IF NOT EXISTS jobs_queue ON planning_jobs(status,created_at);
CREATE TABLE IF NOT EXISTS trip_versions (trip_id text NOT NULL REFERENCES trips(id), version bigint NOT NULL, source_job_id text UNIQUE NOT NULL REFERENCES planning_jobs(id), data jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(trip_id,version));
CREATE TABLE IF NOT EXISTS trip_items (trip_id text NOT NULL, version bigint NOT NULL, item_id text NOT NULL, data jsonb NOT NULL, PRIMARY KEY(trip_id,version,item_id), FOREIGN KEY(trip_id,version) REFERENCES trip_versions(trip_id,version));
CREATE TABLE IF NOT EXISTS trip_routes (trip_id text NOT NULL, version bigint NOT NULL, from_item_id text NOT NULL, to_item_id text NOT NULL, data jsonb NOT NULL, PRIMARY KEY(trip_id,version,from_item_id,to_item_id), FOREIGN KEY(trip_id,version,from_item_id) REFERENCES trip_items(trip_id,version,item_id), FOREIGN KEY(trip_id,version,to_item_id) REFERENCES trip_items(trip_id,version,item_id));

CREATE TABLE IF NOT EXISTS auth_challenges (
 id text PRIMARY KEY, email_key text NOT NULL, purpose text NOT NULL CHECK(purpose IN ('register','bind')),
 user_id text NOT NULL DEFAULT '', code_mac text NOT NULL, status text NOT NULL CHECK(status IN ('sending','ready','failed','superseded','consumed')),
 attempts integer NOT NULL DEFAULT 0, max_attempts integer NOT NULL, created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 expires_at timestamptz NOT NULL, consumed_at timestamptz
);
CREATE INDEX IF NOT EXISTS auth_challenges_email_created ON auth_challenges(email_key,created_at DESC);
CREATE TABLE IF NOT EXISTS auth_rate_limits (key text PRIMARY KEY, attempts integer NOT NULL, resets_at timestamptz NOT NULL);
