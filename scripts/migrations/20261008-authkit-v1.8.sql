-- authkit v1.4 -> v1.8: display names no longer identify password credentials.
-- Run only on the intended, backed-up database while authentication writes are stopped.
-- Inspect the result first: the v1.4 Model names its unique username index `username`.
-- A fresh v1.8 database has no such index and does not need this migration.
SHOW INDEX FROM auth_accounts WHERE Key_name = 'username';

-- Execute only after confirming this is the old unique index on username.
-- MySQL DDL commits independently; this removes a constraint, not account data.
ALTER TABLE auth_accounts DROP INDEX username;
