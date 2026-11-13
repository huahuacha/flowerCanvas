package repository

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
	"yingce/backend/internal/model"
)

func TestCompanyTransferConcurrentReplayAndDisable(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: filepath.Join(t.TempDir(), "company.db") + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"}, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	if err := db.AutoMigrate(&model.User{}, &model.AuthSession{}, &model.Company{}, &model.CompanyMember{}, &model.CreditAccount{}, &model.CreditLedgerEntry{}, &model.BillingOrder{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	users := []model.User{
		{ID: "admin", Username: "admin", Role: model.UserRoleUser, Status: model.UserStatusActive},
		{ID: "employee", Username: "employee", Role: model.UserRoleUser, Status: model.UserStatusActive},
		{ID: "outsider", Username: "outsider", Role: model.UserRoleUser, Status: model.UserStatusActive},
	}
	members := []model.CompanyMember{
		{ID: "admin-member", UserID: "admin", CompanyID: "company", Role: model.CompanyMemberRoleAdmin},
		{ID: "employee-member", UserID: "employee", CompanyID: "company", Role: model.CompanyMemberRoleEmployee},
		{ID: "outsider-member", UserID: "outsider", CompanyID: "other-company", Role: model.CompanyMemberRoleEmployee},
	}
	accounts := []model.CreditAccount{
		{UserID: "admin", AvailableMicrocredits: 10, ReservedMicrocredits: 3},
		{UserID: "employee", AvailableMicrocredits: 1, ReservedMicrocredits: 2},
	}
	for _, rows := range []any{&users, &members, &accounts} {
		if err := db.Create(rows).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.Company{ID: "company", Name: "Company"}).Error; err != nil {
		t.Fatal(err)
	}
	session := &model.AuthSession{ID: "employee-session", UserID: "employee", TokenHash: "session-token"}
	if err := db.Create(session).Error; err != nil {
		t.Fatal(err)
	}
	r := New(db)
	allocate := func(target string, amount int64, requestID, eventID string) (*CompanyTransferResult, error) {
		return r.AllocateCompanyCredits("company", "admin", target, amount, requestID, &model.AdminAuditEvent{ID: eventID})
	}
	if _, err := allocate("outsider", 7, "cross-company", "cross-audit"); !errors.Is(err, ErrCompanyEmployeeRequired) {
		t.Fatalf("cross-company transfer = %v", err)
	}
	var group sync.WaitGroup
	results := make(chan *CompanyTransferResult, 2)
	for _, eventID := range []string{"transfer-audit-1", "transfer-audit-2"} {
		group.Go(func() {
			result, err := allocate("employee", 7, "shared-request", eventID)
			if err != nil {
				t.Errorf("transfer failed: %v", err)
			}
			results <- result
		})
	}
	group.Wait()
	close(results)
	replays := 0
	for result := range results {
		if result == nil {
			continue
		}
		if result.Replayed {
			replays++
		}
		if result.ActorAccount.AvailableMicrocredits != 3 || result.TargetAccount.AvailableMicrocredits != 8 ||
			result.ActorAccount.ReservedMicrocredits != 3 || result.TargetAccount.ReservedMicrocredits != 2 {
			t.Fatalf("transfer balances = %+v", result)
		}
	}
	if replays != 1 {
		t.Fatalf("replays = %d, want 1", replays)
	}
	if _, err := allocate("employee", 6, "shared-request", "conflict-audit"); !errors.Is(err, ErrCompanyTransferConflict) {
		t.Fatalf("conflicting retry = %v", err)
	}
	if _, err := allocate("employee", 4, "overdraft-request", "overdraft-audit"); !errors.Is(err, ErrInsufficientCredits) {
		t.Fatalf("overdraft = %v", err)
	}
	var audit model.AdminAuditEvent
	if err := db.First(&audit).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := allocate("employee", 1, "rollback-request", audit.ID); err == nil {
		t.Fatal("expected duplicate audit ID to roll back allocation")
	}
	var entries []model.CreditLedgerEntry
	if err := db.Order("available_delta_microcredits").Find(&entries).Error; err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].UserID != "admin" || entries[0].AvailableDeltaMicrocredits != -7 ||
		entries[1].UserID != "employee" || entries[1].AvailableDeltaMicrocredits != 7 {
		t.Fatalf("expected balanced ledger entries, got %+v", entries)
	}
	var persisted []model.CreditAccount
	if err := db.Order("user_id").Find(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 2 || persisted[0].AvailableMicrocredits != 3 || persisted[1].AvailableMicrocredits != 8 {
		t.Fatalf("rollback changed balances: %+v", persisted)
	}
	reservations := make(chan error, 2)
	group.Go(func() {
		reservations <- r.ReserveBillingOrder(&model.BillingOrder{ID: "generation", UserID: "admin", IdempotencyKey: "generation", AmountMicrocredits: 2})
	})
	group.Go(func() {
		_, err := allocate("employee", 2, "generation-race", "generation-audit")
		reservations <- err
	})
	group.Wait()
	close(reservations)
	successes, overdrafts := 0, 0
	for err := range reservations {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrInsufficientCredits):
			overdrafts++
		default:
			t.Fatalf("concurrent reserve/allocation: %v", err)
		}
	}
	if successes != 1 || overdrafts != 1 {
		t.Fatalf("concurrent reserve/allocation produced %d successes and %d overdrafts", successes, overdrafts)
	}
	if _, _, err := r.DisableCompanyEmployee("company", "admin", "employee", &model.AdminAuditEvent{ID: "disable-audit"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.AuthSession(session.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("disabled employee session lookup = %v", err)
	}
	if _, err := allocate("employee", 1, "after-disable", "disabled-audit"); !errors.Is(err, ErrCompanyEmployeeRequired) {
		t.Fatalf("new transfer after disable = %v", err)
	}
}
