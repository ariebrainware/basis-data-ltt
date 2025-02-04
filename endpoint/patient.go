package endpoint

import (
	"log"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"github.com/gin-gonic/gin"
)

func CreatePatient(c *gin.Context) {
	patient := &model.Patient{}

	if err := c.ShouldBindJSON(&patient); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	db, err := config.ConnectMySQL()
	if err != nil {
		log.Fatalf("Error connecting to MySQL: %v", err)
	}
	// Assume db is a *gorm.DB instance available in your package.
	if err := db.Create(&patient).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, patient)
}
