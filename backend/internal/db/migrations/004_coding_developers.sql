-- Migration 004: Add total_coding_developers to track all committers per day.
-- This column is the denominator for the utilization metric:
--   utilization = total_active_users (AI tool users) / total_coding_developers (all who committed)

ALTER TABLE tool_usage_snapshots
    ADD COLUMN IF NOT EXISTS total_coding_developers INT NOT NULL DEFAULT 0;
