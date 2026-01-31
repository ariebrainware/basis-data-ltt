package endpoint

import (
	"fmt"
	"strconv"

	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UpdateUserRequest struct {
	Name     string `json:"name" example:"John Doe"`
	Email    string `json:"email" example:"john@example.com"`
	Password string `json:"password" example:"newpassword123"`
}

// validateUpdateRequest checks whether at least one field is provided for update.
func validateUpdateRequest(req *UpdateUserRequest) bool {
	return req.Name != "" || req.Email != "" || req.Password != ""
}

// hashUserPassword generates a salt and hashes the provided password, updating the user model.
func hashUserPassword(c *gin.Context, user *model.User, plainPassword string) error {
	salt, err := util.GenerateSalt()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to generate password salt", Err: err})
		return err
	}

	hashedPassword, err := util.HashPasswordArgon2(plainPassword, salt)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to hash password", Err: err})
		return err
	}

	user.Password = hashedPassword
	user.PasswordSalt = salt
	return nil
}

// updateUserFields applies the changes from an UpdateUserRequest to a user model,
// handling email uniqueness checks, password hashing, and returning whether password changed.
func updateUserFields(c *gin.Context, db *gorm.DB, user *model.User, req *UpdateUserRequest) (passwordChanged bool, err error) {
	if req.Email != "" && req.Email != user.Email {
		exists, err := emailExists(db, req.Email, user.ID)
		if err != nil {
			return false, err
		}
		if exists {
			util.CallUserError(c, util.APIErrorParams{Msg: "Email already exists", Err: fmt.Errorf("email already exists")})
			return false, fmt.Errorf("email already exists")
		}
		user.Email = req.Email
	}

	if req.Name != "" {
		user.Name = req.Name
	}

	if req.Password != "" {
		if err := hashUserPassword(c, user, req.Password); err != nil {
			return false, err
		}
		passwordChanged = true
	}

	return passwordChanged, nil
}

// invalidateUserSessions removes session records from both DB and Redis for a given user.
func invalidateUserSessions(db *gorm.DB, userID uint) {
	_ = db.Where("user_id = ?", userID).Delete(&model.Session{}).Error
	_ = util.InvalidateUserSessions(userID)
}

// UpdateUser godoc
// @Summary      Update current user profile
// @Description  Update authenticated user's name, email, and/or password
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        request body UpdateUserRequest true "Update details"
// @Success      200 {object} util.APIResponse "Update successful"
// @Failure      400 {object} util.APIResponse "Invalid request or email already exists"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /user [patch]
func UpdateUser(c *gin.Context) {
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request payload", Err: err})
		return
	}

	if !validateUpdateRequest(&req) {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "At least one field (name, email, or password) must be provided",
			Err: fmt.Errorf("no fields to update"),
		})
		return
	}

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Database connection not available", Err: fmt.Errorf("db is nil")})
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		util.CallUserNotAuthorized(c, util.APIErrorParams{Msg: "User not authenticated", Err: fmt.Errorf("user id not found in context")})
		return
	}

	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			util.CallErrorNotFound(c, util.APIErrorParams{Msg: "User not found", Err: err})
			return
		}
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve user", Err: err})
		return
	}

	passwordChanged, err := updateUserFields(c, db, &user, &req)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to update user fields", Err: err})
		return
	}

	if err := db.Save(&user).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to update user", Err: err})
		return
	}

	if passwordChanged {
		invalidateUserSessions(db, user.ID)
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "User updated successfully", Data: user})
}

// ListUsers godoc
// @Summary      List all users (admin only)
// @Description  Get a paginated list of users using cursor-based pagination. Admin-only access.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        limit query int false "Limit number of results (default 10, max 100)"
// @Param        cursor query int false "Cursor for pagination (User ID)"
// @Param        keyword query string false "Search keyword for name or email"
// @Success      200 {object} util.APIResponse{data=object} "Users retrieved with cursor pagination"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /user [get]
func ListUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	cursorStr := c.Query("cursor")
	offsetStr := c.Query("offset")
	keyword := c.Query("keyword")

	// Set default and max limits
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	// Parse and validate cursor
	var cursor uint
	if cursorStr != "" {
		cursorVal, err := strconv.ParseUint(cursorStr, 10, strconv.IntSize)
		if err != nil {
			util.CallUserError(c, util.APIErrorParams{
				Msg: "Invalid cursor parameter",
				Err: fmt.Errorf("cursor must be a valid positive integer"),
			})
			return
		}
		cursor = uint(cursorVal)
	}

	// Parse offset (optional). Negative offsets are ignored.
	var offset int
	if offsetStr != "" {
		offVal, err := strconv.Atoi(offsetStr)
		if err == nil && offVal > 0 {
			offset = offVal
		}
	}

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{
			Msg: "Database connection not available",
			Err: fmt.Errorf("db is nil"),
		})
		return
	}

	var users []model.User
	// GORM automatically excludes soft-deleted records (where deleted_at IS NOT NULL)
	// when querying models with gorm.Model. No explicit filter is needed here.
	query := db.Model(&model.User{})

	// Apply keyword filter if provided
	if keyword != "" {
		kw := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR email LIKE ?", kw, kw)
	}

	// Compute total number of matching users (without pagination)
	var total int64
	countQuery := db.Model(&model.User{})
	if keyword != "" {
		kw := "%" + keyword + "%"
		countQuery = countQuery.Where("name LIKE ? OR email LIKE ?", kw, kw)
	}
	if err := countQuery.Count(&total).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to count users", Err: err})
		return
	}

	// Apply cursor-based pagination if provided, otherwise apply offset if provided
	if cursor > 0 {
		query = query.Where("id > ?", cursor)
	} else if offset > 0 {
		query = query.Offset(offset)
	}

	// Fetch one extra record to determine if there are more pages
	if err := query.Order("id ASC").Limit(limit + 1).Find(&users).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve users", Err: err})
		return
	}

	// Determine if there are more pages
	hasMore := len(users) > limit
	if hasMore {
		users = users[:limit]
	}

	// Get the next cursor (only if there are more pages)
	var nextCursor *uint
	if hasMore {
		lastID := users[len(users)-1].ID
		nextCursor = &lastID
	}

	util.CallSuccessOK(c, util.APISuccessParams{
		Msg: "Users retrieved",
		Data: map[string]interface{}{
			"users":         users,
			"total":         total,
			"total_fetched": len(users),
			"has_more":      hasMore,
			"next_cursor":   nextCursor,
		},
	})
}

