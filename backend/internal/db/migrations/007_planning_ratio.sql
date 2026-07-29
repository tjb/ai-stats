ALTER TABLE tool_usage_snapshots
  ADD COLUMN IF NOT EXISTS total_planning_uses INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS total_coding_uses   INT NOT NULL DEFAULT 0;
