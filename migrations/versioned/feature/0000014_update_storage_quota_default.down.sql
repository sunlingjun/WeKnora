-- Migration: 000014_update_storage_quota_default (Rollback)
-- Description: Rollback storage quota default from 1GB to 10GB
DO $$ BEGIN RAISE NOTICE '[Migration 000014] Rolling back storage quota default from 1GB to 10GB...'; END $$;

-- Restore the default value for storage_quota column
-- 10GB = 10737418240 bytes
ALTER TABLE tenants ALTER COLUMN storage_quota SET DEFAULT 10737418240;

-- Note: We don't rollback the updated tenant quotas as we can't distinguish
-- between tenants that were updated by this migration and those that were
-- manually set to 1GB

DO $$ BEGIN RAISE NOTICE '[Migration 000014] Storage quota default rollback completed'; END $$;
