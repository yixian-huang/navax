ALTER TABLE analytics_events
    ADD COLUMN browser TEXT NOT NULL DEFAULT '' CHECK (length(browser) <= 40);
