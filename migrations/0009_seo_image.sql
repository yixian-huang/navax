-- Dedicated SEO / Open Graph share image (optional; falls back to background on publish).

ALTER TABLE page_publications
    ADD COLUMN seo_image TEXT NOT NULL DEFAULT '' CHECK (length(seo_image) <= 2048);
