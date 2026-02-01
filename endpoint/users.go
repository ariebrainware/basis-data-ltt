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

type listUsersParams struct {
	limit   int
	cursor  uint
	offset  int
	keyword string
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
	req, ok := bindUpdateUserRequest(c)
	if !ok {
		return
	}
	if !requireUpdateFields(c, req) {
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		util.CallUserNotAuthorized(c, util.APIErrorParams{
			Msg: "User not authenticated",
			Err: fmt.Errorf("user id not found in context"),
		})
		return
	}

	updateUserWithRequest(c, userID, req, true)
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
	params, err := parseListUsersParams(c)
	if err != nil {
		util.CallUserError(c, util.APIErrorParams{
			Msg: "Invalid cursor parameter",
			Err: err,
		})
		return
	}

	db, ok := getDBOrRespond(c)
	if !ok {
		return
	}

	query := buildUsersQuery(db, params.keyword)

	total, err := countUsers(query)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to count users", Err: err})
		return
	}

	query = applyUsersPagination(query, params.cursor, params.offset)

	users, err := fetchUsers(query, params.limit)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve users", Err: err})
		return
	}

	// Determine if there are more pages
	hasMore := len(users) > params.limit
	if hasMore {
		users = users[:params.limit]
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
	req, ok := bindUpdateUserRequest(c)
	if !ok {
		return
	}
	if !requireUpdateFields(c, req) {
		return
	}

	updateUserWithRequest(c, uid, req, false)
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

	// Use a transaction to ensure user deletion and session invalidation are atomic.
	if err := db.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.First(&user, uid).Error; err != nil {
			return err
		}

		// Explicitly delete all sessions associated with this user so that any
		// active tokens/sessions are invalidated immediately.
		if err := tx.Where("user_id = ?", uid).Delete(&model.Session{}).Error; err != nil {
			return err
		}

		if err := tx.Delete(&user).Error; err != nil {
			return err
		}

		return nil
	}); err != nil {
		if err == gorm.ErrRecordNotFound {
			util.CallErrorNotFound(c, util.APIErrorParams{Msg: "User not found", Err: err})
			return
		}
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to delete user", Err: err})
		return
	}

	// Also remove any Redis session keys for this user (best-effort)
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
	if req.Name != "" || req.Email != "" || req.Password != "" {
		return true
	}
	util.CallUserError(c, util.APIErrorParams{
		Msg: "At least one field (name, email, or password) must be provided",
		Err: fmt.Errorf("no fields to update"),
	})
	return false
}

func updateUserWithRequest(c *gin.Context, userID uint, req UpdateUserRequest, invalidateSessions bool) {
	db, ok := getDBOrRespond(c)
	if !ok {
		return
	}

	user, err := fetchUserByID(db, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			util.CallErrorNotFound(c, util.APIErrorParams{Msg: "User not found", Err: err})
			return
		}
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve user", Err: err})
		return
	}

	passwordChanged, ok := applyUserUpdates(c, db, user, req)
	if !ok {
		return
	}

	if err := db.Save(user).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to update user", Err: err})
		return
	}

	if invalidateSessions && passwordChanged {
		_ = db.Where("user_id = ?", user.ID).Delete(&model.Session{}).Error
		_ = util.InvalidateUserSessions(user.ID)
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "User updated successfully", Data: user})
}

func fetchUserByID(db *gorm.DB, uid uint) (*model.User, error) {
	var user model.User
	if err := db.First(&user, uid).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func applyUserUpdates(c *gin.Context, db *gorm.DB, user *model.User, req UpdateUserRequest) (bool, bool) {
	if req.Email != "" && req.Email != user.Email {
		exists, err := emailExists(db, req.Email, user.ID)
		if err != nil {
			util.CallServerError(c, util.APIErrorParams{Msg: "Failed to validate email uniqueness", Err: err})
			return false, false
		}
		if exists {
			util.CallUserError(c, util.APIErrorParams{Msg: "Email already exists", Err: fmt.Errorf("email already exists")})
			return false, false
		}
		user.Email = req.Email
	}

	if req.Name != "" {
		user.Name = req.Name
	}

	if req.Password == "" {
		return false, true
	}

	salt, err := util.GenerateSalt()
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to generate password salt", Err: err})
		return false, false
	}

	hashedPassword, err := util.HashPasswordArgon2(req.Password, salt)
	if err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to hash password", Err: err})
		return false, false
	}

	user.Password = hashedPassword
	user.PasswordSalt = salt
	return true, true
}

func parseListUsersParams(c *gin.Context) (listUsersParams, error) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	var cursor uint
	if cursorStr := c.Query("cursor"); cursorStr != "" {
		cursorVal, err := strconv.ParseUint(cursorStr, 10, strconv.IntSize)
		if err != nil {
			return listUsersParams{}, fmt.Errorf("cursor must be a valid positive integer")
		}
		cursor = uint(cursorVal)
	}

	var offset int
	if offsetStr := c.Query("offset"); offsetStr != "" {
		offVal, err := strconv.Atoi(offsetStr)
		if err == nil && offVal > 0 {
			offset = offVal
		}
	}

	return listUsersParams{
		limit:   limit,
		cursor:  cursor,
		offset:  offset,
		keyword: c.Query("keyword"),
	}, nil
}

func buildUsersQuery(db *gorm.DB, keyword string) *gorm.DB {
	query := db.Model(&model.User{})
	if keyword != "" {
		kw := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR email LIKE ?", kw, kw)
	}
	return query
}

func countUsers(query *gorm.DB) (int64, error) {
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func applyUsersPagination(query *gorm.DB, cursor uint, offset int) *gorm.DB {
	if cursor > 0 {
		return query.Where("id > ?", cursor)
	}
	if offset > 0 {
		return query.Offset(offset)
	}
	return query
}

func fetchUsers(query *gorm.DB, limit int) ([]model.User, error) {
	var users []model.User
	if err := query.Order("id ASC").Limit(limit + 1).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
