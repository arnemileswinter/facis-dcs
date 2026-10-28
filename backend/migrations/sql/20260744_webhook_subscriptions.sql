-- Webhook subscriptions of the ORCE webhook platform (DCS-FR-TR-22,
-- DCS-FR-CSA-20). owner_did is the participant DID of the organization that
-- created the subscription; listing and deletion are scoped to it.
--
-- secret is the bearer value DCS sends to callback_url on each delivery, so it
-- must be recoverable. The API never returns it.
CREATE TABLE IF NOT EXISTS webhook_subscriptions
(
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    event        VARCHAR(255) NOT NULL,
    callback_url TEXT         NOT NULL,
    secret       TEXT         NOT NULL DEFAULT '',
    owner_did    VARCHAR(255) NOT NULL,
    created_by   VARCHAR(255) NOT NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_webhook_subscriptions_event ON webhook_subscriptions (event);
CREATE INDEX IF NOT EXISTS idx_webhook_subscriptions_owner_did ON webhook_subscriptions (owner_did);
