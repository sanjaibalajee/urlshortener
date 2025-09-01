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


ALTER TABLE urls 
ADD CONSTRAINT urls_not_reserved_check 
CHECK (short_code NOT IN (SELECT code FROM reserved_codes));
