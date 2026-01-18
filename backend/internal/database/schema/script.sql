CREATE TABLE urls (
  id bigserial PRIMARY KEY,
  short_code text NOT NULL,
  target_url text NOT NULL,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz
);

CREATE UNIQUE INDEX urls_short_code_uniq ON urls (short_code);
CREATE INDEX urls_created_at_idx ON urls (created_at DESC);
CREATE INDEX urls_active_idx ON urls (is_active);
CREATE INDEX urls_expiry_idx ON urls (expires_at);

CREATE TABLE url_counters_live (
  url_id bigint NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
  shard_id smallint NOT NULL CHECK (shard_id BETWEEN 0 AND 63),
  clicks bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (url_id, shard_id)
);

CREATE TABLE click_events (
  id bigserial PRIMARY KEY,
  url_id bigint NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  ip inet,
  ua text,
  referrer text,
  utm_source text,
  utm_medium text,
  utm_campaign text,
  utm_term text,
  utm_content text,
  query_params jsonb
);

CREATE INDEX click_events_time_brin ON click_events USING brin (occurred_at);
CREATE INDEX click_events_url_time_idx ON click_events (url_id, occurred_at DESC);
CREATE INDEX click_events_utm_idx ON click_events (utm_source, utm_medium, utm_campaign, occurred_at DESC);


CREATE TABLE reserved_codes (
  code text PRIMARY KEY,
  reason text NOT NULL,
  description text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX reserved_codes_reason_idx ON reserved_codes (reason);


-- Enforce that no new URLs may use a reserved code via trigger instead of a CHECK constraint,
-- since PostgreSQL does not support subqueries in CHECK constraints.

-- Create a trigger function to check reserved codes on insert/update
CREATE OR REPLACE FUNCTION prevent_reserved_short_code()
RETURNS trigger AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM reserved_codes WHERE code = NEW.short_code) THEN
    RAISE EXCEPTION 'short_code "%" is reserved and cannot be used', NEW.short_code;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create the trigger on INSERT and UPDATE on 'urls'
CREATE TRIGGER urls_prevent_reserved_code_trigger
BEFORE INSERT OR UPDATE ON urls
FOR EACH ROW
EXECUTE FUNCTION prevent_reserved_short_code();

-- Performance indexes for analytics queries
CREATE INDEX IF NOT EXISTS click_events_url_date_idx
    ON click_events(url_id, DATE(occurred_at));
CREATE INDEX IF NOT EXISTS click_events_referrer_idx
    ON click_events(url_id, referrer) WHERE referrer IS NOT NULL;
CREATE INDEX IF NOT EXISTS click_events_ua_idx
    ON click_events(url_id, ua) WHERE ua IS NOT NULL;

