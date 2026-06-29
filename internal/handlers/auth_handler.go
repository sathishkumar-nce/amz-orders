package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sathishkumar-nce/amz-orders/internal/config"
	"github.com/sathishkumar-nce/amz-orders/internal/models"
	"github.com/sathishkumar-nce/amz-orders/internal/repository"
	"github.com/sathishkumar-nce/amz-orders/internal/utils"
)

type AuthHandler struct {
	userRepo *repository.UserRepository
	config   *config.Config
}

func NewAuthHandler(userRepo *repository.UserRepository, config *config.Config) *AuthHandler {
	return &AuthHandler{
		userRepo: userRepo,
		config:   config,
	}
}

// Register creates a new user account
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Register payload invalid: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Printf("👤 Register requested (username=%s email=%s)", req.Username, req.Email)

	user, err := h.userRepo.CreateUser(c.Request.Context(), &req, req.Username, false)
	if err != nil {
		log.Printf("❌ Register failed (username=%s): %v", req.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user: " + err.Error()})
		return
	}
	log.Printf("✅ Register completed (username=%s user_id=%d)", user.Username, user.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message": "user created successfully",
		"user":    user,
	})
}

// Login authenticates a user and returns a JWT token
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Login payload invalid: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	log.Printf("🔐 Login requested (username=%s)", req.Username)

	user, err := h.userRepo.GetUserByUsername(c.Request.Context(), req.Username)
	if err != nil {
		log.Printf("❌ Login failed during lookup (username=%s): %v", req.Username, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	if err := h.userRepo.VerifyPassword(user.Password, req.Password); err != nil {
		log.Printf("❌ Login failed due to password mismatch (username=%s)", req.Username)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	token, expiresAt, err := utils.GenerateToken(user, h.config.JWTSecret, h.config.JWTExpirationHours)
	if err != nil {
		log.Printf("❌ Login token generation failed (username=%s): %v", req.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	log.Printf("✅ Login completed (username=%s user_id=%d expires_at=%d)", user.Username, user.ID, expiresAt)

	c.JSON(http.StatusOK, models.LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      *user,
	})
}

// Me returns the current authenticated user's information
func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		log.Printf("❌ /auth/me called without user_id in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	log.Printf("👤 /auth/me requested (user_id=%v)", userID)

	user, err := h.userRepo.GetUserByID(c.Request.Context(), userID.(int64))
	if err != nil {
		log.Printf("❌ /auth/me failed (user_id=%v): %v", userID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	log.Printf("✅ /auth/me completed (user_id=%d username=%s)", user.ID, user.Username)

	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) ListUsers(c *gin.Context) {
	if !currentActorIsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	users, err := h.userRepo.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *AuthHandler) AdminCreateUser(c *gin.Context) {
	if !currentActorIsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	var req models.AdminCreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userRepo.AdminCreateUser(c.Request.Context(), &req, currentActorName(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to create user: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user":             user,
		"default_password": repository.DefaultWelcomePassword,
	})
}

func (h *AuthHandler) DeleteUser(c *gin.Context) {
	if !currentActorIsAdmin(c) {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
		return
	}

	userIDValue, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	targetUserID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil || targetUserID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	if currentUserID, ok := userIDValue.(int64); ok && currentUserID == targetUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "you cannot delete the currently logged in admin user"})
		return
	}

	if err := h.userRepo.DeleteUser(c.Request.Context(), targetUserID); err != nil {
		if err == repository.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user deleted successfully"})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.userRepo.ChangePassword(c.Request.Context(), userID.(int64), req.CurrentPassword, req.NewPassword, currentActorName(c)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userRepo.GetUserByID(c.Request.Context(), userID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "password changed but user refresh failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "password changed successfully",
		"user":    user,
	})
}

func currentActorName(c *gin.Context) string {
	if username, exists := c.Get("username"); exists {
		if value, ok := username.(string); ok && value != "" {
			return value
		}
	}
	return "system"
}

func currentActorIsAdmin(c *gin.Context) bool {
	return currentActorName(c) == "admin"
}
