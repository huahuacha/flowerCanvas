package repository

import (
	"errors"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yingce/backend/internal/model"
)

var (
	ErrCompanyAdminRequired    = errors.New("需要公司管理员权限")
	ErrCompanyEmployeeRequired = errors.New("目标账号不是本公司的普通员工")
	ErrCompanyUsernameUsed     = errors.New("用户名已存在")
	ErrCompanyTransferConflict = errors.New("同一分配请求的接收人或金额不一致")
	ErrCompanyTransferOverflow = errors.New("员工可用积分超出余额上限")
)

type CompanyTransferResult struct {
	ActorAccount  model.CreditAccount `json:"actorAccount"`
	TargetAccount model.CreditAccount `json:"targetAccount"`
	Replayed      bool                `json:"replayed"`
}

func (r *Repository) CompanyMemberByUserID(userID string) (*model.CompanyMember, error) {
	var member model.CompanyMember
	if err := r.db.First(&member, "user_id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *Repository) CompanyByID(id string) (*model.Company, error) {
	var company model.Company
	if err := r.db.First(&company, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &company, nil
}

func (r *Repository) UserCompany(userID string) (UserCompany, error) {
	var company UserCompany
	err := r.db.Table("company_members AS m").
		Select("c.name AS company_name, m.role AS company_role, c.status AS company_status").
		Joins("LEFT JOIN companies AS c ON c.id = m.company_id").
		Where("m.user_id = ?", userID).Scan(&company).Error
	return company, err
}

var ErrCompanyDisabled = errors.New("公司已停用")
var ErrLastCompanyAdmin = errors.New("启用中的公司至少需要一位可用管理员")
var ErrCompanyPlatformRole = errors.New("公司成员必须使用普通平台账号")

// LockCompany serializes changes to membership, company status and session issuance.
func (r *Repository) LockCompany(companyID string) (*model.Company, error) {
	if r.Dialect() == "sqlite" {
		if err := r.db.Model(&model.Company{}).Where("id = ?", companyID).UpdateColumn("status", gorm.Expr("status")).Error; err != nil {
			return nil, err
		}
	}
	var company model.Company
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&company, "id = ?", companyID).Error; err != nil {
		return nil, err
	}
	return &company, nil
}

func (r *Repository) requireActiveCompany(companyID string) error {
	company, err := r.LockCompany(companyID)
	if err != nil {
		return err
	}
	if company.Status != model.UserStatusActive {
		return ErrCompanyDisabled
	}
	return nil
}

func (r *Repository) RequireCompanyAdminRemaining(companyID string, excludedUserIDs []string) error {
	var count int64
	query := r.db.Table("company_members AS m").Joins("JOIN users AS u ON u.id = m.user_id").
		Where("m.company_id = ? AND m.role = ? AND u.role = ? AND u.status = ?", companyID, model.CompanyMemberRoleAdmin, model.UserRoleUser, model.UserStatusActive)
	if len(excludedUserIDs) != 0 {
		query = query.Where("m.user_id NOT IN ?", excludedUserIDs)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrLastCompanyAdmin
	}
	return nil
}

// CheckCompanyUserChanges runs after locking the affected user rows.
func (r *Repository) CheckCompanyUserChanges(userIDs []string, role model.UserRole, status model.UserStatus) error {
	if role != model.UserRoleAdmin && status != model.UserStatusDisabled {
		return nil
	}
	var memberships []model.CompanyMember
	if err := r.db.Where("user_id IN ?", userIDs).Order("company_id, user_id").Find(&memberships).Error; err != nil {
		return err
	}
	if len(memberships) > 0 && role == model.UserRoleAdmin {
		return ErrCompanyPlatformRole
	}
	var checkedCompany string
	for _, member := range memberships {
		if status != model.UserStatusDisabled || member.Role != model.CompanyMemberRoleAdmin || member.CompanyID == checkedCompany {
			continue
		}
		company, err := r.LockCompany(member.CompanyID)
		if err != nil {
			return err
		}
		if company.Status == model.UserStatusActive {
			if err := r.RequireCompanyAdminRemaining(company.ID, userIDs); err != nil {
				return err
			}
		}
		checkedCompany = company.ID
	}
	return nil
}

func (r *Repository) SetCompanyStatus(company *model.Company, status model.UserStatus) error {
	if err := r.db.Model(company).Update("status", status).Error; err != nil {
		return err
	}
	if status == model.UserStatusDisabled {
		members := r.db.Model(&model.CompanyMember{}).Select("user_id").Where("company_id = ?", company.ID)
		return r.db.Delete(&model.AuthSession{}, "user_id IN (?)", members).Error
	}
	return nil
}

func (r *Repository) LockCompanyAccountEmail(email string) error {
	if email == "" {
		return nil
	}
	return lockRegistrationEmail(r.db, email)
}

func (r *Repository) Companies() ([]model.Company, error) {
	var companies []model.Company
	err := r.db.Order("created_at DESC, id ASC").Find(&companies).Error
	return companies, err
}

type CompanyMemberRecord struct {
	model.CompanyMember
	Username              string
	DisplayName           string
	Email                 string
	Status                model.UserStatus
	AvailableMicrocredits int64
	ReservedMicrocredits  int64
}

func (r *Repository) CompanyMembers(companyIDs []string) ([]CompanyMemberRecord, error) {
	var members []CompanyMemberRecord
	err := r.db.Table("company_members AS m").
		Select("m.*, u.username, u.display_name, u.email, u.status, COALESCE(a.available_microcredits, 0) AS available_microcredits, COALESCE(a.reserved_microcredits, 0) AS reserved_microcredits").
		Joins("JOIN users u ON u.id = m.user_id").
		Joins("LEFT JOIN credit_accounts a ON a.user_id = m.user_id").
		Where("m.company_id IN ?", companyIDs).
		Order("m.role, m.created_at, m.id").
		Scan(&members).Error
	return members, err
}

func (r *Repository) CreateCompanyEmployee(actorID string, user *model.User, member *model.CompanyMember, event *model.AdminAuditEvent) error {
	return r.Transaction(func(repo *Repository) error {
		// Match the lock order used by email updates and other signup paths.
		if user.Email != "" {
			if err := lockRegistrationEmail(repo.db, user.Email); err != nil {
				return err
			}
		}
		if err := repo.lockCompanyAdmin(member.CompanyID, actorID); err != nil {
			return err
		}
		if err := repo.requireActiveCompany(member.CompanyID); err != nil {
			return err
		}
		return repo.InsertCompanyAccount(user, member, event)
	})
}

// InsertCompanyAccount inserts the account, membership and audit within the caller's transaction.
func (r *Repository) InsertCompanyAccount(user *model.User, member *model.CompanyMember, event *model.AdminAuditEvent) error {
	if r.Dialect() == "postgres" {
		// Account lookup is case-insensitive even though the legacy unique index is not.
		if err := r.db.Exec("SELECT pg_advisory_xact_lock(hashtext(?)::bigint)", "company-username:"+strings.ToLower(user.Username)).Error; err != nil {
			return err
		}
	}
	if err := r.CheckEmailAvailable(user.Email, ""); err != nil {
		return err
	}
	if _, err := r.UserByUsername(user.Username); err == nil {
		return ErrCompanyUsernameUsed
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	insert := r.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "username"}}, DoNothing: true}).Create(user)
	if insert.Error != nil {
		return insert.Error
	}
	if insert.RowsAffected != 1 {
		return ErrCompanyUsernameUsed
	}
	if err := r.Create(member); err != nil {
		return err
	}
	if err := r.Create(&model.CreditAccount{UserID: user.ID}); err != nil {
		return err
	}
	return r.Create(event)
}

