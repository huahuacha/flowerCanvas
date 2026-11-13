package app

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

// The allocation amount is a JSON number and must round-trip exactly in JavaScript.
const maxCompanyAllocationMicrocredits = 1<<53 - 1

var companyRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,120}$`)

type CompanyIdentity struct {
	CompanyID   string                  `json:"companyId"`
	CompanyName string                  `json:"companyName"`
	Role        model.CompanyMemberRole `json:"role"`
	IsAdmin     bool                    `json:"isAdmin"`
}

type CompanyMembershipView struct {
	Company *CompanyIdentity `json:"company"`
}

type CompanyMemberBrief struct {
	UserID      string                  `json:"userId"`
	Username    string                  `json:"username"`
	DisplayName string                  `json:"displayName"`
	Role        model.CompanyMemberRole `json:"role"`
	Status      model.UserStatus        `json:"status"`
}

type CompanyMemberView struct {
	CompanyMemberBrief
	Email                 string    `json:"email,omitempty"`
	AvailableMicrocredits int64     `json:"availableMicrocredits"`
	ReservedMicrocredits  int64     `json:"reservedMicrocredits"`
	JoinedAt              time.Time `json:"joinedAt"`
}

type CompanyMembersView struct {
	Company                 CompanyIdentity     `json:"company"`
	Members                 []CompanyMemberView `json:"members"`
	MyAvailableMicrocredits int64               `json:"myAvailableMicrocredits"`
	MyReservedMicrocredits  int64               `json:"myReservedMicrocredits"`
}

type CreateCompanyMemberRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

type CompanyCreditAllocationRequest struct {
	AmountMicrocredits int64  `json:"amountMicrocredits"`
	RequestID          string `json:"requestId"`
}

type AdminCompanyView struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	MemberCount int                  `json:"memberCount"`
	AdminCount  int                  `json:"adminCount"`
	Admins      []CompanyMemberBrief `json:"admins"`
	CreatedAt   time.Time            `json:"createdAt"`
}

type CreateCompanyRequest struct {
	Name          string `json:"name"`
	AdminUserID   string `json:"adminUserId"`
	AdminUsername string `json:"adminUsername"`
}

func companyWriteError(err error) error {
	switch {
	case errors.Is(err, repository.ErrCompanyAdminRequired), errors.Is(err, repository.ErrCompanyEmployeeRequired):
		return Forbidden(err.Error())
	case errors.Is(err, repository.ErrCompanyTransferConflict):
		return NewAppError(http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrInsufficientCredits):
		return BadAuthRequest("可用积分不足")
	case errors.Is(err, repository.ErrCompanyUsernameUsed), errors.Is(err, repository.ErrCompanyTransferOverflow):
		return BadAuthRequest(err.Error())
	default:
		return err
	}
}

func (s *Service) CompanyMembership(actor *model.User) (*CompanyMembershipView, error) {
	if actor == nil {
		return nil, Unauthorized("请先登录")
	}
	view := &CompanyMembershipView{}
	if actor.Role != model.UserRoleUser || actor.Status != model.UserStatusActive {
		return view, nil
	}
	member, err := s.repo.CompanyMemberByUserID(actor.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return view, nil
	}
	if err != nil {
		return nil, err
	}
	company, err := s.repo.CompanyByID(member.CompanyID)
	if err != nil {
		return nil, err
	}
	view.Company = &CompanyIdentity{
		CompanyID: company.ID, CompanyName: company.Name, Role: member.Role, IsAdmin: member.Role == model.CompanyMemberRoleAdmin,
	}
	return view, nil
}

func (s *Service) requireCompanyAdmin(actor *model.User) (*CompanyIdentity, error) {
	view, err := s.CompanyMembership(actor)
	if err != nil {
		return nil, err
	}
	if view.Company == nil || !view.Company.IsAdmin {
		return nil, Forbidden("需要公司管理员权限")
	}
	return view.Company, nil
}

func (s *Service) CompanyMembers(actor *model.User) (*CompanyMembersView, error) {
	company, err := s.requireCompanyAdmin(actor)
	if err != nil {
		return nil, err
	}
	members, err := s.repo.CompanyMembers([]string{company.CompanyID})
	if err != nil {
		return nil, err
	}
	view := &CompanyMembersView{Company: *company, Members: make([]CompanyMemberView, 0, len(members))}
	for _, member := range members {
		view.Members = append(view.Members, CompanyMemberView{
			CompanyMemberBrief: CompanyMemberBrief{UserID: member.UserID, Username: member.Username, DisplayName: member.DisplayName, Role: member.Role, Status: member.Status},
			Email:              member.Email, AvailableMicrocredits: member.AvailableMicrocredits, ReservedMicrocredits: member.ReservedMicrocredits, JoinedAt: member.CreatedAt,
		})
		if member.UserID == actor.ID {
			view.MyAvailableMicrocredits = member.AvailableMicrocredits
			view.MyReservedMicrocredits = member.ReservedMicrocredits
		}
	}
	return view, nil
}

func (s *Service) CreateCompanyMember(actor *model.User, req CreateCompanyMemberRequest) (*CompanyMemberView, error) {
	company, err := s.requireCompanyAdmin(actor)
	if err != nil {
		return nil, err
	}
	username, email := normalizeUsername(req.Username), normalizeEmail(req.Email)
	if err := validateUsername(username); err != nil {
		return nil, err
	}
	if err := validatePassword(req.Password); err != nil {
		return nil, err
	}
	if email != "" {
		if utf8.RuneCountInString(email) > 160 {
			return nil, BadAuthRequest("邮箱地址不能超过 160 个字符")
		}
		if err := validateEmail(email); err != nil {
			return nil, err
		}
	}
	passwordHash, err := hashPassword(req.Password)
	if errors.Is(err, bcrypt.ErrPasswordTooLong) {
		return nil, BadAuthRequest("密码不能超过 72 字节")
	}
	if err != nil {
		return nil, err
	}
	user := &model.User{
		ID: newID(), Username: username, Email: email, DisplayName: normalizeDisplayName(req.DisplayName, username),
		Role: model.UserRoleUser, Status: model.UserStatusActive, UserGroup: model.DefaultUserGroupCode, PasswordHash: passwordHash,
	}
	member := &model.CompanyMember{ID: newID(), CompanyID: company.CompanyID, UserID: user.ID, Role: model.CompanyMemberRoleEmployee}
	event, err := newAdminAuditEvent(actor, "company.member.create", "user", user.ID, "公司开通员工账号", map[string]string{"companyId": company.CompanyID})
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateCompanyEmployee(actor.ID, user, member, event); err != nil {
		return nil, companyWriteError(err)
	}
	return companyEmployeeView(user, member, &model.CreditAccount{}), nil
}

func (s *Service) DisableCompanyMember(actor *model.User, userID string) (*CompanyMemberView, error) {
	company, err := s.requireCompanyAdmin(actor)
	if err != nil {
		return nil, err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, BadAuthRequest("请选择要停用的员工")
	}
	event, err := newAdminAuditEvent(actor, "company.member.disable", "user", userID, "公司停用员工并撤销登录会话", map[string]string{"companyId": company.CompanyID})
	if err != nil {
		return nil, err
	}
	user, member, err := s.repo.DisableCompanyEmployee(company.CompanyID, actor.ID, userID, event)
	if err != nil {
		return nil, companyWriteError(err)
	}
	accounts, err := s.repo.CreditAccounts([]string{userID})
	if err != nil {
		return nil, err
	}
	var account model.CreditAccount
	if len(accounts) == 1 {
		account = accounts[0]
	}
	return companyEmployeeView(user, member, &account), nil
}

func companyEmployeeView(user *model.User, member *model.CompanyMember, account *model.CreditAccount) *CompanyMemberView {
	return &CompanyMemberView{
		CompanyMemberBrief: CompanyMemberBrief{UserID: user.ID, Username: user.Username, DisplayName: user.DisplayName, Role: member.Role, Status: user.Status},
		Email:              user.Email, AvailableMicrocredits: account.AvailableMicrocredits, ReservedMicrocredits: account.ReservedMicrocredits, JoinedAt: member.CreatedAt,
	}
}

func (s *Service) AllocateCompanyCredits(actor *model.User, userID string, req CompanyCreditAllocationRequest) (*repository.CompanyTransferResult, error) {
	company, err := s.requireCompanyAdmin(actor)
	if err != nil {
		return nil, err
	}
	if req.AmountMicrocredits <= 0 || req.AmountMicrocredits > maxCompanyAllocationMicrocredits {
		return nil, BadAuthRequest("分配金额必须为正数且在可精确表示的范围内")
	}
	requestID := strings.TrimSpace(req.RequestID)
	if !companyRequestIDPattern.MatchString(requestID) {
		return nil, BadAuthRequest("分配请求缺少有效的 requestId")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, BadAuthRequest("请选择要分配积分的员工")
	}
	event, err := newAdminAuditEvent(actor, "company.credits.allocate", "user", userID, "公司管理员向员工分配积分", map[string]any{
		"companyId": company.CompanyID, "amountMicrocredits": req.AmountMicrocredits, "requestId": requestID,
	})
	if err != nil {
		return nil, err
	}
	result, err := s.repo.AllocateCompanyCredits(company.CompanyID, actor.ID, userID, req.AmountMicrocredits, requestID, event)
	return result, companyWriteError(err)
}

func (s *Service) AdminCompanies(actor *model.User) ([]AdminCompanyView, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	companies, err := s.repo.Companies()
	if err != nil {
		return nil, err
	}
	views := make([]AdminCompanyView, 0, len(companies))
	companyIDs := make([]string, 0, len(companies))
	viewIndex := make(map[string]int, len(companies))
	for _, company := range companies {
		companyIDs = append(companyIDs, company.ID)
		viewIndex[company.ID] = len(views)
		views = append(views, AdminCompanyView{ID: company.ID, Name: company.Name, CreatedAt: company.CreatedAt, Admins: []CompanyMemberBrief{}})
	}
	members, err := s.repo.CompanyMembers(companyIDs)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		view := &views[viewIndex[member.CompanyID]]
		view.MemberCount++
		if member.Role == model.CompanyMemberRoleAdmin {
			view.AdminCount++
			view.Admins = append(view.Admins, CompanyMemberBrief{UserID: member.UserID, Username: member.Username, DisplayName: member.DisplayName, Role: member.Role, Status: member.Status})
		}
	}
	return views, nil
}

func (s *Service) CreateAdminCompany(actor *model.User, req CreateCompanyRequest) (*AdminCompanyView, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > 120 {
		return nil, BadAuthRequest("请填写不超过 120 个字符的公司名称")
	}
	admin, err := s.findInitialCompanyAdmin(req)
	if err != nil {
		return nil, err
	}
	company := &model.Company{ID: newID(), Name: name}
	member := &model.CompanyMember{ID: newID(), CompanyID: company.ID, UserID: admin.ID, Role: model.CompanyMemberRoleAdmin}
	event, err := newAdminAuditEvent(actor, "company.create", "company", company.ID, "平台开通公司并绑定首位公司管理员", map[string]string{"adminUserId": admin.ID})
	if err != nil {
		return nil, err
	}
	err = s.repo.Transaction(func(repo *repository.Repository) error {
		currentActor, err := repo.LockUserForUpdate(actor.ID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Forbidden("需要管理员权限")
		}
		if err != nil {
			return err
		}
		if currentActor.Status != model.UserStatusActive || currentActor.Role != model.UserRoleAdmin {
			return Forbidden("需要管理员权限")
		}
		admin, err = repo.LockUserForUpdate(member.UserID)
		if err != nil {
			return err
		}
		if admin.Role != model.UserRoleUser || admin.Status != model.UserStatusActive {
			return BadAuthRequest("首位公司管理员必须是启用的全站普通用户")
		}
		if _, err := repo.CompanyMemberByUserID(admin.ID); err == nil {
			return BadAuthRequest("该账号已归属公司")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		groups := []string{model.DefaultUserGroupCode}
		if adminGroup := normalizeUserGroupCode(admin.UserGroup); adminGroup != model.DefaultUserGroupCode {
			groups = append(groups, adminGroup)
		}
		for _, group := range groups {
			authorized, err := s.fixedChannelGroupCodeAuthorized(repo, group)
			if err != nil {
				return err
			}
			if !authorized {
				return BadAuthRequest("用户分组 " + group + " 尚未获得固定渠道授权")
			}
		}
		if err := repo.Create(company); err != nil {
			return err
		}
		if err := repo.Create(member); err != nil {
			return err
		}
		return repo.Create(event)
	})
	if err != nil {
		return nil, err
	}
	return &AdminCompanyView{
		ID: company.ID, Name: company.Name, MemberCount: 1, AdminCount: 1, CreatedAt: company.CreatedAt,
		Admins: []CompanyMemberBrief{{UserID: admin.ID, Username: admin.Username, DisplayName: admin.DisplayName, Role: member.Role, Status: admin.Status}},
	}, nil
}

func (s *Service) findInitialCompanyAdmin(req CreateCompanyRequest) (*model.User, error) {
	id, account := strings.TrimSpace(req.AdminUserID), strings.TrimSpace(req.AdminUsername)
	var admin *model.User
	var err error
	switch {
	case id != "" && account != "":
		return nil, BadAuthRequest("请仅指定一个首位公司管理员账号")
	case id != "":
		admin, err = s.repo.User(id)
	case strings.Contains(account, "@"):
		admin, err = s.repo.UserByEmail(normalizeEmail(account))
	case account != "":
		admin, err = s.repo.UserByUsername(normalizeUsername(account))
	default:
		return nil, BadAuthRequest("请指定首位公司管理员的账号")
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, BadAuthRequest("指定的公司管理员账号不存在")
	}
	if err != nil {
		return nil, err
	}
	if admin.Role != model.UserRoleUser || admin.Status != model.UserStatusActive {
		return nil, BadAuthRequest("首位公司管理员必须是启用的全站普通用户")
	}
	return admin, nil
}
