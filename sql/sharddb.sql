USE N_SHARD_10;

DROP TABLE IF EXISTS `TB_ASSET`;
CREATE TABLE `TB_ASSET`
(
    `idx`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`     BIGINT UNSIGNED NOT NULL COMMENT '유저 아이디',
    `asset_id`    INT UNSIGNED NOT NULL COMMENT '무료/유료는 id로 나뉨',
    `quantity`    BIGINT NOT NULL DEFAULT '0' COMMENT '수량 (마이너스 통장 가능)',
    `insert_time` TIMESTAMP NOT NULL COMMENT '데이터 최초 생성일' DEFAULT CURRENT_TIMESTAMP,
    `update_time` TIMESTAMP NOT NULL COMMENT '데이터 갱신일' DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`idx`) USING BTREE,
    UNIQUE INDEX `user_id_asset_id` (`user_id`, `asset_id`) USING BTREE
) COMMENT='단일 재화 정보 테이블 (무료는 필수 존재, 유료는 optional)'
COLLATE='utf8mb4_unicode_ci'
ENGINE=InnoDB;


DROP TABLE IF EXISTS `TB_CARD`;
CREATE TABLE `TB_CARD`
(
    `idx`                      BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`                  BIGINT UNSIGNED NOT NULL COMMENT '유저 아이디',
    `card_id`                  INT UNSIGNED NOT NULL COMMENT '카드 아이디',
    `two_way_idx`              BIGINT UNSIGNED NOT NULL COMMENT '투타겸업 카드쌍 아이디',
    `level_break_step`         SMALLINT UNSIGNED NOT NULL COMMENT '훈련 돌파 횟수',
    `level_break_cur_material` SMALLINT UNSIGNED NOT NULL COMMENT 'level_break_step 에 재료로 들어간 카드 갯수',
    `limit_break_level`        SMALLINT UNSIGNED NOT NULL COMMENT 'limit_break_exp 에 따른 레벨 (= 한계 돌파)',
    `limit_break_exp`          INT UNSIGNED NOT NULL COMMENT '카드로 쌓은 한계 돌파 경험치 (maximumExp = 기획 데이터 정의)',
    `level`                    SMALLINT UNSIGNED NOT NULL COMMENT '개인훈련 레벨',
    `exp`                      INT UNSIGNED NOT NULL COMMENT '개인훈련 습득 경험치',
    `exp_exceed`               INT UNSIGNED NOT NULL COMMENT '개인훈련 초과 경험치 (exp 초과 시 누적)',
    `theme_id`                 SMALLINT UNSIGNED NOT NULL COMMENT '카드의 테마 정보',
    `extra_theme_id`           SMALLINT UNSIGNED NOT NULL DEFAULT '0' COMMENT '카드의 2번째 테마 정보 (0: 없음)',
    `is_lock`                  TINYINT UNSIGNED NOT NULL DEFAULT '0' COMMENT '카드 잠금 여부',
    `skill`                    JSON NOT NULL COMMENT '스킬 목록',
    `potential_list`           JSON NOT NULL COMMENT '잠재력 리스트',
    `enhance_card_idx`         BIGINT UNSIGNED NOT NULL COMMENT '인핸스 카드 인덱스 (= TB_ENHANCE_CARD.idx)',
    `insert_time`              TIMESTAMP NOT NULL COMMENT '데이터 최초 생성일' DEFAULT CURRENT_TIMESTAMP,
    `update_time`              TIMESTAMP NOT NULL COMMENT '데이터 갱신일' DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`idx`) USING BTREE,
    INDEX                      `user_id` (`user_id`) USING BTREE
) COMMENT='카드 정보'
COLLATE='utf8mb4_unicode_ci'
ENGINE=InnoDB;


DROP TABLE IF EXISTS `TB_ITEM`;
CREATE TABLE `TB_ITEM`
(
    `idx`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`       BIGINT UNSIGNED NOT NULL,
    `item_type`     TINYINT UNSIGNED NOT NULL COMMENT '아이템 타입',
    `item_id`       INT UNSIGNED NOT NULL,
    `amount`        BIGINT NOT NULL DEFAULT '0' COMMENT '마이너트 통장 가능',
    `insert_time`   TIMESTAMP NOT NULL COMMENT '데이터 최초 생성일' DEFAULT CURRENT_TIMESTAMP,
    `update_time`   TIMESTAMP NOT NULL COMMENT '데이터 갱신일' DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`idx`) USING BTREE,
    UNIQUE INDEX `user_id` (`user_id`, `item_id`) USING BTREE
) COMMENT='아이템 정보 테이블'
COLLATE='utf8mb4_unicode_ci'
ENGINE=InnoDB;
