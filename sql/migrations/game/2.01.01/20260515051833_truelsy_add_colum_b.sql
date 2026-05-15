-- migration: add_colum_b
-- author:    truelsy
-- created:   2026-05-15T05:18:33Z UTC

-- +goose Up
ALTER TABLE `TB_ACCOUNT` ADD COLUMN `b` int NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE `TB_ACCOUNT` DROP COLUMN `b`;
