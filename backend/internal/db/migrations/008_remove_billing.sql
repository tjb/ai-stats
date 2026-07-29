-- Migration 008: Remove Stripe billing and seat-limit paywall.
-- AI Stats is self-hosted and open source — there is no billing tier to enforce.

ALTER TABLE users
  DROP COLUMN IF EXISTS stripe_customer_id;

ALTER TABLE organizations
  DROP COLUMN IF EXISTS plan,
  DROP COLUMN IF EXISTS stripe_subscription_id;
