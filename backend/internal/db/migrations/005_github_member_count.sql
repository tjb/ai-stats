ALTER TABLE organizations
  ADD COLUMN IF NOT EXISTS github_member_count INT;