// LockUserForUpdate runs inside a transaction and serializes with status
// changes and login session issuance. SQLite must acquire the write lock
// before the first read to avoid upgrading a stale WAL snapshot.
func (r *Repository) LockUserForUpdate(userID string) (*model.User, error) {
	if err := lockProfileUser(r.db, userID); err != nil {
		return nil, err
	}
	var user model.User
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) lockCompanyAdmin(companyID, actorID string) error {
	actor, err := r.LockUserForUpdate(actorID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCompanyAdminRequired
	}
	if err != nil {
		return err
	}
	return r.checkCompanyAdmin(companyID, actor)
}

func (r *Repository) checkCompanyAdmin(companyID string, actor *model.User) error {
	if actor.Role != model.UserRoleUser || actor.Status != model.UserStatusActive {
		return ErrCompanyAdminRequired
	}
	admin, err := r.CompanyMemberByUserID(actor.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrCompanyAdminRequired
	}
	if err != nil {
		return err
	}
	if admin.CompanyID != companyID || admin.Role != model.CompanyMemberRoleAdmin {
		return ErrCompanyAdminRequired
	}
	return nil
}

func (r *Repository) lockCompanyParticipants(companyID, actorID, employeeID string) (*model.User, *model.CompanyMember, error) {
	if actorID == employeeID {
		return nil, nil, ErrCompanyEmployeeRequired
	}
	// Match bulk user operations: take user locks by ID, then the company lock.
	firstID, secondID := actorID, employeeID
	if secondID < firstID {
		firstID, secondID = secondID, firstID
	}
	first, err := r.LockUserForUpdate(firstID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrCompanyEmployeeRequired
		}
		return nil, nil, err
	}
	second, err := r.LockUserForUpdate(secondID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrCompanyEmployeeRequired
		}
		return nil, nil, err
	}
	actor, user := first, second
	if firstID != actorID {
		actor, user = second, first
	}
	if err := r.checkCompanyAdmin(companyID, actor); err != nil {
		return nil, nil, err
	}
	if user.Role != model.UserRoleUser {
		return nil, nil, ErrCompanyEmployeeRequired
	}
	employee, err := r.CompanyMemberByUserID(employeeID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, ErrCompanyEmployeeRequired
	}
	if err != nil {
		return nil, nil, err
	}
	if employee.CompanyID != companyID || employee.Role != model.CompanyMemberRoleEmployee {
		return nil, nil, ErrCompanyEmployeeRequired
	}
	if err := r.requireActiveCompany(companyID); err != nil {
		return nil, nil, err
	}
	return user, employee, nil
}

