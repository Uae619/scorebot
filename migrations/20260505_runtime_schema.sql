-- Apply this migration before deploying the FC build that removes runtime DDL.
-- It moves the schema previously checked from database.go init/runtime paths into
-- an explicit deployment step.

CREATE TABLE IF NOT EXISTS user_exam_context (
	qqid VARCHAR(64) NOT NULL PRIMARY KEY,
	exam VARCHAR(64) NOT NULL,
	subject_map LONGTEXT NOT NULL,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

ALTER TABLE user_exam_context
	MODIFY COLUMN exam VARCHAR(64) NOT NULL;

CREATE TABLE IF NOT EXISTS qt_student_exam_cache (
	qqid VARCHAR(64) NOT NULL PRIMARY KEY,
	payload LONGTEXT NOT NULL,
	expires_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS advanced_search_lb_report_cache (
	qqid VARCHAR(64) NOT NULL,
	report_id BIGINT NOT NULL,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
	PRIMARY KEY (qqid, report_id)
) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS qqbot_message_dedup (
	message_key VARCHAR(191) NOT NULL PRIMARY KEY,
	user_id VARCHAR(64) NOT NULL,
	seq VARCHAR(64) NOT NULL,
	message_id VARCHAR(128) NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
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

DROP PROCEDURE IF EXISTS add_column_if_missing;

DELIMITER //
CREATE PROCEDURE add_column_if_missing(
	IN table_name_value VARCHAR(64),
	IN column_name_value VARCHAR(64),
	IN column_definition_value TEXT
)
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
			AND TABLE_NAME = table_name_value
			AND COLUMN_NAME = column_name_value
	) THEN
		SET @ddl = CONCAT(
			'ALTER TABLE `',
			REPLACE(table_name_value, '`', '``'),
			'` ADD COLUMN `',
			REPLACE(column_name_value, '`', '``'),
			'` ',
			column_definition_value
		);
		PREPARE stmt FROM @ddl;
		EXECUTE stmt;
		DEALLOCATE PREPARE stmt;
	END IF;
END//
DELIMITER ;

CALL add_column_if_missing('teadata', 'login_mode', 'VARCHAR(16) NULL DEFAULT NULL AFTER `tofenxi`');
CALL add_column_if_missing('qt_teacher_rule_cache', 'rule_type', 'INT NOT NULL DEFAULT 0 AFTER `rule_guid`');

DROP PROCEDURE IF EXISTS add_column_if_missing;
