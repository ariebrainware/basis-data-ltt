package endpoint

import (
	"fmt"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/middleware"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/ariebrainware/basis-data-ltt/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DebugDBInfo returns basic DB connection info (masked) and allows
// optional lookup of a user by `email` query parameter. This endpoint
// is intended for local debugging only.
func DebugDBInfo(c *gin.Context) {
	cfg := config.LoadConfig()

	db := middleware.GetDB(c)
	if db == nil {
		util.CallServerError(c, util.APIErrorParams{Msg: "Database connection not available", Err: fmt.Errorf("db is nil")})
		return
	}

	email := c.Query("email")
	if email == "" {
		runtimeInfo := map[string]interface{}{
			"dialect": db.Dialector.Name(),
		}

		if db.Dialector.Name() == "mysql" {
			var mysqlInfo struct {
				Database string
				User     string
				Host     string
				Port     string
			}
			if err := db.Raw("SELECT DATABASE() AS `database`, CURRENT_USER() AS user, @@hostname AS host, @@port AS port").Scan(&mysqlInfo).Error; err != nil {
				util.CallServerError(c, util.APIErrorParams{Msg: "Database query error", Err: err})
				return
			}
			runtimeInfo["details"] = mysqlInfo
		} else {
			var sqliteInfo struct {
				Version string
			}
			if err := db.Raw("SELECT sqlite_version() AS version").Scan(&sqliteInfo).Error; err != nil {
				util.CallServerError(c, util.APIErrorParams{Msg: "Database query error", Err: err})
				return
			}
			runtimeInfo["details"] = sqliteInfo
		}

		util.CallSuccessOK(c, util.APISuccessParams{Msg: "DB info", Data: map[string]interface{}{
			"config": map[string]interface{}{
				"host":   cfg.DBHost,
				"port":   cfg.DBPort,
				"dbname": cfg.DBName,
				"user":   cfg.DBUSER,
				"env":    cfg.AppEnv,
				"dsn":    config.MySQLDSN(true),
			},
			"runtime": runtimeInfo,
		}})
		return
	}

	// Use Unscoped to include soft-deleted rows for diagnosis.
	var user model.User
	if err := db.Unscoped().Select("id, email, created_at, updated_at, deleted_at").Where("email = ?", email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			util.CallErrorNotFound(c, util.APIErrorParams{Msg: "User not found", Err: err})
			return
		}
		util.CallServerError(c, util.APIErrorParams{Msg: "Database query error", Err: err})
		return
	}

	util.CallSuccessOK(c, util.APISuccessParams{Msg: "User record", Data: user})
}
