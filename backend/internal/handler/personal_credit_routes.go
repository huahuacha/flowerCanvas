package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"yingce/backend/internal/service"
)

func registerPersonalCreditRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.POST("/wallet/self-adjustment", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4<<10)
		var req service.PersonalCreditAdjustmentRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			failService(c, service.BadAuthRequest("请求内容无效或超过大小限制"))
			return
		}
		account, err := svc.AdjustPersonalCredits(c.Request.Context(), user, req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, account)
	})
}
