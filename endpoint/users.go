package endpoint

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Sentinel errors for user update operations
var (
	ErrUserEmailAlreadyExists = errors.New("email already exists")
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

// validateAndUpdateEmail checks email uniqueness and updates the user model if valid.
// Returns an error without sending HTTP responses, letting the caller handle the response.
func validateAndUpdateEmail(db *gorm.DB, user *model.User, newEmail string) error {
	if newEmail == "" || newEmail == user.Email {
		return nil
	}
	exists, err := emailExists(db, newEmail, user.ID)
	if err != nil {
		return fmt.Errorf("failed to validate email uniqueness: %w", err)
	}
	if exists {
		return ErrUserEmailAlreadyExists
	}
	user.Email = newEmail
	return nil
}

// hashUserPassword generates a salt and hashes the provided password, updating the user model.
// Returns an error without sending HTTP responses, letting the caller handle the response.
func hashUserPassword(user *model.User, plainPassword string) error {
	salt, err := util.GenerateSalt()
	if err != nil {
		return fmt.Errorf("failed to generate password salt: %w", err)
	}

	hashedPassword, err := util.HashPasswordArgon2(plainPassword, salt)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user.Password = hashedPassword
	user.PasswordSalt = salt
	return nil
}

// updateUserFields applies the changes from an UpdateUserRequest to a user model,
// handling email uniqueness checks, password hashing, and returning whether password changed.
// Returns an error without sending HTTP responses, letting the caller handle the response.
func updateUserFields(db *gorm.DB, user *model.User, req *UpdateUserRequest) (passwordChanged bool, err error) {
	if err := validateAndUpdateEmail(db, user, req.Email); err != nil {
		return false, err
	}

	if req.Name != "" {
		user.Name = req.Name
	}

	if req.Password != "" {
		if err := hashUserPassword(user, req.Password); err != nil {
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

// performUserUpdate updates a user and returns success, handling all error cases and session invalidation.
func performUserUpdate(c *gin.Context, db *gorm.DB, user *model.User, req *UpdateUserRequest) bool {
	passwordChanged, err := updateUserFields(db, user, req)
	if err != nil {
		// Check if it's a user error (email exists) or server error
		if errors.Is(err, ErrUserEmailAlreadyExists) {
			util.CallUserError(c, util.APIErrorParams{Msg: "Email already exists", Err: err})
		} else {
			util.CallServerError(c, util.APIErrorParams{Msg: "Failed to update user fields", Err: err})
		}
		return false
	}

	if err := db.Save(user).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to update user", Err: err})
		return false
	}

	if passwordChanged {
		invalidateUserSessions(db, user.ID)
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "User updated successfully", Data: user})
	return true
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

	performUserUpdate(c, db, &user, &req)
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
	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Database connection not available", Err: fmt.Errorf("db is nil")})
		return
	}

	limit, cursor, offset := parsePaginationParams(c)
	keyword := c.Query("keyword")

	// Apply filters
	query := db.Model(&model.User{})
	filterClause, filterArgs := buildKeywordFilter(keyword)
	if filterClause != "" {
		query = query.Where(filterClause, filterArgs...)
	}

	// Count total matching users
	var total int64
	if err := query.Count(&total).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to count users", Err: err})
		return
	}

	// Apply pagination and fetch users (one extra to detect if more pages exist)
	query = applyPaginationQuery(query, cursor, offset)
	var users []model.User
	if err := query.Order("id ASC").Limit(limit + 1).Find(&users).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve users", Err: err})
		return
	}

	// Determine if there are more pages and trim to limit
	hasMore := len(users) > limit
	if hasMore {
		users = users[:limit]
	}

	// Get next cursor only if there are more pages
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
	req, ok := bindUpdateUserRequest(c)
	if !ok {
		return
	}
	if !requireUpdateFields(c, req) {
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

	performUserUpdate(c, db, &user, &req)
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

// parsePaginationParams extracts and validates limit, cursor, and offset query parameters.
func parsePaginationParams(c *gin.Context) (limit int, cursor uint, offset int) {
	// Use small helpers to keep parsing logic clear and testable
	limit = parsePositiveInt(c.Query("limit"), 10, 100)
	cursor = parseUintQuery(c, "cursor")
	offset = parsePositiveInt(c.Query("offset"), 0, 0)
	return limit, cursor, offset
}

// parsePositiveInt parses a positive integer from a query value returning a default
// when the value is missing or invalid. If max > 0 it caps the returned value.
func parsePositiveInt(q string, defaultVal, max int) int {
	if q == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(q)
	if err != nil || v <= 0 {
		return defaultVal
	}
	if max > 0 && v > max {
		return max
	}
	return v
}

// parseUintQuery parses an unsigned integer query parameter and returns 0 on error.
// A zero value is treated as invalid/missing since cursor-based pagination requires positive IDs.
func parseUintQuery(c *gin.Context, name string) uint {
	s := c.Query(name)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil || v == 0 {
		return 0
	}
	return uint(v)
}

// fetchUserByID retrieves a user by ID, returning appropriate error responses for not found or DB errors.
func fetchUserByID(c *gin.Context, db *gorm.DB, userID uint) (*model.User, bool) {
	var user model.User
	if err := db.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			util.CallErrorNotFound(c, util.APIErrorParams{Msg: "User not found", Err: err})
			return nil, false
		}
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve user", Err: err})
		return nil, false
	}
	return &user, true
}

// applyPaginationQuery applies cursor or offset-based pagination to a query.
func applyPaginationQuery(query *gorm.DB, cursor uint, offset int) *gorm.DB {
	if cursor > 0 {
		return query.Where("id > ?", cursor)
	}
	if offset > 0 {
		return query.Offset(offset)
	}
	return query
}

// buildKeywordFilter returns the keyword filter string for search queries.
func buildKeywordFilter(keyword string) (string, []interface{}) {
	if keyword != "" {
		kw := "%" + keyword + "%"
		return "name LIKE ? OR email LIKE ?", []interface{}{kw, kw}
	}
	return "", nil
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

	user, ok := fetchUserByID(c, db, uid)
	if !ok {
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "User retrieved", Data: user})
}

// UpdateUserByID is a compatibility wrapper that calls AdminUpdateUser
func UpdateUserByID(c *gin.Context) {
	AdminUpdateUser(c)
}

// deleteUserWithSessions deletes a user and all their sessions atomically.
func deleteUserWithSessions(db *gorm.DB, userID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		user := &model.User{}
		if err := tx.First(user, userID).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&model.Session{}).Error; err != nil {
			return err
		}
		return tx.Delete(user).Error
	})
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

	if err := deleteUserWithSessions(db, uid); err != nil {
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

func bindUpdateUserRequest(c *gin.Context) (UpdateUserRequest, bool) {
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request payload", Err: err})
		return UpdateUserRequest{}, false
	}
	return req, true
}

func requireUpdateFields(c *gin.Context, req UpdateUserRequest) bool {
	if validateUpdateRequest(&req) {
		return true
	}
	util.CallUserError(c, util.APIErrorParams{
		Msg: "At least one field (name, email, or password) must be provided",
		Err: fmt.Errorf("no fields to update"),
	})
	return false
}

// Helper functions for listing and pagination exist earlier in the file
