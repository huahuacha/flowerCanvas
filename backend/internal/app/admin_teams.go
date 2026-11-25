package app

import (
	"yingce/backend/internal/model"
	"yingce/backend/internal/repository"
)

type AdminTeamMember struct {
	ID          string `json:"id"`
	UserID      string `json:"userId"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

func (s *Service) AdminTeams(actor *model.User) ([]repository.AdminTeamView, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	return s.repo.ListAdminTeams()
}

func (s *Service) AdminTeamMembers(actor *model.User, teamID string) ([]AdminTeamMember, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	if err := s.repo.RequireTeamByID(teamID); err != nil {
		return nil, teamNotFound(err)
	}
	rows, err := s.repo.ListTeamMembers(teamID)
	if err != nil {
		return nil, err
	}
	members := make([]AdminTeamMember, 0, len(rows))
	for _, row := range rows {
		members = append(members, AdminTeamMember{
			ID:          row.ID,
			UserID:      row.UserID,
			Username:    row.Username,
			DisplayName: row.DisplayName,
			Role:        row.Role,
			Status:      row.Status,
		})
	}
	return members, nil
}
