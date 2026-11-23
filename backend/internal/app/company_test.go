package app

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

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
		&model.TaskTextDelta{},
		&model.Company{},
		&model.CompanyMember{},
		&model.AdminAuditEvent{},
		&model.CreditAccount{},
		&model.CreditLedgerEntry{},
		&model.PaymentOrder{},
		&model.BillingOrder{},
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
	for _, user := range []*model.User{platform} {
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
	request := func(name, username string) CreateCompanyRequest {
		return CreateCompanyRequest{Name: name, Admin: CreateCompanyAdminRequest{CreateCompanyMemberRequest: CreateCompanyMemberRequest{Username: username, Password: "admin-password"}}}
	}
	if _, err := s.CreateAdminCompany(platform, request("No authorization", adminA.Username)); err == nil {
		t.Fatal("company opened without the default group's fixed-channel grant")
	}
	if _, err := s.repo.UserByUsername(adminA.Username); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("failed company opening created an account: %v", err)
	}
	if err := defaultGroup.Update("all_channels", true).Error; err != nil {
		t.Fatal(err)
	}
	for _, admin := range []*model.User{adminA, adminB} {
		company, err := s.CreateAdminCompany(platform, request(admin.Username, admin.Username))
		if err != nil {
			t.Fatal(err)
		}
		created, err := s.repo.User(company.Admins[0].UserID)
		if err != nil {
			t.Fatal(err)
		}
		*admin = *created
	}
	if _, err := s.CreateAdminCompany(platform, request("Duplicate", adminA.Username)); err == nil {
		t.Fatal("company opening accepted a duplicate username")
	}
	if _, err := s.CreateAdminCompany(platform, request("Platform admin", platform.Username)); err == nil {
		t.Fatal("company opening accepted a duplicate platform admin username")
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
	adminUsers, err := s.AdminUsers(platform, AdminListQuery{ExcludeCompanyEmployees: true, Type: "user", Limit: 1})
	if err != nil || adminUsers.Total != 2 || len(adminUsers.Users) != 1 {
		t.Fatalf("filtered users = %+v, err = %v", adminUsers, err)
	}
	listed := adminUsers.Users[0]
	if listed.CompanyRole != model.CompanyMemberRoleAdmin || listed.CompanyName != listed.Username {
		t.Fatalf("expected a company administrator with company name, got %q / %q", listed.CompanyRole, listed.CompanyName)
	}
	allUsers, err := s.AdminUsers(platform, AdminListQuery{Keyword: employee.Username})
	if err != nil || allUsers.Total != 1 || len(allUsers.Users) != 1 || allUsers.Users[0].ID != employee.UserID {
		t.Fatalf("unfiltered employee search = %+v, err = %v", allUsers, err)
	}
	companies, err := s.AdminCompanies(platform)
	if err != nil {
		t.Fatal(err)
	}
	for _, company := range companies {
		if company.Name != adminA.Username {
			continue
		}
		if len(company.Members) != 2 || company.Members[0].Role != model.CompanyMemberRoleAdmin || company.Members[1].UserID != employee.UserID ||
			company.Members[1].Role != model.CompanyMemberRoleEmployee {
			t.Fatalf("company member list = %+v", company.Members)
		}
	}
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
	companyID := membersA.Company.CompanyID
	filtered, err := s.AdminUsers(platform, AdminListQuery{CompanyID: companyID, CompanyRole: "employee", Status: "disabled"})
	if err != nil || filtered.Total != 1 || filtered.Users[0].ID != employee.UserID {
		t.Fatalf("company employee filter = %+v, err = %v", filtered, err)
	}
	if _, err := s.AddAdminCompanyAdmin(adminA, companyID, user.Username); err == nil {
		t.Fatal("company administrator appointed another administrator")
	}
	if _, err := s.AddAdminCompanyAdmin(platform, companyID, adminB.Username); err == nil {
		t.Fatal("company accepted an account from another company")
	}
	if err := s.DeleteUser(platform, adminA.ID); err == nil {
		t.Fatal("disabled the only administrator")
	}
	if _, err := s.UpdateUser(platform, adminA.ID, UpdateUserRequest{Status: model.UserStatusDisabled}); err == nil {
		t.Fatal("updated the only administrator to disabled")
	}
	if _, err := s.DemoteAdminCompanyAdmin(platform, companyID, adminA.ID); err == nil {
		t.Fatal("demoted the only administrator")
	}
	alternate := &model.User{ID: "alternate", Username: "alternate", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(alternate).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddAdminCompanyAdmin(platform, companyID, alternate.Username); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BulkDisableUsers(platform, BulkDisableUsersRequest{UserIDs: []string{adminA.ID, alternate.ID}}); err == nil {
		t.Fatal("bulk-disabled all company administrators")
	}
	session := &model.AuthSession{ID: "company-session", UserID: adminA.ID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := s.repo.CreateAuthSessionForUser(adminA, session); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateAdminCompanyStatus(platform, companyID, model.UserStatusDisabled); err != nil {
		t.Fatal(err)
	}
	if _, err := s.repo.AuthSession(session.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("company session was not revoked: %v", err)
	}
	if err := s.repo.CreateAuthSessionForUser(adminA, session); !errors.Is(err, repository.ErrCompanyDisabled) {
		t.Fatalf("disabled company session creation: %v", err)
	}
	if _, err := s.UpdateAdminCompanyStatus(platform, companyID, model.UserStatusActive); err != nil {
		t.Fatal(err)
	}
	if err := s.repo.CreateAuthSessionForUser(adminA, session); err != nil {
		t.Fatal(err)
	}
	user, err = s.repo.User(employee.UserID)
	if err != nil || user.Status != model.UserStatusDisabled {
		t.Fatalf("restoration changed disabled employee: %+v, %v", user, err)
	}
}
