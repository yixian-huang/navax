-- Optional CNAME/custom host alias for approved subdomains.
ALTER TABLE subdomain_requests ADD COLUMN custom_domain TEXT COLLATE NOCASE
    CHECK (custom_domain IS NULL OR (length(custom_domain) BETWEEN 3 AND 253));

CREATE UNIQUE INDEX idx_subdomain_custom_domain_active
    ON subdomain_requests(custom_domain)
    WHERE custom_domain IS NOT NULL AND status IN ('pending', 'approved');
