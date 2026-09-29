-- GM 管理台邮件系统：邮件存储表（阶段 1：管理台侧邮件；游戏内收信待服务端协议逆向，阶段 2）
CREATE TABLE IF NOT EXISTS gm_mail (
  id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  to_account_id  bigint       NOT NULL,
  to_character_id bigint      NOT NULL DEFAULT 0,
  template       bigint       NOT NULL CHECK (template > 0),
  amount         bigint       NOT NULL CHECK (amount > 0 AND amount <= 4294967295),
  title          text         NOT NULL DEFAULT '',
  body           text         NOT NULL DEFAULT '',
  status         text         NOT NULL DEFAULT 'unread',
  claimed_at     timestamptz,
  expires_at     timestamptz,
  created_at     timestamptz  NOT NULL DEFAULT now(),
  CONSTRAINT gm_mail_status_check CHECK (status IN ('unread','read','claimed','expired','revoked'))
);
CREATE INDEX IF NOT EXISTS gm_mail_to_account_idx ON gm_mail(to_account_id, status);
CREATE INDEX IF NOT EXISTS gm_mail_to_char_idx ON gm_mail(to_character_id, status);