func (r *Repository) DisableCompanyEmployee(companyID, actorID, employeeID string, event *model.AdminAuditEvent) (*model.User, *model.CompanyMember, error) {
	var user *model.User
	var member *model.CompanyMember
	err := r.Transaction(func(repo *Repository) error {
		var err error
		user, member, err = repo.lockCompanyParticipants(companyID, actorID, employeeID)
		if err != nil {
			return err
		}
		if err := repo.DeleteUserAuthSessions(employeeID); err != nil {
			return err
		}
		if user.Status == model.UserStatusDisabled {
			return nil
		}
		if err := repo.db.Model(user).Update("status", model.UserStatusDisabled).Error; err != nil {
			return err
		}
		return repo.Create(event)
	})
	return user, member, err
}

func (r *Repository) AllocateCompanyCredits(companyID, actorID, employeeID string, amount int64, requestID string, event *model.AdminAuditEvent) (*CompanyTransferResult, error) {
	if amount <= 0 {
		return nil, gorm.ErrInvalidData
	}
	var result CompanyTransferResult
	err := r.Transaction(func(repo *Repository) error {
		employee, _, err := repo.lockCompanyParticipants(companyID, actorID, employeeID)
		if err != nil {
			return err
		}
		// The locked sender row serializes requests sharing an idempotency scope.
		// Keep the sender in the scope so request IDs can be reused by other companies.
		base := "company-transfer:" + actorID + ":" + requestID
		outRef, inRef := base+":out", base+":in"
		var existingOut model.CreditLedgerEntry
		err = repo.db.First(&existingOut, "reference_key = ?", outRef).Error
		switch {
		case err == nil:
			var existingIn model.CreditLedgerEntry
			if err := repo.db.First(&existingIn, "reference_key = ?", inRef).Error; err != nil {
				return err
			}
			if existingOut.Type != model.CreditLedgerCompanyOut || existingOut.UserID != actorID || existingOut.AmountMicrocredits != -amount ||
				existingIn.Type != model.CreditLedgerCompanyIn || existingIn.UserID != employeeID || existingIn.AmountMicrocredits != amount {
				return ErrCompanyTransferConflict
			}
			result.Replayed = true
		case errors.Is(err, gorm.ErrRecordNotFound):
			if employee.Status != model.UserStatusActive {
				return ErrCompanyEmployeeRequired
			}
			if err := repo.applyCompanyTransfer(actorID, employeeID, amount); err != nil {
				return err
			}
		default:
			return err
		}
		if err := repo.db.First(&result.ActorAccount, "user_id = ?", actorID).Error; err != nil {
			return err
		}
		if err := repo.db.First(&result.TargetAccount, "user_id = ?", employeeID).Error; err != nil {
			return err
		}
		if result.Replayed {
			return nil
		}
		entries := []model.CreditLedgerEntry{
			newCompanyTransferEntry(result.ActorAccount, model.CreditLedgerCompanyOut, -amount, actorID, &outRef),
			newCompanyTransferEntry(result.TargetAccount, model.CreditLedgerCompanyIn, amount, actorID, &inRef),
		}
		if err := repo.Create(&entries); err != nil {
			return err
		}
		return repo.Create(event)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *Repository) applyCompanyTransfer(actorID, employeeID string, amount int64) error {
	now := time.Now()
	updated := r.db.Model(&model.CreditAccount{}).
		Where("user_id = ? AND available_microcredits >= ?", actorID, amount).
		Updates(map[string]any{
			"available_microcredits": gorm.Expr("available_microcredits - ?", amount),
			"version":                gorm.Expr("version + 1"),
			"updated_at":             now,
		})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ErrInsufficientCredits
	}
	updated = r.db.Model(&model.CreditAccount{}).
		Where("user_id = ? AND available_microcredits <= ?", employeeID, math.MaxInt64-amount).
		Updates(map[string]any{
			"available_microcredits": gorm.Expr("available_microcredits + ?", amount),
			"version":                gorm.Expr("version + 1"),
			"updated_at":             now,
		})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ErrCompanyTransferOverflow
	}
	return nil
}

func newCompanyTransferEntry(account model.CreditAccount, entryType model.CreditLedgerType, delta int64, actorID string, ref *string) model.CreditLedgerEntry {
	return model.CreditLedgerEntry{
		ID: newRepositoryID(), UserID: account.UserID, Type: entryType,
		AmountMicrocredits: delta, AvailableDeltaMicrocredits: delta,
		AvailableAfterMicrocredits: account.AvailableMicrocredits, ReservedAfterMicrocredits: account.ReservedMicrocredits,
		ActorUserID: actorID, ReferenceKey: ref,
	}
}
