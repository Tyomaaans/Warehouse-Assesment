-- Delivery bookkeeping only: existing stable IDs, payloads and publication state are retained.
ALTER TABLE notification_outbox
  ADD COLUMN lease_token uuid,
  ADD COLUMN lease_until timestamptz,
  ADD COLUMN attempts bigint NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now(),
  ADD CONSTRAINT outbox_lease_pair CHECK ((lease_token IS NULL) = (lease_until IS NULL)),
  ADD CONSTRAINT outbox_lease_uuid_v4 CHECK (lease_token IS NULL OR
    (substring(lease_token::text,15,1)='4' AND substring(lease_token::text,20,1) IN ('8','9','a','b')));
CREATE INDEX outbox_delivery_pending ON notification_outbox(next_attempt_at,created_at,id)
  WHERE published_at IS NULL;
