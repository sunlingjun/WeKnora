-- Migration: 000014_update_storage_quota_default
-- Description: Update storage quota default from 10GB to 1GB
DO $$ BEGIN RAISE NOTICE '[Migration 000014] Updating storage quota default from 10GB to 1GB...'; END $$;

-- Update the default value for storage_quota column
-- 1GB = 1073741824 bytes
ALTER TABLE tenants ALTER COLUMN storage_quota SET DEFAULT 1073741824;

-- Update existing tenants that have the old default value (10GB) to new default (1GB)
-- Only update tenants that have exactly 10GB quota and haven't been customized
UPDATE tenants 
SET storage_quota = 1073741824 
WHERE storage_quota = 10737418240 
  AND deleted_at IS NULL;

DO $$ BEGIN RAISE NOTICE '[Migration 000014] Storage quota default updated successfully'; END $$;
