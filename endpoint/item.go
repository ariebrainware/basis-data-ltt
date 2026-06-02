package endpoint

import (
	"fmt"

	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type createItemRequest struct {
	Name     string `json:"name" example:"Bandage"`
	Quantity int    `json:"quantity" example:"100"`
	Price    int64  `json:"price" example:"25000"`
}

type updateItemRequest struct {
	Name     *string `json:"name" example:"Bandage"`
	Quantity *int    `json:"quantity" example:"100"`
	Price    *int64  `json:"price" example:"25000"`
}

func getItemIDParam(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if id == "" {
		util.CallUserError(c, util.APIErrorParams{Msg: "Missing item ID", Err: fmt.Errorf("item ID is required")})
		return "", false
	}
	return id, true
}

func validateCreateItemInput(c *gin.Context, req createItemRequest) bool {
	if req.Name == "" {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body: name is required", Err: fmt.Errorf("name is required")})
		return false
	}
	if req.Quantity < 0 {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body: quantity must be >= 0", Err: fmt.Errorf("quantity must be >= 0")})
		return false
	}
	if req.Price < 0 {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body: price must be >= 0", Err: fmt.Errorf("price must be >= 0")})
		return false
	}
	return true
}

func loadItemOrAbort(c *gin.Context, db *gorm.DB, id string) (model.Item, bool) {
	var item model.Item
	if err := db.Where("id = ? AND deleted_at IS NULL", id).First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			util.CallUserError(c, util.APIErrorParams{Msg: "Item not found", Err: err})
			return model.Item{}, false
		}

		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve item", Err: err})
		return model.Item{}, false
	}

	return item, true
}

func buildItemUpdates(c *gin.Context, req updateItemRequest) (map[string]interface{}, bool) {
	updates := make(map[string]interface{})

	if !addItemUpdate(c, updates, itemUpdateRule[string]{
		key:        "name",
		msg:        "Invalid request body: name must not be empty",
		invalidErr: fmt.Errorf("invalid name"),
		valid: func(name string) bool {
			return name != ""
		},
	}, req.Name) {
		return nil, false
	}

	if !addItemUpdate(c, updates, itemUpdateRule[int]{
		key:        "quantity",
		msg:        "Invalid request body: quantity must be >= 0",
		invalidErr: fmt.Errorf("invalid quantity"),
		valid: func(quantity int) bool {
			return quantity >= 0
		},
	}, req.Quantity) {
		return nil, false
	}

	if !addItemUpdate(c, updates, itemUpdateRule[int64]{
		key:        "price",
		msg:        "Invalid request body: price must be >= 0",
		invalidErr: fmt.Errorf("invalid price"),
		valid: func(price int64) bool {
			return price >= 0
		},
	}, req.Price) {
		return nil, false
	}

	if len(updates) == 0 {
		util.CallUserError(c, util.APIErrorParams{Msg: "No fields to update", Err: fmt.Errorf("empty update payload")})
		return nil, false
	}

	return updates, true
}

type itemUpdateRule[T any] struct {
	key        string
	msg        string
	invalidErr error
	valid      func(T) bool
}

func addItemUpdate[T any](c *gin.Context, updates map[string]interface{}, rule itemUpdateRule[T], value *T) bool {
	if value == nil {
		return true
	}

	if rule.valid != nil && !rule.valid(*value) {
		util.CallUserError(c, util.APIErrorParams{Msg: rule.msg, Err: rule.invalidErr})
		return false
	}

	updates[rule.key] = *value
	return true
}

// ListItems godoc
// @Summary      List all items
// @Description  Get a paginated list of items
// @Tags         Item
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        limit query int false "Limit number of results" default(100)
// @Param        offset query int false "Offset for pagination" default(0)
// @Success      200 {object} util.APIResponse{data=[]model.Item} "Items retrieved"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /item [get]
func ListItems(c *gin.Context) {
	limit := parsePositiveInt(c.Query("limit"), 100, 100)
	offset := parsePositiveInt(c.Query("offset"), 0, 0)

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	var items []model.Item
	if err := db.Where("deleted_at IS NULL").Order("id DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to retrieve items", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Items retrieved", Data: items})
}

// GetItemInfo godoc
// @Summary      Get item information
// @Description  Retrieve an item record by ID
// @Tags         Item
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Item ID"
// @Success      200 {object} util.APIResponse{data=model.Item} "Item retrieved"
// @Failure      400 {object} util.APIResponse "Invalid ID or item not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /item/{id} [get]
func GetItemInfo(c *gin.Context) {
	id, ok := getItemIDParam(c)
	if !ok {
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	item, ok := loadItemOrAbort(c, db, id)
	if !ok {
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Item retrieved", Data: item})
}

// CreateItem godoc
// @Summary      Create a new item
// @Description  Add a new item record
// @Tags         Item
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        request body createItemRequest true "Item information"
// @Success      200 {object} util.APIResponse{data=model.Item} "Item created"
// @Failure      400 {object} util.APIResponse "Invalid request"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /item [post]
func CreateItem(c *gin.Context) {
	var req createItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body", Err: err})
		return
	}

	if !validateCreateItemInput(c, req) {
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	item := model.Item{Name: req.Name, Quantity: req.Quantity, Price: req.Price}
	if err := db.Create(&item).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to create item", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Item created", Data: item})
}

// UpdateItem godoc
// @Summary      Update item information
// @Description  Update an existing item record
// @Tags         Item
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Item ID"
// @Param        request body updateItemRequest true "Updated item information"
// @Success      200 {object} util.APIResponse{data=model.Item} "Item updated"
// @Failure      400 {object} util.APIResponse "Invalid request or item not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /item/{id} [patch]
func UpdateItem(c *gin.Context) {
	id, ok := getItemIDParam(c)
	if !ok {
		return
	}

	var req updateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.CallUserError(c, util.APIErrorParams{Msg: "Invalid request body", Err: err})
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	item, ok := loadItemOrAbort(c, db, id)
	if !ok {
		return
	}

	updates, ok := buildItemUpdates(c, req)
	if !ok {
		return
	}

	if err := db.Model(&item).Updates(updates).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to update item", Err: err})
		return
	}

	if err := db.Where("id = ? AND deleted_at IS NULL", id).First(&item).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to reload item", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Item updated", Data: item})
}

// DeleteItem godoc
// @Summary      Delete an item
// @Description  Soft delete an item by ID
// @Tags         Item
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     SessionToken
// @Param        id path string true "Item ID"
// @Success      200 {object} util.APIResponse "Item deleted"
// @Failure      400 {object} util.APIResponse "Item not found"
// @Failure      401 {object} util.APIResponse "Unauthorized"
// @Failure      500 {object} util.APIResponse "Server error"
// @Router       /item/{id} [delete]
func DeleteItem(c *gin.Context) {
	id, ok := getItemIDParam(c)
	if !ok {
		return
	}

	db, ok := getDBOrAbort(c)
	if !ok {
		return
	}

	item, ok := loadItemOrAbort(c, db, id)
	if !ok {
		return
	}

	if err := db.Delete(&item).Error; err != nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Failed to delete item", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "Item deleted", Data: nil})
}
