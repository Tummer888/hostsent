-- 004_security_schema.sql
-- 安全与风控一期落库表结构

CREATE TABLE IF NOT EXISTS login_logs (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT      NOT NULL,
    username          VARCHAR(64) NOT NULL,
    login_type        VARCHAR(32) NOT NULL,
    result            VARCHAR(32) NOT NULL,
    failure_reason    VARCHAR(255),
    ip                VARCHAR(64) NOT NULL,
    ip_region         VARCHAR(128),
    user_agent        VARCHAR(255),
    device_fingerprint VARCHAR(255),
    platform          VARCHAR(32) NOT NULL,
    risk_flag         VARCHAR(32),
    created_at        TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_login_logs_user_id ON login_logs (user_id);
CREATE INDEX IF NOT EXISTS idx_login_logs_username ON login_logs (username);
CREATE INDEX IF NOT EXISTS idx_login_logs_login_type ON login_logs (login_type);
CREATE INDEX IF NOT EXISTS idx_login_logs_result ON login_logs (result);
CREATE INDEX IF NOT EXISTS idx_login_logs_ip ON login_logs (ip);
CREATE INDEX IF NOT EXISTS idx_login_logs_platform ON login_logs (platform);
CREATE INDEX IF NOT EXISTS idx_login_logs_risk_flag ON login_logs (risk_flag);

CREATE TABLE IF NOT EXISTS audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    operator_id     BIGINT      NOT NULL,
    operator_name   VARCHAR(64) NOT NULL,
    module          VARCHAR(64) NOT NULL,
    resource_type   VARCHAR(64) NOT NULL,
    resource_id     VARCHAR(64),
    action          VARCHAR(64) NOT NULL,
    request_method  VARCHAR(16) NOT NULL,
    request_path    VARCHAR(255) NOT NULL,
    request_payload TEXT,
    response_code   INT         NOT NULL,
    response_message VARCHAR(255),
    ip              VARCHAR(64),
    user_agent      VARCHAR(255),
    trace_id        VARCHAR(128),
    created_at      TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_operator_id ON audit_logs (operator_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_module ON audit_logs (module);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_type ON audit_logs (resource_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs (action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_trace_id ON audit_logs (trace_id);

CREATE TABLE IF NOT EXISTS risk_events (
    id               BIGSERIAL PRIMARY KEY,
    risk_type        VARCHAR(64)  NOT NULL,
    risk_level       VARCHAR(32)  NOT NULL,
    user_id          BIGINT       NOT NULL,
    username         VARCHAR(64)  NOT NULL,
    ip               VARCHAR(64),
    device_fingerprint VARCHAR(255),
    rule_code        VARCHAR(128) NOT NULL,
    summary          VARCHAR(255) NOT NULL,
    detail_payload   TEXT,
    occur_count      INT          NOT NULL DEFAULT 1,
    first_occurred_at TIMESTAMP    NOT NULL,
    last_occurred_at TIMESTAMP     NOT NULL,
    status           VARCHAR(32)  NOT NULL,
    handled_by       BIGINT,
    handled_at       TIMESTAMP,
    handle_note      VARCHAR(255),
    created_at       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_risk_events_risk_type ON risk_events (risk_type);
CREATE INDEX IF NOT EXISTS idx_risk_events_risk_level ON risk_events (risk_level);
CREATE INDEX IF NOT EXISTS idx_risk_events_status ON risk_events (status);
CREATE INDEX IF NOT EXISTS idx_risk_events_user_id ON risk_events (user_id);
CREATE INDEX IF NOT EXISTS idx_risk_events_rule_code ON risk_events (rule_code);

CREATE TABLE IF NOT EXISTS blacklists (
    id            BIGSERIAL PRIMARY KEY,
    type          VARCHAR(32)  NOT NULL,
    target_value  VARCHAR(255) NOT NULL,
    status        VARCHAR(32)  NOT NULL,
    source        VARCHAR(32)  NOT NULL,
    reason        VARCHAR(255),
    effective_at  TIMESTAMP    NOT NULL,
    expired_at    TIMESTAMP,
    hit_count     INT          NOT NULL DEFAULT 0,
    created_by    BIGINT       NOT NULL,
    updated_by    BIGINT       NOT NULL,
    created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT idx_blacklists_type_target UNIQUE (type, target_value)
);
CREATE INDEX IF NOT EXISTS idx_blacklists_status ON blacklists (status);
CREATE INDEX IF NOT EXISTS idx_blacklists_source ON blacklists (source);
CREATE INDEX IF NOT EXISTS idx_blacklists_created_by ON blacklists (created_by);

CREATE TABLE IF NOT EXISTS user_sessions (
    id               BIGSERIAL PRIMARY KEY,
    session_id       VARCHAR(128) NOT NULL UNIQUE,
    user_id          BIGINT       NOT NULL,
    username         VARCHAR(64)  NOT NULL,
    platform         VARCHAR(32)  NOT NULL,
    ip               VARCHAR(64),
    ip_region        VARCHAR(128),
    user_agent       VARCHAR(255),
    device_fingerprint VARCHAR(255),
    login_at         TIMESTAMP    NOT NULL,
    last_active_at   TIMESTAMP    NOT NULL,
    expired_at       TIMESTAMP,
    status           VARCHAR(32)  NOT NULL,
    risk_flag        VARCHAR(32),
    revoked_reason   VARCHAR(255),
    revoked_by       BIGINT,
    revoked_at       TIMESTAMP,
    created_at       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_status ON user_sessions (status);
CREATE INDEX IF NOT EXISTS idx_user_sessions_platform ON user_sessions (platform);
CREATE INDEX IF NOT EXISTS idx_user_sessions_risk_flag ON user_sessions (risk_flag);
CREATE INDEX IF NOT EXISTS idx_user_sessions_login_at ON user_sessions (login_at);
