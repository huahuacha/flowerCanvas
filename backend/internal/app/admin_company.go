package app

import (
	"errors"
	"strings"

	"gorm.io/gorm"
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

func (s *Service) UpdateAdminCompanyStatus(actor *model.User, companyID string, status model.UserStatus) (*AdminCompanyView, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	if status != model.UserStatusActive && status != model.UserStatusDisabled {
		return nil, BadAuthRequest("公司状态无效")
	}
	var result *AdminCompanyView
	err := s.repo.Transaction(func(repo *repository.Repository) error {
		currentActor, err := repo.LockUserForUpdate(actor.ID)
		if err != nil {
			return err
		}
		if err := s.RequireAdmin(currentActor); err != nil {
			return err
		}
		if currentActor.Status != model.UserStatusActive {
			return Forbidden("需要启用的平台管理员账号")
		}
		company, err := repo.LockCompany(companyID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NotFound("公司不存在")
		}
		if err != nil {
			return err
		}
		if company.Status != status {
			if status == model.UserStatusActive {
				if err := repo.RequireCompanyAdminRemaining(companyID, nil); err != nil {
					return err
				}
			}
			if err := repo.SetCompanyStatus(company, status); err != nil {
				return err
			}
			event, err := newAdminAuditEvent(actor, "company.status", "company", companyID, "更新公司状态", map[string]model.UserStatus{"status": status})
			if err != nil {
				return err
			}
			if err := repo.AppendAdminAudit(event); err != nil {
				return err
			}
		}
		views, err := adminCompanyViews(repo, []model.Company{*company})
		if err != nil {
			return err
		}
		result = &views[0]
		return nil
	})
	return result, companyWriteError(err)
}

func (s *Service) AddAdminCompanyAdmin(actor *model.User, companyID, account string) (*CompanyMemberBrief, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	account = strings.TrimSpace(account)
	if account == "" {
		return nil, BadAuthRequest("请输入用户名或邮箱")
	}
	var user *model.User
	var err error
	if strings.Contains(account, "@") {
		user, err = s.repo.UserByEmail(normalizeEmail(account))
	} else {
		user, err = s.repo.UserByUsername(normalizeUsername(account))
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("账号不存在")
	}
	if err != nil {
		return nil, err
	}
	return s.changeCompanyAdminRole(actor, companyID, user.ID, model.CompanyMemberRoleAdmin)
}

func (s *Service) DemoteAdminCompanyAdmin(actor *model.User, companyID, userID string) (*CompanyMemberBrief, error) {
	return s.changeCompanyAdminRole(actor, companyID, userID, model.CompanyMemberRoleEmployee)
}

func (s *Service) changeCompanyAdminRole(actor *model.User, companyID, userID string, role model.CompanyMemberRole) (*CompanyMemberBrief, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	if actor.ID == userID {
		return nil, BadAuthRequest("平台管理员不能加入公司")
	}
	var result *CompanyMemberBrief
	err := s.repo.Transaction(func(repo *repository.Repository) error {
		currentActor, err := repo.LockUserForUpdate(actor.ID)
		if err != nil {
			return err
		}
		if err := s.RequireAdmin(currentActor); err != nil {
			return err
		}
		if currentActor.Status != model.UserStatusActive {
			return Forbidden("需要启用的平台管理员账号")
		}
		user, err := repo.LockUserForUpdate(userID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NotFound("账号不存在")
		}
		if err != nil {
			return err
		}
		company, err := repo.LockCompany(companyID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NotFound("公司不存在")
		}
		if err != nil {
			return err
		}
		member, err := repo.CompanyMemberByUserID(userID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if member != nil && member.CompanyID != company.ID {
			return BadAuthRequest("该账号已归属其他公司")
		}
		if role == model.CompanyMemberRoleAdmin {
			if user.Role != model.UserRoleUser || user.Status != model.UserStatusActive {
				return BadAuthRequest("请选择已启用的普通平台账号")
			}
			authorized, err := s.fixedChannelGroupCodeAuthorized(repo, normalizeUserGroupCode(user.UserGroup))
			if err != nil {
				return err
			}
			if !authorized {
				return BadAuthRequest("该账号的用户分组尚未获得固定渠道授权")
			}
		} else {
			if member == nil || member.Role != model.CompanyMemberRoleAdmin {
				return BadAuthRequest("该账号不是本公司的管理员")
			}
			if company.Status == model.UserStatusActive {
				if err := repo.RequireCompanyAdminRemaining(company.ID, []string{userID}); err != nil {
					return err
				}
			}
		}
		result = &CompanyMemberBrief{UserID: user.ID, Username: user.Username, DisplayName: user.DisplayName, Role: role, Status: user.Status}
		if member == nil {
			member = &model.CompanyMember{ID: newID(), CompanyID: companyID, UserID: userID, Role: role}
			if err := repo.Create(member); err != nil {
				return err
			}
		} else if member.Role == role {
			return nil
		} else {
			member.Role = role
			if err := repo.Save(member); err != nil {
				return err
			}
		}
		if err := repo.DeleteUserAuthSessions(userID); err != nil {
			return err
		}
		event, err := newAdminAuditEvent(actor, "company.member.role", "user", userID, "更新公司成员身份", map[string]string{"companyId": companyID, "role": string(role)})
		if err != nil {
			return err
		}
		if err := repo.AppendAdminAudit(event); err != nil {
			return err
		}
		return nil
	})
	return result, companyWriteError(err)
}