// AdminUpdateUser godoc
// @Summary      Update other user's profile (admin only)
// @Description  Admins can update another user's name, email, and password
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path int true "User ID"
// @Param        request body UpdateUserRequest true "Update details"
// @Success      200 {object} util.APIResponse "Update successful"
// @Failure      400 {object} util.APIResponse "Invalid request or email already exists"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /user/{id} [patch]
func AdminUpdateUser(c *gin.Context) {
	uid, err := parseIDParam(c)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: err.Error(), Err: err})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request payload", Err: err})
		return
	}

	if !validateUpdateRequest(&req) {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "At least one field (name, email, or password) must be provided",
			Err: fmt.Errorf("no fields to update"),
		})
		return
	}

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Database connection not available", Err: fmt.Errorf("db is nil")})
		return
	}

	var user model.User
	if err := db.First(&user, uid).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			util.CallErrorNotFound(c, util.APIErrorParams{Msg: "User not found", Err: err})
			return
		}
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve user", Err: err})
		return
	}

	_, err = updateUserFields(c, db, &user, &req)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to update user fields", Err: err})
		return
	}

	if err := db.Save(&user).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to update user", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "User updated successfully", Data: user})
}

// parseIDParam parses the "id" path parameter into a uint and returns an error if invalid.
func parseIDParam(c *gin.Context) (uint, error) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("user ID must be a valid integer")
	}
	if id <= 0 {
		return 0, fmt.Errorf("user ID must be a positive integer")
	}
	return uint(id), nil
}

// emailExists checks whether an email already exists in users table excluding a given user ID.
func emailExists(db *gorm.DB, email string, excludeID uint) (bool, error) {
	var count int64
	if err := db.Model(&model.User{}).Where("email = ? AND id != ?", email, excludeID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetUserInfo godoc
// @Summary      Get user info (admin only)
// @Description  Retrieve a user's information by ID. Admin-only access.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path int true "User ID"
// @Success      200 {object} util.APIResponse "User retrieved"
// @Failure      400 {object} util.APIResponse "Invalid user id"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      404 {object} util.APIResponse "User not found"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /user/{id} [get]
func GetUserInfo(c *gin.Context) {
	uid, err := parseIDParam(c)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: err.Error(), Err: err})
		return
	}

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Database connection not available", Err: fmt.Errorf("db is nil")})
		return
	}

	var user model.User
	if err := db.First(&user, uid).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			util.CallErrorNotFound(c, util.APIErrorParams{Msg: "User not found", Err: err})
			return
		}
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve user", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "User retrieved", Data: user})
}

// UpdateUserByID is a compatibility wrapper that calls AdminUpdateUser
func UpdateUserByID(c *gin.Context) {
	AdminUpdateUser(c)
}

// deleteUserTransaction performs the atomic delete of a user and their sessions within a transaction.
func deleteUserTransaction(tx *gorm.DB, userID uint) error {
	var user model.User
	if err := tx.First(&user, userID).Error; err != nil {
		return err
	}

	if err := tx.Where("user_id = ?", userID).Delete(&model.Session{}).Error; err != nil {
		return err
	}

	return tx.Delete(&user).Error
}

// DeleteUser godoc
// @Summary      Delete user (admin only)
// @Description  Soft-delete a user by ID. Admin-only access.
// @Tags         Authentication
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path int true "User ID"
// @Success      200 {object} util.APIResponse "User deleted"
// @Failure      400 {object} util.APIResponse "Invalid user id"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      404 {object} util.APIResponse "User not found"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /user/{id} [delete]
func DeleteUser(c *gin.Context) {
	uid, err := parseIDParam(c)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: err.Error(), Err: err})
		return
	}

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Database connection not available", Err: fmt.Errorf("db is nil")})
		return
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return deleteUserTransaction(tx, uid)
	}); err != nil {
		if err == gorm.ErrRecordNotFound {
			util.CallErrorNotFound(c, util.APIErrorParams{Msg: "User not found", Err: err})
			return
		}
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to delete user", Err: err})
		return
	}

	_ = util.InvalidateUserSessions(uid)
	util.CallSuccessOK(c, util.APISuccessParams{Msg: "User deleted"})
}
