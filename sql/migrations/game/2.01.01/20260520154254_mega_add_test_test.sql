-- migration: add_test_test
-- author:    mega
-- created:   2026-05-20 15:42:54 KST

-- +goose Up
ALTER TABLE TB_ACCOUNT ADD COLUMN last_seen BIGINT UNSIGNED NOT NULL DEFAULT 0;


-- +goose Down
ALTER TABLE TB_ACCOUNT DROP COLUMN last_seen;

