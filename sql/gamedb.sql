USE N_GAME;

DROP TABLE IF EXISTS `TB_ADD_SERVER_TIME`;
CREATE TABLE `TB_ADD_SERVER_TIME`
(
    `idx`                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `add_second`          BIGINT      NOT NULL COMMENT '더해진 시간을 초로 환산한 값',
    `last_edit_user_name` VARCHAR(20) NOT NULL DEFAULT '' COMMENT '마지막으로 수정한 유저이름' COLLATE 'utf8mb4_unicode_ci',
    `insert_time` TIMESTAMP NOT NULL COMMENT '데이터 최초 생성일' DEFAULT CURRENT_TIMESTAMP,
    `update_time` TIMESTAMP NOT NULL COMMENT '데이터 갱신일' DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`idx`) USING BTREE
) COMMENT='서버 시간 변경 정보 테이블'
COLLATE='utf8mb4_unicode_ci'
ENGINE=InnoDB;


DROP TABLE IF EXISTS `TB_ACCOUNT`;
CREATE TABLE `TB_ACCOUNT`
(
    `user_id`     BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '게임서버가 발급한 유저 id',
    `channel_uid` BIGINT UNSIGNED NOT NULL COMMENT '하이브에서 발급한 유저 id',
    `device_id`   VARCHAR(20) NOT NULL COMMENT '하이브에서 발급한 디바이스 id' COLLATE 'utf8mb4_unicode_ci',
    `is_active`   TINYINT UNSIGNED NOT NULL DEFAULT '1' COMMENT '계정 활성화 여부 (0: 비활성, 1: 정식, 2: 게스트)',
    `db_shard_id` TINYINT     NOT NULL DEFAULT '-1' COMMENT '유저별 DB Shard 번호',
    `insert_time` TIMESTAMP NOT NULL COMMENT '데이터 최초 생성일' DEFAULT CURRENT_TIMESTAMP,
    `update_time` TIMESTAMP NOT NULL COMMENT '데이터 갱신일' DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`user_id`) USING BTREE,
    INDEX `channel_uid_is_active` (`channel_uid`, `is_active`) USING BTREE
) COMMENT='계정 정보 테이블'
COLLATE='utf8mb4_unicode_ci'
ENGINE=InnoDB
AUTO_INCREMENT=1000000;



DROP TABLE IF EXISTS `TB_SERVER_MAINTENANCE`;
CREATE TABLE `TB_SERVER_MAINTENANCE`
(
    `idx`                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `server_type`         TINYINT UNSIGNED NOT NULL DEFAULT '1' COMMENT '점검 서버 종류 (1: 웹서버 만 사용함)',
    `start_time`          INT UNSIGNED NOT NULL COMMENT '점검 시작 시간',
    `end_time`            INT UNSIGNED NOT NULL COMMENT '점검 종료 시간',
    `msg`                 VARCHAR(128)  NOT NULL COMMENT '점검 메시지',
    `maintenance_version` INT UNSIGNED NOT NULL COMMENT '해당하는 점검 버전',
    `use_flag`            TINYINT UNSIGNED NOT NULL COMMENT '사용 여부',
    `last_modifier`       VARCHAR(50)  NOT NULL COMMENT '마지막 변경자' COLLATE 'utf8mb4_unicode_ci',
    `description`         VARCHAR(100) NOT NULL COMMENT '설명' COLLATE 'utf8mb4_unicode_ci',
    `insert_time`         TIMESTAMP NOT NULL COMMENT '데이터 최초 생성일' DEFAULT CURRENT_TIMESTAMP,
    `update_time`         TIMESTAMP NOT NULL COMMENT '데이터 갱신일' DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`idx`) USING BTREE,
    UNIQUE INDEX `maintenance_version` (`maintenance_version`) USING BTREE
) COMMENT='서버 점검 정보 테이블 (idx=1에 샘플 데이터 필수로 존재하고 그 이후로 데이터가 추가되어야 함)'
COLLATE='utf8mb4_unicode_ci'
ENGINE=InnoDB;




DROP TABLE IF EXISTS `TB_VERSION`;
CREATE TABLE `TB_VERSION`
(
    `idx`              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `client_version`   VARCHAR(10)  NOT NULL COLLATE 'utf8mb4_unicode_ci',
    `server_version`   VARCHAR(10)  NOT NULL COLLATE 'utf8mb4_unicode_ci',
    `app_id`           VARCHAR(100) NOT NULL COLLATE 'utf8mb4_unicode_ci',
    `is_active`        TINYINT UNSIGNED NOT NULL DEFAULT '1' COMMENT '버전 활성화 여부',
    `update_flag`      TINYINT UNSIGNED NOT NULL DEFAULT '0' COMMENT '해당 app_id 에 대한 업데이트 유도 여부',
    `inspection_flag`  TINYINT UNSIGNED NOT NULL DEFAULT '0' COMMENT '검수 활성화 여부 (0: 검수 완료, 1: 검수중)',
    `catalog_filename` VARCHAR(100) NOT NULL COMMENT '데이터 최근 갱신일' COLLATE 'utf8mb4_unicode_ci',
    `comment`          VARCHAR(248) NOT NULL COMMENT '코멘트 (내부 관리용)' COLLATE 'utf8mb4_unicode_ci',
    `insert_time`      TIMESTAMP NOT NULL COMMENT '데이터 최초 생성일' DEFAULT CURRENT_TIMESTAMP,
    `update_time`      TIMESTAMP NOT NULL COMMENT '데이터 갱신일' DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`idx`) USING BTREE,
    UNIQUE INDEX `app_id_client_version` (`app_id`, `client_version`) USING BTREE
) COMMENT='버전 정보 테이블'
COLLATE='utf8mb4_unicode_ci'
ENGINE=InnoDB;

insert into N_GAME.TB_VERSION (client_version, server_version, app_id, is_active, update_flag, inspection_flag, catalog_filename, comment)
values  ('2.01.01', '2.01.01.00', 'com.com2us.heatfuturenpb.android.google.jp.normal', 1, 0, 0, 'catalog_2.01.01.json', ''),
        ('2.01.01', '2.01.01.00', 'com.com2us.ent.futurenpb', 1, 0, 0, 'catalog_2.01.01.json', '');