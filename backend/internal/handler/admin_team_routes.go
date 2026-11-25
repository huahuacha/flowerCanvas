package handler

import (
	"github.com/gin-gonic/gin"
	"yingce/backend/internal/service"
)

func registerAdminTeamRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/admin/teams", func(c *gin.Context) {
		actor, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		teams, err := svc.AdminTeams(actor)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"teams": teams})
	})
	r.GET("/admin/teams/:id/members", func(c *gin.Context) {
		actor, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		members, err := svc.AdminTeamMembers(actor, c.Param("id"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"members": members})
	})
}
