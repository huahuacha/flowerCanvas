package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"yingce/backend/internal/service"
)

func registerFixedChannelRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/fixed-channels", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		channels, err := svc.FixedChannels(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, channels)
	})
	r.PUT("/fixed-channels/:id/credential", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var req struct {
			APIKey string `json:"apiKey"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
		if err := c.ShouldBindJSON(&req); err != nil {
			failService(c, service.BadAuthRequest("固定渠道凭据格式无效"))
			return
		}
		channel, err := svc.SaveFixedChannelCredential(user, c.Param("id"), req.APIKey)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, channel)
	})
	r.DELETE("/fixed-channels/:id/credential", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		if err := svc.DeleteFixedChannelCredential(user, c.Param("id")); err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"ok": true})
	})
}
