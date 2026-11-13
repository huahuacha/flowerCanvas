package app

import (
	"errors"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"

	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

func TestCompanyOnboardingIsolatedAndStartsAtZero(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{
		DriverName: "sqlite",
		DSN:        filepath.Join(t.TempDir(), "company.db"),
	}, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	if err := db.AutoMigrate(
		&model.User{},
		&model.AuthSession{},
		&model.Company{},
		&model.CompanyMember{},
		&model.AdminAuditEvent{},
		&model.CreditAccount{},
		&model.CreditLedgerEntry{},
		&model.ModelChannel{},
		&model.UserGroup{},
		&model.UserGroupChannel{},
		&model.UserGroupModelOverride{},
	); err != nil {
		t.Fatal(err)
	}
	s := &Service{repo: repository.New(db)}
	if err := s.EnsureDefaultUserGroup(); err != nil {
		t.Fatal(err)
	}
	platform := &model.User{
		ID: "platform", Username: "platform",
		Role: model.UserRoleAdmin, Status: model.UserStatusActive,
	}
	adminA := &model.User{
		ID: "admin-a", Username: "admin-a",
		Role: model.UserRoleUser, Status: model.UserStatusActive,
	}
	adminB := &model.User{
		ID: "admin-b", Username: "admin-b",
		Role: model.UserRoleUser, Status: model.UserStatusActive,
	}
	for _, user := range []*model.User{platform, adminA, adminB} {
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Create(&model.ModelChannel{
		ID:               "fixed",
		Scope:            model.ChannelScopeSystem,
		Enabled:          true,
		CredentialSource: model.ChannelCredentialSourceUser,
	}).Error; err != nil {
		t.Fatal(err)
	}
	defaultGroup := db.Model(&model.UserGroup{}).Where("code = ?", model.DefaultUserGroupCode)
	if err := defaultGroup.Update("all_channels", false).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateAdminCompany(platform, CreateCompanyRequest{Name: "No authorization", AdminUserID: adminA.ID}); err == nil {
		t.Fatal("company opened without the default group's fixed-channel grant")
	}
	if err := defaultGroup.Update("all_channels", true).Error; err != nil {
		t.Fatal(err)
	}
	for _, admin := range []*model.User{adminA, adminB} {
		if _, err := s.CreateAdminCompany(platform, CreateCompanyRequest{Name: admin.Username, AdminUserID: admin.ID}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CreateAdminCompany(platform, CreateCompanyRequest{Name: "Duplicate", AdminUserID: adminA.ID}); err == nil {
		t.Fatal("company binding accepted an existing member")
	}
	if _, err := s.CreateAdminCompany(platform, CreateCompanyRequest{Name: "Platform admin", AdminUserID: platform.ID}); err == nil {
		t.Fatal("company binding accepted a platform admin")
	}
	if companies, err := s.AdminCompanies(platform); err != nil || len(companies) != 2 {
		t.Fatalf("company list = %+v, err = %v", companies, err)
	}
	employee, err := s.CreateCompanyMember(adminA, CreateCompanyMemberRequest{
		Username: "employee-a", Email: "a.b+company@gmail.com", Password: "employee-password",
	})
	if err != nil {
		t.Fatal(err)
	}
	var account model.CreditAccount
	if err := db.First(&account, "user_id = ?", employee.UserID).Error; err != nil {
		t.Fatalf("company onboarding must create the account: %v", err)
	}
	if account.AvailableMicrocredits != 0 || account.ReservedMicrocredits != 0 {
		t.Fatalf("new employee balance = %+v", account)
	}
	user, err := s.repo.User(employee.UserID)
	if err != nil || user.Role != model.UserRoleUser || user.UserGroup != model.DefaultUserGroupCode || user.Status != model.UserStatusActive ||
		user.PasswordHash == "" || user.PasswordHash == "employee-password" {
		t.Fatalf("new employee account = %+v, err = %v", user, err)
	}
	if _, err := s.CreateCompanyMember(adminB, CreateCompanyMemberRequest{
		Username: "employee-b", Email: "ab@googlemail.com", Password: "employee-password",
	}); err == nil {
		t.Fatal("company onboarding accepted an existing canonical email")
	}
	if _, err := s.repo.UserByUsername("employee-b"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("rejected onboarding created an account: %v", err)
	}
	if _, err := s.CompanyMembers(user); err == nil {
		t.Fatal("employee accessed the company admin member list")
	}
	membersB, err := s.CompanyMembers(adminB)
	if err != nil || len(membersB.Members) != 1 || membersB.Members[0].UserID != adminB.ID {
		t.Fatalf("second company member list = %+v, err = %v", membersB, err)
	}
	if _, err := s.DisableCompanyMember(adminB, employee.UserID); err == nil {
		t.Fatal("company admin disabled another company's employee")
	}
	disabled, err := s.DisableCompanyMember(adminA, employee.UserID)
	if err != nil || disabled.Status != model.UserStatusDisabled {
		t.Fatalf("disable result = %+v, err = %v", disabled, err)
	}
	membersA, err := s.CompanyMembers(adminA)
	if err != nil || len(membersA.Members) != 2 || membersA.Members[1].Status != model.UserStatusDisabled {
		t.Fatalf("company list after disable = %+v, err = %v", membersA, err)
	}
}
