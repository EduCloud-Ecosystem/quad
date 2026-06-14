-- Indexes for the student feedback loop: webhook repo lookup and a student's
-- own-work query (roster join on host + username).
CREATE INDEX IF NOT EXISTS idx_roster_host_username ON roster_entries (host, host_username);
CREATE INDEX IF NOT EXISTS idx_submissions_repo ON submissions (repo_host, repo_namespace, repo_name);
