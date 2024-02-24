ALTER TABLE tag DROP COLUMN revision;
ALTER TABLE tag ADD updated_at_ms INTEGER;