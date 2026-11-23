package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"yingce/backend/internal/model"
	"yingce/backend/internal/service"
)

func registerCompanyRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/company/membership", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		view, err := svc.CompanyMembership(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, view)
	})
	r.GET("/company/members", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		view, err := svc.CompanyMembers(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, view)
	})
	r.POST("/company/members", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		var req service.CreateCompanyMemberRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		member, err := svc.CreateCompanyMember(user, req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"member": member})
	})
	r.POST("/company/members/:id/disable", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		member, err := svc.DisableCompanyMember(user, c.Param("id"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"member": member})
	})
	r.POST("/company/members/:id/credits", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		var req service.CompanyCreditAllocationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		result, err := svc.AllocateCompanyCredits(user, c.Param("id"), req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	r.GET("/admin/companies", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		companies, err := svc.AdminCompanies(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"companies": companies})
	})
	r.POST("/admin/companies", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		var req service.CreateCompanyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		company, err := svc.CreateAdminCompany(user, req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"company": company})
	})
	r.PATCH("/admin/companies/:id", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		var req struct {
			Status model.UserStatus `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		company, err := svc.UpdateAdminCompanyStatus(user, c.Param("id"), req.Status)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"company": company})
	})
	r.POST("/admin/companies/:id/admins", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		var req struct {
			UsernameOrEmail string `json:"usernameOrEmail"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		member, err := svc.AddAdminCompanyAdmin(user, c.Param("id"), req.UsernameOrEmail)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"member": member})
	})
	r.DELETE("/admin/companies/:id/admins/:userId", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		member, err := svc.DemoteAdminCompanyAdmin(user, c.Param("id"), c.Param("userId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"member": member})
	})
}
