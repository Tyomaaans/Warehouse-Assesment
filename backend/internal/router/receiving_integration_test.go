package router_test

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"warehouse/internal/inventory"
	"warehouse/internal/products"
	"warehouse/internal/receiving"
	"warehouse/pkg"
)

func newReceiving(t *testing.T, f *workflowFixture, quantities ...int64) receiving.ReceivingResponse {
	t.Helper()
	items := make([]receiving.ExpectedItemRequest, len(quantities))
	for i, quantity := range quantities {
		response := f.request("POST", "/api/v1/products", products.CreateProductRequest{SKU: "SKU-" + uuid.NewString(), Name: "Receiving test SKU"}, pkg.Supervisor)
		if response.Code != 201 {
			t.Fatalf("setup product status = %d", response.Code)
		}
		product := responseData[products.ProductResponse](t, response)
		items[i] = receiving.ExpectedItemRequest{ProductID: product.ID, ExpectedQuantity: quantity}
	}
	return createReceiving(t, f, items)
}

func createReceiving(t *testing.T, f *workflowFixture, items []receiving.ExpectedItemRequest) receiving.ReceivingResponse {
	t.Helper()
	response := f.request("POST", "/api/v1/receivings", receiving.CreateReceivingRequest{OrderReference: "ORD-" + uuid.NewString(), Items: items}, pkg.Operator)
	if response.Code != 201 {
		t.Fatalf("setup receiving status = %d", response.Code)
	}
	return responseData[receiving.ReceivingResponse](t, response)
}

func actualItems(doc receiving.ReceivingResponse, quantities ...int64) []receiving.ActualItemRequest {
	items := make([]receiving.ActualItemRequest, len(quantities))
	for i, quantity := range quantities {
		items[i] = receiving.ActualItemRequest{ReceivingDocumentItemID: doc.Items[i].ID, ActualQuantity: quantity}
	}
	return items
}

func submitTransaction(t *testing.T, f *workflowFixture, doc receiving.ReceivingResponse, quantities ...int64) receiving.TransactionResponse {
	t.Helper()
	response := f.request("POST", "/api/v1/receivings/"+doc.ID+"/transactions", receiving.CreateTransactionRequest{Items: actualItems(doc, quantities...)}, pkg.Operator)
	if response.Code != 201 {
		t.Fatalf("submission status = %d, want persisted 201", response.Code)
	}
	return responseData[receiving.TransactionResponse](t, response)
}

func approveTransaction(t *testing.T, f *workflowFixture, transaction receiving.TransactionResponse) receiving.TransactionResponse {
	t.Helper()
	response := f.request("POST", "/api/v1/transactions/"+transaction.ID+"/approve", nil, pkg.Supervisor)
	if response.Code != 200 {
		t.Fatalf("approval status = %d", response.Code)
	}
	return responseData[receiving.TransactionResponse](t, response)
}

func stockFor(t *testing.T, f *workflowFixture, item receiving.ReceivingItemResponse) int64 {
	t.Helper()
	response := f.request("GET", "/api/v1/inventory?search="+item.SKU, nil, pkg.Operator)
	if response.Code != 200 {
		t.Fatalf("inventory status = %d", response.Code)
	}
	page := responseData[inventory.PaginatedBalanceResponse](t, response)
	if len(page.Balances) == 0 {
		return 0
	}
	if len(page.Balances) != 1 || page.Balances[0].ProductID != item.ProductID {
		t.Fatal("inventory search returned wrong SKU")
	}
	return page.Balances[0].Quantity
}

func movementsFor(t *testing.T, f *workflowFixture, productID string) []inventory.MovementResponse {
	t.Helper()
	response := f.request("GET", "/api/v1/inventory/transactions?product_id="+productID, nil, pkg.Supervisor)
	if response.Code != 200 {
		t.Fatalf("movements status = %d", response.Code)
	}
	return responseData[inventory.PaginatedMovementResponse](t, response).Movements
}

func documentNow(t *testing.T, f *workflowFixture, id string) receiving.ReceivingResponse {
	t.Helper()
	return responseData[receiving.ReceivingResponse](t, f.request("GET", "/api/v1/receivings/"+id, nil, pkg.Operator))
}

