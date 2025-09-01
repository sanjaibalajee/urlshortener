

ALTER TABLE urls DROP CONSTRAINT IF EXISTS urls_not_reserved_check;

DROP TABLE IF EXISTS click_events;
DROP TABLE IF EXISTS url_counters_live;
DROP TABLE IF EXISTS reserved_codes;
DROP TABLE IF EXISTS urls;
