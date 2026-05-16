-- ScoreBot-Go 数据库初始化脚本

CREATE TABLE IF NOT EXISTS userdata (
    qqid VARCHAR(64) NOT NULL PRIMARY KEY,
    mode VARCHAR(16),
    zh VARCHAR(128),
    pw VARCHAR(128),
    id BIGINT,
    school VARCHAR(255),
    xuehao VARCHAR(64),
    name VARCHAR(64),
    grade VARCHAR(32),
    banji VARCHAR(64),
    exam VARCHAR(64),
    token VARCHAR(255)
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS teadata (
    school VARCHAR(255) NOT NULL PRIMARY KEY,
    account VARCHAR(128),
    password VARCHAR(128),
    cookie TEXT,
    cookie_fx VARCHAR(128),
    cookie_js VARCHAR(128),
    tofenxi VARCHAR(16),
    login_mode VARCHAR(16)
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS user_exam_context (
    qqid VARCHAR(64) NOT NULL PRIMARY KEY,
    exam VARCHAR(64) NOT NULL,
    subject_map LONGTEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS qt_student_exam_cache (
    qqid VARCHAR(64) NOT NULL PRIMARY KEY,
    payload LONGTEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS qt_teacher_rule_cache (
    school VARCHAR(255) NOT NULL,
    exam_guid VARCHAR(128) NOT NULL,
    rule_guid VARCHAR(128) NOT NULL,
    rule_type INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (school, exam_guid)
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS qt_teacher_overall_cache (
    school VARCHAR(255) NOT NULL,
    exam_ru_code VARCHAR(128) NOT NULL,
    exam_guid VARCHAR(128) NOT NULL,
    rule_guid VARCHAR(128) NOT NULL,
    payload LONGTEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (school, exam_ru_code, exam_guid, rule_guid)
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS qqbot_message_dedup (
    message_key VARCHAR(191) NOT NULL PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    seq VARCHAR(64) NOT NULL,
    message_id VARCHAR(128) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS advanced_search_lb_report_cache (
    qqid VARCHAR(64) NOT NULL,
    report_id BIGINT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (qqid, report_id)
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
