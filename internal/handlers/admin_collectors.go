package handlers

import (
	"net/http"

	"backend-pasarata/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type UpdateCollectorRequest struct {
	Username string `json:"username"`
	FullName string `json:"full_name"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type SetCollectorStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func (h *AdminHandler) UpdateCollector(c *gin.Context) {
	id := c.Param("id")
	var req UpdateCollectorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var collector models.User
	if err := h.DB.Where("id = ? AND role = ?", id, models.RoleCollector).First(&collector).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "collector not found"})
		return
	}

	if req.Username != "" && req.Username != collector.Username {
		var existing models.User
		if err := h.DB.Where("username = ?", req.Username).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
			return
		}
		collector.Username = req.Username
	}
	if req.FullName != "" {
		collector.FullName = req.FullName
	}

	if err := h.DB.Save(&collector).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update collector"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "collector updated", "data": collector})
}

func (h *AdminHandler) SetCollectorStatus(c *gin.Context) {
	id := c.Param("id")
	var req SetCollectorStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Status != string(models.UserStatusActive) && req.Status != string(models.UserStatusInactive) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be active or inactive"})
		return
	}

	var collector models.User
	if err := h.DB.Where("id = ? AND role = ?", id, models.RoleCollector).First(&collector).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "collector not found"})
		return
	}

	collector.Status = models.UserStatus(req.Status)
	if err := h.DB.Save(&collector).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update collector status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "collector status updated", "data": collector})
}

func (h *AdminHandler) ResetCollectorPassword(c *gin.Context) {
	id := c.Param("id")
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var collector models.User
	if err := h.DB.Where("id = ? AND role = ?", id, models.RoleCollector).First(&collector).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "collector not found"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), 12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	collector.PasswordHash = string(hash)
	if err := h.DB.Save(&collector).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "collector password reset"})
}
