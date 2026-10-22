CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE CHECK (email = lower(email)),
  password_hash text NOT NULL,
  role text NOT NULL CHECK (role IN ('OPERATOR','SUPERVISOR')),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE products (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  sku text NOT NULL UNIQUE CHECK (btrim(sku) <> ''),
  name text NOT NULL CHECK (btrim(name) <> ''),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE receiving_documents (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  order_reference text NOT NULL UNIQUE CHECK (btrim(order_reference) <> ''),
  status text NOT NULL DEFAULT 'INCOMPLETE' CHECK (status IN ('INCOMPLETE','COMPLETED')),
  created_by uuid NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  CHECK ((status='COMPLETED') = (completed_at IS NOT NULL))
);

CREATE TABLE receiving_document_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  receiving_document_id uuid NOT NULL REFERENCES receiving_documents(id),
  product_id uuid NOT NULL REFERENCES products(id),
  expected_quantity bigint NOT NULL CHECK (expected_quantity > 0),
  UNIQUE(receiving_document_id, product_id),
  UNIQUE(id, receiving_document_id)
);

CREATE TABLE receiving_transactions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  receiving_document_id uuid NOT NULL REFERENCES receiving_documents(id),
  status text NOT NULL CHECK (status IN ('PENDING_APPROVAL','APPROVED','REJECTED_BY_SYSTEM')),
  review_status text CHECK (review_status IN ('UNRESOLVED','RESOLVED')),
  rejection_reason text,
  review_note text,
  created_by uuid NOT NULL REFERENCES users(id),
  approved_by uuid REFERENCES users(id),
  approved_at timestamptz,
  corrected_by uuid REFERENCES users(id),
  corrected_at timestamptz,
  reviewed_by uuid REFERENCES users(id),
  reviewed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(id, receiving_document_id),
  CHECK ((status='REJECTED_BY_SYSTEM' AND review_status IS NOT NULL AND rejection_reason IS NOT NULL)
    OR (status<>'REJECTED_BY_SYSTEM' AND review_status IS NULL AND rejection_reason IS NULL)),
  CHECK ((approved_by IS NULL)=(approved_at IS NULL)),
  CHECK ((corrected_by IS NULL)=(corrected_at IS NULL)),
  CHECK ((reviewed_by IS NULL)=(reviewed_at IS NULL)),
  CHECK ((status='APPROVED')=(approved_at IS NOT NULL)),
  CHECK ((review_status IS NOT DISTINCT FROM 'RESOLVED')=(reviewed_at IS NOT NULL)),
  CHECK (reviewed_at IS NULL OR (review_note IS NOT NULL AND btrim(review_note)<>''))
);
CREATE UNIQUE INDEX uq_active_trx_per_rcv ON receiving_transactions(receiving_document_id)
WHERE status='PENDING_APPROVAL' OR (status='REJECTED_BY_SYSTEM' AND review_status='UNRESOLVED');
CREATE INDEX trx_document_status ON receiving_transactions(receiving_document_id,status);

CREATE TABLE receiving_transaction_items (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  receiving_transaction_id uuid NOT NULL REFERENCES receiving_transactions(id),
  receiving_document_item_id uuid NOT NULL REFERENCES receiving_document_items(id),
  actual_quantity bigint NOT NULL CHECK (actual_quantity > 0),
  UNIQUE(receiving_transaction_id, receiving_document_item_id)
);

CREATE TABLE inventory_balances (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id uuid NOT NULL UNIQUE REFERENCES products(id),
  quantity bigint NOT NULL DEFAULT 0 CHECK (quantity>=0),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE inventory_transactions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id uuid NOT NULL REFERENCES products(id),
  receiving_transaction_item_id uuid NOT NULL UNIQUE REFERENCES receiving_transaction_items(id),
  quantity_delta bigint NOT NULL CHECK (quantity_delta>0),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX inventory_product_created ON inventory_transactions(product_id,created_at);

CREATE TABLE receiving_audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  receiving_document_id uuid NOT NULL REFERENCES receiving_documents(id),
  receiving_transaction_id uuid,
  actor_id uuid REFERENCES users(id),
  event_type text NOT NULL CHECK (event_type IN ('RECEIVING_CREATED','TRANSACTION_SUBMITTED','TRANSACTION_REJECTED',
    'TRANSACTION_CORRECTED','REVIEW_RESOLVED','TRANSACTION_APPROVED','RECEIVING_COMPLETED')),
  before_data jsonb,
  after_data jsonb,
  reason text,
  created_at timestamptz NOT NULL DEFAULT now(),
  FOREIGN KEY (receiving_transaction_id,receiving_document_id) REFERENCES receiving_transactions(id,receiving_document_id)
);
CREATE INDEX audit_document_created ON receiving_audit_logs(receiving_document_id,created_at);

