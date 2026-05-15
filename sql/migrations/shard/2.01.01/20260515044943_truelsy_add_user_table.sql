-- migration: add_user_table
-- author:    truelsy
-- created:   2026-05-15T04:49:43Z UTC

-- +goose Up
CREATE TABLE `TB_USER`
(
    `user_id`          BIGINT UNSIGNED NOT NULL,
    `hive_img_url`     VARCHAR(200)    NOT NULL,
    `client_version`   VARCHAR(10)     NOT NULL,
    `server_version`   VARCHAR(10)     NOT NULL,
    `language`         VARCHAR(10)     NOT NULL,
    `platform`         VARCHAR(10)     NOT NULL,
    `app_id`           VARCHAR(100)    NOT NULL,
    `last_login_time`  INT UNSIGNED    NOT NULL COMMENT '마지막 GameLogin 한 시간',
    `country`          VARCHAR(10)     NOT NULL,
    `device_name`      VARCHAR(20)     NOT NULL,
    `os_version`       VARCHAR(30)     NOT NULL,
    `insert_time` TIMESTAMP NOT NULL COMMENT '데이터 최초 생성일' DEFAULT CURRENT_TIMESTAMP,
    `update_time` TIMESTAMP NOT NULL COMMENT '데이터 갱신일' DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`user_id`) USING BTREE,
    INDEX `last_login_time` (`last_login_time`) USING BTREE
) COMMENT ='유저 정보 테이블'
    COLLATE = 'utf8mb4_unicode_ci'
ENGINE=InnoDB;


-- +goose Down
DROP TABLE IF EXISTS `TB_USER`;
