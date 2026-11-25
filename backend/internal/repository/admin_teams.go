package repository

import "yingce/backend/internal/model"

type AdminTeamView struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	ParentID         string `json:"parentId,omitempty"`
	OwnerUserID      string `json:"ownerUserId"`
	Status           string `json:"status"`
	OwnerUsername    string `json:"ownerUsername"`
	OwnerDisplayName string `json:"ownerDisplayName"`
}

func (r *Repository) ListAdminTeams() ([]AdminTeamView, error) {
	teams := []AdminTeamView{}
	err := r.db.Model(&model.Team{}).
		Select("teams.id, teams.name, teams.parent_id, teams.owner_user_id, teams.status, COALESCE(owner.username, '') AS owner_username, COALESCE(owner.display_name, '') AS owner_display_name").
		Joins("LEFT JOIN users AS owner ON owner.id = teams.owner_user_id").
		Order("teams.created_at, teams.id").
		Scan(&teams).Error
	return teams, err
}

func (r *Repository) RequireTeamByID(teamID string) error {
	return r.db.Select("id").Take(&model.Team{}, "id = ?", teamID).Error
}
