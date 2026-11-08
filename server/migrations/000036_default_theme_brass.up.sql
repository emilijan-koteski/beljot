-- New accounts start in brass mode, the dark felt-and-brass look. Existing
-- players keep the theme stored on their row; the app sets theme_preference
-- explicitly on registration, so this default only covers other inserts.
ALTER TABLE users ALTER COLUMN theme_preference SET DEFAULT 'brass';