func TestCoreReceivingIntegration(t *testing.T) {
	f := newWorkflowFixture(t)
	t.Run("T01_partial_then_full_sequential_receiving", func(t *testing.T) {
		doc := newReceiving(t, f, 130)
		first := submitTransaction(t, f, doc, 70)
		if first.Status != "PENDING_APPROVAL" || first.ApprovedAt != nil || stockFor(t, f, doc.Items[0]) != 0 {
			t.Fatal("pending receipt affected inventory")
		}
		first = approveTransaction(t, f, first)
		partial := documentNow(t, f, doc.ID)
		if stockFor(t, f, doc.Items[0]) != 70 || partial.Status != "INCOMPLETE" || partial.Items[0].ApprovedQuantity != 70 || partial.Items[0].RemainingQuantity != 60 || first.ApprovedBy == nil || *first.ApprovedBy != pkg.EncodeID(f.users[pkg.Supervisor].ID) {
			t.Fatal("incorrect partial approval state")
		}
		second := approveTransaction(t, f, submitTransaction(t, f, doc, 60))
		completed := documentNow(t, f, doc.ID)
		if stockFor(t, f, doc.Items[0]) != 130 || completed.Status != "COMPLETED" || completed.CompletedAt == nil || completed.Items[0].RemainingQuantity != 0 || first.ID == second.ID {
			t.Fatal("incorrect full receiving state")
		}
		movements := movementsFor(t, f, doc.Items[0].ProductID)
		if len(movements) != 2 || movements[0].QuantityDelta+movements[1].QuantityDelta != 130 {
			t.Fatal("partial/full ledger incorrect")
		}
		for _, row := range movements {
			if row.ReceivingDocumentID != doc.ID || row.ReceivingTransactionID == "" || row.ReceivingTransactionItemID == "" {
				t.Fatal("ledger source trace missing")
			}
		}
		history := responseData[receiving.PaginatedTransactionResponse](t, f.request("GET", "/api/v1/receivings/"+doc.ID+"/transactions", nil, pkg.Supervisor))
		if len(history.Transactions) != 2 || history.Meta.TotalData != 2 {
			t.Fatal("sequential transaction history missing")
		}
		workflowError(t, f.request("POST", "/api/v1/receivings/"+doc.ID+"/transactions", receiving.CreateTransactionRequest{Items: actualItems(doc, 1)}, pkg.Operator), 409, "conflict")
		workflowError(t, f.request("POST", "/api/v1/transactions/"+second.ID+"/approve", nil, pkg.Supervisor), 409, "conflict")
		if stockFor(t, f, doc.Items[0]) != 130 || len(movementsFor(t, f, doc.Items[0].ProductID)) != 2 {
			t.Fatal("repeat operation changed stock")
		}
	})
	t.Run("T02_T05_rejection_correction_and_immutable_audit", func(t *testing.T) {
		doc := newReceiving(t, f, 10)
		rejected := submitTransaction(t, f, doc, 60)
		if rejected.Status != "REJECTED_BY_SYSTEM" || rejected.ReviewStatus == nil || *rejected.ReviewStatus != "UNRESOLVED" || rejected.RejectionReason == nil || stockFor(t, f, doc.Items[0]) != 0 || len(movementsFor(t, f, doc.Items[0].ProductID)) != 0 {
			t.Fatal("over-limit attempt was not retained without inventory")
		}
		workflowError(t, f.request("POST", "/api/v1/transactions/"+rejected.ID+"/approve", nil, pkg.Supervisor), 409, "conflict")
		workflowError(t, f.request("POST", "/api/v1/receivings/"+doc.ID+"/transactions", receiving.CreateTransactionRequest{Items: actualItems(doc, 1)}, pkg.Operator), 409, "conflict")
		for _, quantity := range []int64{20, 10} {
			response := f.request("PATCH", "/api/v1/transactions/"+rejected.ID+"/correct", receiving.CorrectTransactionRequest{Reason: "verified input error", Items: actualItems(doc, quantity)}, pkg.Supervisor)
			if response.Code != 200 {
				t.Fatalf("correction status = %d", response.Code)
			}
			corrected := responseData[receiving.TransactionResponse](t, response)
			if corrected.ID != rejected.ID || corrected.ReceivingDocumentID != doc.ID || corrected.Items[0].ID != rejected.Items[0].ID || corrected.CorrectedAt == nil || corrected.CorrectedBy == nil || *corrected.CorrectedBy != pkg.EncodeID(f.users[pkg.Supervisor].ID) {
				t.Fatal("correction lost identity or actor/time")
			}
			if quantity == 20 && corrected.Status != "REJECTED_BY_SYSTEM" {
				t.Fatal("correction overrode expected cap")
			}
			if quantity == 10 && (corrected.Status != "PENDING_APPROVAL" || corrected.ReviewStatus != nil || corrected.RejectionReason != nil) {
				t.Fatal("valid correction did not become pending")
			}
		}
		if stockFor(t, f, doc.Items[0]) != 0 {
			t.Fatal("correction posted stock")
		}
		audits := responseData[receiving.PaginatedAuditResponse](t, f.request("GET", "/api/v1/receiving-audit-logs?receiving_id="+doc.ID, nil, pkg.Supervisor))
		corrections := 0
		for _, audit := range audits.AuditLogs {
			if audit.EventType != "TRANSACTION_CORRECTED" {
				continue
			}
			var before, after receiving.TransactionResponse
			if json.Unmarshal(audit.BeforeData, &before) != nil || json.Unmarshal(audit.AfterData, &after) != nil || before.ID != rejected.ID || after.ID != rejected.ID || before.Items[0].ActualQuantity <= after.Items[0].ActualQuantity || audit.Reason == nil || audit.ActorID == nil || audit.CreatedAt.IsZero() {
				t.Fatal("correction audit missing immutable before/after/reason/actor/time")
			}
			corrections++
		}
		if corrections != 2 {
			t.Fatalf("correction audit count = %d", corrections)
		}
		workflowError(t, f.request("PATCH", "/api/v1/transactions/"+rejected.ID+"/correct", receiving.CorrectTransactionRequest{Reason: "pending edit", Items: actualItems(doc, 1)}, pkg.Supervisor), 409, "conflict")
		approveTransaction(t, f, rejected)
		if stockFor(t, f, doc.Items[0]) != 10 {
			t.Fatal("corrected approval posted wrong quantity")
		}
	})
	t.Run("T08_resolve_real_excess_and_allow_next_receipt", func(t *testing.T) {
		doc := newReceiving(t, f, 10)
		e := submitTransaction(t, f, doc, 60)
		response := f.request("POST", "/api/v1/transactions/"+e.ID+"/review", receiving.ReviewTransactionRequest{Note: "confirmed physical excess; segregated for external handling"}, pkg.Supervisor)
		if response.Code != 200 {
			t.Fatalf("review status = %d", response.Code)
		}
		resolved := responseData[receiving.TransactionResponse](t, response)
		if resolved.Status != "REJECTED_BY_SYSTEM" || resolved.ReviewStatus == nil || *resolved.ReviewStatus != "RESOLVED" || resolved.ReviewNote == nil || resolved.ReviewedAt == nil || resolved.ReviewedBy == nil || stockFor(t, f, doc.Items[0]) != 0 {
			t.Fatal("resolution changed receiving semantics")
		}
		workflowError(t, f.request("PATCH", "/api/v1/transactions/"+e.ID+"/correct", receiving.CorrectTransactionRequest{Reason: "reopen", Items: actualItems(doc, 10)}, pkg.Supervisor), 409, "conflict")
		workflowError(t, f.request("POST", "/api/v1/transactions/"+e.ID+"/review", receiving.ReviewTransactionRequest{Note: "repeat"}, pkg.Supervisor), 409, "conflict")
		approveTransaction(t, f, submitTransaction(t, f, doc, 5))
		if stockFor(t, f, doc.Items[0]) != 5 || documentNow(t, f, doc.ID).Status != "INCOMPLETE" {
			t.Fatal("resolved excess affected stock or blocked next receipt")
		}
	})
	t.Run("T04_concurrent_pending_and_rejected_submission", func(t *testing.T) {
		for _, quantity := range []int64{5, 25} {
			doc := newReceiving(t, f, 20)
			start := make(chan struct{})
			responses := make(chan *httptest.ResponseRecorder, 8)
			var wg sync.WaitGroup
			for range 8 {
				wg.Go(func() {
					<-start
					responses <- f.request("POST", "/api/v1/receivings/"+doc.ID+"/transactions", receiving.CreateTransactionRequest{Items: actualItems(doc, quantity)}, pkg.Operator)
				})
			}
			close(start)
			wg.Wait()
			close(responses)
			successes := 0
			for response := range responses {
				if response.Code == 201 {
					successes++
				} else {
					workflowError(t, response, 409, "conflict")
				}
			}
			history := responseData[receiving.PaginatedTransactionResponse](t, f.request("GET", "/api/v1/receivings/"+doc.ID+"/transactions", nil, pkg.Operator))
			if successes != 1 || history.Meta.TotalData != 1 || stockFor(t, f, doc.Items[0]) != 0 {
				t.Fatal("multiple active transactions or premature inventory")
			}
		}
	})
	t.Run("T03_concurrent_and_repeated_approval_exactly_once", func(t *testing.T) {
		doc := newReceiving(t, f, 40)
		e := submitTransaction(t, f, doc, 7)
		start := make(chan struct{})
		responses := make(chan *httptest.ResponseRecorder, 8)
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				<-start
				responses <- f.request("POST", "/api/v1/transactions/"+e.ID+"/approve", nil, pkg.Supervisor)
			})
		}
		close(start)
		wg.Wait()
		close(responses)
		successes := 0
		for response := range responses {
			if response.Code == 200 {
				successes++
			} else {
				workflowError(t, response, 409, "conflict")
			}
		}
		workflowError(t, f.request("POST", "/api/v1/transactions/"+e.ID+"/approve", nil, pkg.Supervisor), 409, "conflict")
		if successes != 1 || stockFor(t, f, doc.Items[0]) != 7 || len(movementsFor(t, f, doc.Items[0].ProductID)) != 1 {
			t.Fatal("concurrent approval double-posted inventory")
		}
		id, _ := pkg.ParseID(e.ID)
		var count int64
		if err := f.db.Table("notification_outbox").Where("aggregate_id = ? AND event_type = ?", id, "TRANSACTION_APPROVED").Count(&count).Error; err != nil || count != 1 {
			t.Fatal("approval event was not committed exactly once")
		}
	})
	t.Run("completion_requires_all_expected_SKUs", func(t *testing.T) {
		doc := newReceiving(t, f, 10, 10)
		approveTransaction(t, f, submitTransaction(t, f, doc, 10))
		if documentNow(t, f, doc.ID).Status != "INCOMPLETE" {
			t.Fatal("completion ignored an omitted SKU")
		}
		response := f.request("POST", "/api/v1/receivings/"+doc.ID+"/transactions", receiving.CreateTransactionRequest{Items: []receiving.ActualItemRequest{{ReceivingDocumentItemID: doc.Items[1].ID, ActualQuantity: 10}}}, pkg.Operator)
		if response.Code != 201 {
			t.Fatalf("second SKU submission status = %d", response.Code)
		}
		approveTransaction(t, f, responseData[receiving.TransactionResponse](t, response))
		if documentNow(t, f, doc.ID).Status != "COMPLETED" {
			t.Fatal("all-SKU equality did not complete RCV")
		}
	})
	t.Run("correction_replaces_lines_and_rejects_entire_overlimit_attempt", func(t *testing.T) {
		doc := newReceiving(t, f, 10, 10, 10)
		e := submitTransaction(t, f, doc, 5, 11)
		for _, item := range doc.Items {
			if stockFor(t, f, item) != 0 {
				t.Fatal("rejected attempt partially posted inventory")
			}
		}
		response := f.request("PATCH", "/api/v1/transactions/"+e.ID+"/correct", receiving.CorrectTransactionRequest{Reason: "wrong SKU lines entered", Items: []receiving.ActualItemRequest{{ReceivingDocumentItemID: doc.Items[2].ID, ActualQuantity: 5}}}, pkg.Supervisor)
		if response.Code != 200 {
			t.Fatalf("replacement correction status = %d", response.Code)
		}
		corrected := responseData[receiving.TransactionResponse](t, response)
		read := responseData[receiving.TransactionResponse](t, f.request("GET", "/api/v1/transactions/"+e.ID, nil, pkg.Operator))
		if corrected.ID != e.ID || len(read.Items) != 1 || read.Items[0].ReceivingDocumentItemID != doc.Items[2].ID {
			t.Fatal("correction failed full item replacement")
		}
		approveTransaction(t, f, corrected)
		if stockFor(t, f, doc.Items[0]) != 0 || stockFor(t, f, doc.Items[1]) != 0 || stockFor(t, f, doc.Items[2]) != 5 {
			t.Fatal("removed SKU lines were posted")
		}
	})
	t.Run("cross_RCV_approvals_share_balance_without_lost_updates", func(t *testing.T) {
		first := newReceiving(t, f, 10, 10)
		items := []receiving.ExpectedItemRequest{{ProductID: first.Items[1].ProductID, ExpectedQuantity: 10}, {ProductID: first.Items[0].ProductID, ExpectedQuantity: 10}}
		second := createReceiving(t, f, items)
		a, b := submitTransaction(t, f, first, 3, 3), submitTransaction(t, f, second, 4, 4)
		responses := make(chan *httptest.ResponseRecorder, 2)
		var wg sync.WaitGroup
		for _, e := range []receiving.TransactionResponse{a, b} {
			wg.Go(func() { responses <- f.request("POST", "/api/v1/transactions/"+e.ID+"/approve", nil, pkg.Supervisor) })
		}
		wg.Wait()
		close(responses)
		for response := range responses {
			if response.Code != 200 {
				t.Fatalf("shared-stock approval status = %d", response.Code)
			}
		}
		for _, item := range first.Items {
			if stockFor(t, f, item) != 7 || len(movementsFor(t, f, item.ProductID)) != 2 {
				t.Fatal("shared balance lost an increment or ledger row")
			}
		}
	})
	t.Run("posting_and_late_outbox_failures_rollback_all_effects", func(t *testing.T) {
		for _, stage := range []string{"inventory", "outbox"} {
			doc := newReceiving(t, f, 10, 10)
			e := submitTransaction(t, f, doc, 10, 10)
			ids := make([]string, len(doc.Items))
			for i, item := range doc.Items {
				id, _ := pkg.ParseID(item.ProductID)
				ids[i] = id.String()
			}
			sort.Strings(ids)
			table, condition := "inventory_transactions", fmt.Sprintf("NEW.product_id = '%s'::uuid", ids[len(ids)-1])
			if stage == "outbox" {
				id, _ := pkg.ParseID(e.ID)
				table, condition = "notification_outbox", fmt.Sprintf("NEW.aggregate_id = '%s'::uuid AND NEW.event_type = 'TRANSACTION_APPROVED'", id.String())
			}
			if err := f.db.Exec(`CREATE FUNCTION task2d_fail_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected transaction failure' USING ERRCODE='P0001'; END; $$`).Error; err != nil {
				t.Fatal(err)
			}
			if err := f.db.Exec("CREATE TRIGGER task2d_fail_write BEFORE INSERT ON " + table + " FOR EACH ROW WHEN (" + condition + ") EXECUTE FUNCTION task2d_fail_write()").Error; err != nil {
				t.Fatal(err)
			}
			cleanup := func() {
				_ = f.db.Exec("DROP TRIGGER IF EXISTS task2d_fail_write ON " + table).Error
				_ = f.db.Exec("DROP FUNCTION IF EXISTS task2d_fail_write()").Error
			}
			workflowError(t, f.request("POST", "/api/v1/transactions/"+e.ID+"/approve", nil, pkg.Supervisor), 500, "internal")
			read := responseData[receiving.TransactionResponse](t, f.request("GET", "/api/v1/transactions/"+e.ID, nil, pkg.Supervisor))
			current := documentNow(t, f, doc.ID)
			if read.Status != "PENDING_APPROVAL" || read.ApprovedAt != nil || current.Status != "INCOMPLETE" || current.CompletedAt != nil {
				cleanup()
				t.Fatal("failed approval retained status/completion")
			}
			for _, item := range doc.Items {
				if stockFor(t, f, item) != 0 || len(movementsFor(t, f, item.ProductID)) != 0 {
					cleanup()
					t.Fatal("failed posting retained partial inventory")
				}
			}
			id, _ := pkg.ParseID(e.ID)
			var auditCount, outboxCount int64
			if err := f.db.Table("receiving_audit_logs").Where("receiving_transaction_id = ?", id).Count(&auditCount).Error; err != nil {
				cleanup()
				t.Fatal(err)
			}
			if err := f.db.Table("notification_outbox").Where("aggregate_id = ?", id).Count(&outboxCount).Error; err != nil {
				cleanup()
				t.Fatal(err)
			}
			if auditCount != 1 || outboxCount != 1 {
				cleanup()
				t.Fatal("failed approval retained audit/outbox effects")
			}
			cleanup()
			approveTransaction(t, f, e)
			for _, item := range doc.Items {
				if stockFor(t, f, item) != 10 || len(movementsFor(t, f, item.ProductID)) != 1 {
					t.Fatal("retry after rollback did not post once")
				}
			}
		}
	})
	t.Run("approved_transactions_ledger_and_audit_are_immutable", func(t *testing.T) {
		doc := newReceiving(t, f, 10)
		e := approveTransaction(t, f, submitTransaction(t, f, doc, 5))
		workflowError(t, f.request("PATCH", "/api/v1/transactions/"+e.ID+"/correct", receiving.CorrectTransactionRequest{Reason: "attempt approved edit", Items: actualItems(doc, 1)}, pkg.Supervisor), 409, "conflict")
		workflowError(t, f.request("POST", "/api/v1/transactions/"+e.ID+"/review", receiving.ReviewTransactionRequest{Note: "attempt review"}, pkg.Supervisor), 409, "conflict")
		id, _ := pkg.ParseID(e.ID)
		for _, statement := range []string{
			"UPDATE receiving_transactions SET updated_at=now() WHERE id=?",
			"DELETE FROM receiving_transactions WHERE id=?",
			"UPDATE receiving_transaction_items SET actual_quantity=1 WHERE receiving_transaction_id=?",
			"DELETE FROM receiving_transaction_items WHERE receiving_transaction_id=?",
			"UPDATE inventory_transactions SET quantity_delta=1 WHERE receiving_transaction_item_id IN (SELECT id FROM receiving_transaction_items WHERE receiving_transaction_id=?)",
			"DELETE FROM receiving_audit_logs WHERE receiving_transaction_id=?",
		} {
			if err := f.db.Exec(statement, id).Error; err == nil {
				t.Fatal("database permitted immutable history mutation")
			}
		}
		if stockFor(t, f, doc.Items[0]) != 5 || len(movementsFor(t, f, doc.Items[0].ProductID)) != 1 {
			t.Fatal("immutable mutation changed stock")
		}
	})
	t.Run("T06_T07_permissions_validation_and_read_contracts", func(t *testing.T) {
		doc, foreign := newReceiving(t, f, 10), newReceiving(t, f, 10)
		path := "/api/v1/receivings/" + doc.ID + "/transactions"
		payload := receiving.CreateTransactionRequest{Items: actualItems(doc, 1)}
		workflowError(t, f.request("POST", path, payload, pkg.Supervisor), 403, "forbidden")
		for _, body := range []string{`null`, `{"items":[]}`, `{"items":[null]}`, `{"items":[{"actual_quantity":1}]}`,
			fmt.Sprintf(`{"items":[{"receiving_document_item_id":%q,"actual_quantity":1}]}`, uuid.NewString()),
			fmt.Sprintf(`{"items":[{"receiving_document_item_id":%q,"actual_quantity":1}]}`, foreign.Items[0].ID),
			fmt.Sprintf(`{"items":[{"receiving_document_item_id":%q,"actual_quantity":1},{"receiving_document_item_id":%q,"actual_quantity":1}]}`, doc.Items[0].ID, doc.Items[0].ID),
			fmt.Sprintf(`{"items":[{"receiving_document_item_id":%q,"actual_quantity":1}],"created_by":%q}`, doc.Items[0].ID, pkg.EncodeID(f.users[pkg.Supervisor].ID)),
		} {
			workflowError(t, f.request("POST", path, body, pkg.Operator), 400, "invalid_input")
		}
		for _, quantity := range []string{"0", "-1", "1.5", "9223372036854775808"} {
			workflowError(t, f.request("POST", path, fmt.Sprintf(`{"items":[{"receiving_document_item_id":%q,"actual_quantity":%s}]}`, doc.Items[0].ID, quantity), pkg.Operator), 400, "invalid_input")
		}
		e := submitTransaction(t, f, doc, 11)
		for _, operation := range []struct {
			method, path string
			body         any
		}{
			{"POST", "/api/v1/transactions/" + e.ID + "/approve", nil},
			{"PATCH", "/api/v1/transactions/" + e.ID + "/correct", receiving.CorrectTransactionRequest{Reason: "fix", Items: actualItems(doc, 10)}},
			{"POST", "/api/v1/transactions/" + e.ID + "/review", receiving.ReviewTransactionRequest{Note: "verify"}},
			{"GET", "/api/v1/receiving-audit-logs", nil},
		} {
			workflowError(t, f.request(operation.method, operation.path, operation.body, pkg.Operator), 403, "forbidden")
		}
		workflowError(t, f.request("PATCH", "/api/v1/transactions/"+e.ID+"/correct", receiving.CorrectTransactionRequest{Reason: " ", Items: actualItems(doc, 10)}, pkg.Supervisor), 400, "invalid_input")
		workflowError(t, f.request("POST", "/api/v1/transactions/"+e.ID+"/review", receiving.ReviewTransactionRequest{Note: " "}, pkg.Supervisor), 400, "invalid_input")
		for _, role := range []string{pkg.Operator, pkg.Supervisor} {
			for _, target := range []string{path, "/api/v1/transactions/" + e.ID, "/api/v1/inventory", "/api/v1/inventory/transactions"} {
				if response := f.request("GET", target, nil, role); response.Code != 200 {
					t.Fatalf("allowed read status = %d", response.Code)
				}
				workflowError(t, f.request("GET", target, nil, ""), 401, "unauthorized")
			}
		}
		for _, target := range []string{"/api/v1/transactions/" + uuid.NewString(), "/api/v1/inventory/transactions?product_id=", "/api/v1/inventory/transactions?product_id=" + uuid.NewString(), "/api/v1/receiving-audit-logs?receiving_id=", "/api/v1/receiving-audit-logs?receiving_id=" + uuid.NewString(), path + "?limit=0", "/api/v1/inventory?limit=101", "/api/v1/inventory?search=" + strings.Repeat("x", 101)} {
			workflowError(t, f.request("GET", target, nil, pkg.Supervisor), 400, "invalid_input")
		}
		for _, target := range []string{"/api/v1/transactions/" + pkg.EncodeID(uuid.New()), "/api/v1/receivings/" + pkg.EncodeID(uuid.New()) + "/transactions"} {
			workflowError(t, f.request("GET", target, nil, pkg.Supervisor), 404, "not_found")
		}
		for _, target := range []string{"/api/v1/inventory/transactions?product_id=" + pkg.EncodeID(uuid.New()), "/api/v1/receiving-audit-logs?receiving_id=" + pkg.EncodeID(uuid.New()), "/api/v1/inventory?page=10000", path + "?page=10000"} {
			if response := f.request("GET", target, nil, pkg.Supervisor); response.Code != 200 {
				t.Fatalf("empty page/filter status = %d", response.Code)
			}
		}
		var published int64
		if err := f.db.Table("notification_outbox").Where("published_at IS NOT NULL").Count(&published).Error; err != nil || published != 0 || tableCount(t, f.db, "notifications") != 0 {
			t.Fatal("Task 2D activated notification delivery")
		}
	})
}