CREATE TABLE notification_outbox (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  event_type text NOT NULL,
  aggregate_id uuid NOT NULL REFERENCES receiving_transactions(id),
  payload jsonb NOT NULL,
  published_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX outbox_unpublished_created ON notification_outbox(published_at,created_at);

CREATE TABLE notifications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  recipient_id uuid NOT NULL REFERENCES users(id),
  source_event_id uuid NOT NULL REFERENCES notification_outbox(id),
  receiving_transaction_id uuid REFERENCES receiving_transactions(id),
  event_type text NOT NULL,
  message text NOT NULL,
  read_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(source_event_id,recipient_id)
);
CREATE INDEX notification_recipient_created ON notifications(recipient_id,created_at);

DO $$
DECLARE relation text;
BEGIN
  FOREACH relation IN ARRAY ARRAY['users','products','receiving_documents','receiving_document_items',
    'receiving_transactions','receiving_transaction_items','inventory_balances','inventory_transactions',
    'receiving_audit_logs','notification_outbox','notifications'] LOOP
    EXECUTE format('ALTER TABLE %I ADD CONSTRAINT %I CHECK (substring(id::text,15,1)=''4'' AND substring(id::text,20,1) IN (''8'',''9'',''a'',''b''))', relation, relation || '_uuid_v4');
  END LOOP;
END;
$$;

CREATE FUNCTION forbid_history_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'history is append only' USING ERRCODE='23514';
END;
$$;
CREATE TRIGGER inventory_immutable BEFORE UPDATE OR DELETE ON inventory_transactions FOR EACH ROW EXECUTE FUNCTION forbid_history_change();
CREATE TRIGGER audit_immutable BEFORE UPDATE OR DELETE ON receiving_audit_logs FOR EACH ROW EXECUTE FUNCTION forbid_history_change();
CREATE TRIGGER expected_snapshot_immutable BEFORE UPDATE OR DELETE ON receiving_document_items FOR EACH ROW EXECUTE FUNCTION forbid_history_change();

CREATE FUNCTION guard_transaction() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP<>'INSERT' AND (OLD.status='APPROVED' OR (OLD.status='REJECTED_BY_SYSTEM' AND OLD.review_status='RESOLVED')) THEN
    RAISE EXCEPTION 'transaction is terminal' USING ERRCODE='23514';
  END IF;
  IF TG_OP='INSERT' AND EXISTS(SELECT 1 FROM receiving_documents WHERE id=NEW.receiving_document_id AND status='COMPLETED') THEN
    RAISE EXCEPTION 'receiving is completed' USING ERRCODE='23514';
  END IF;
  IF TG_OP='DELETE' THEN RETURN OLD; END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER transaction_guard BEFORE INSERT OR UPDATE OR DELETE ON receiving_transactions FOR EACH ROW EXECUTE FUNCTION guard_transaction();

CREATE FUNCTION guard_transaction_item() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE parent_id uuid; item_id uuid; parent_status text; review text; document_id uuid;
BEGIN
  IF TG_OP='UPDATE' AND EXISTS(SELECT 1 FROM receiving_transactions
    WHERE id=OLD.receiving_transaction_id AND (status='APPROVED' OR (status='REJECTED_BY_SYSTEM' AND review_status='RESOLVED'))) THEN
    RAISE EXCEPTION 'transaction items are immutable' USING ERRCODE='23514';
  END IF;
  IF TG_OP='DELETE' THEN parent_id:=OLD.receiving_transaction_id; ELSE parent_id:=NEW.receiving_transaction_id; END IF;
  SELECT status,review_status,receiving_document_id INTO parent_status,review,document_id FROM receiving_transactions WHERE id=parent_id;
  IF parent_status='APPROVED' OR (parent_status='REJECTED_BY_SYSTEM' AND review='RESOLVED') THEN
    RAISE EXCEPTION 'transaction items are immutable' USING ERRCODE='23514';
  END IF;
  IF TG_OP='DELETE' THEN RETURN OLD; END IF;
  IF NOT EXISTS(SELECT 1 FROM receiving_document_items WHERE id=NEW.receiving_document_item_id AND receiving_document_id=document_id) THEN
    RAISE EXCEPTION 'item belongs to a different receiving' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER transaction_item_guard BEFORE INSERT OR UPDATE OR DELETE ON receiving_transaction_items FOR EACH ROW EXECUTE FUNCTION guard_transaction_item();

CREATE FUNCTION guard_inventory_source() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NOT EXISTS(SELECT 1 FROM receiving_transaction_items ti
    JOIN receiving_transactions t ON t.id=ti.receiving_transaction_id
    JOIN receiving_document_items di ON di.id=ti.receiving_document_item_id
    WHERE ti.id=NEW.receiving_transaction_item_id AND t.status='APPROVED'
    AND di.product_id=NEW.product_id AND ti.actual_quantity=NEW.quantity_delta) THEN
    RAISE EXCEPTION 'inventory source must be approved' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END;
$$;
CREATE TRIGGER inventory_source_guard BEFORE INSERT ON inventory_transactions FOR EACH ROW EXECUTE FUNCTION guard_inventory_source();
