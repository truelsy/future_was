-- migration: add_white_device
-- author:    mega
-- created:   2026-05-15T07:17:32Z UTC

-- +goose Up
CREATE TABLE `TB_SERVER_MAINTENANCE_WHITE_DEVICE`
(
    `idx`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `device_id`   VARCHAR(20)  NOT NULL COMMENT '하이브에서 발급한 디바이스 id' COLLATE 'utf8mb4_unicode_ci',
    `comment`     VARCHAR(100) NOT NULL COMMENT '계정 설명' COLLATE 'utf8mb4_unicode_ci',
    `insert_time` TIMESTAMP NOT NULL COMMENT '데이터 최초 생성일' DEFAULT CURRENT_TIMESTAMP,
    `update_time` TIMESTAMP NOT NULL COMMENT '데이터 갱신일' DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`idx`) USING BTREE,
    UNIQUE INDEX `device_id` (`device_id`) USING BTREE
) COMMENT='서버 점검 관련 화이트 유저 정보 테이블'
COLLATE='utf8mb4_unicode_ci'
ENGINE=InnoDB;


-- +goose Down
DROP TABLE IF EXISTS `TB_SERVER_MAINTENANCE_WHITE_DEVICE`;

