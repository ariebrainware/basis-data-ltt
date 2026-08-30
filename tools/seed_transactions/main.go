package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/ariebrainware/basis-data-ltt/config"
	"github.com/ariebrainware/basis-data-ltt/model"
	"gorm.io/gorm"
)

const defaultRemarks = "Seeded from recent treatments"

type seederStats struct {
	TreatmentsFetched int
	AlreadyHasTx      int
	Created           int
	MissingPricing    int
}

func main() {
	limit := flag.Int("limit", 100, "number of latest treatments to process")
	dryRun := flag.Bool("dry-run", false, "preview what would be created without writing to DB")
	flag.Parse()

	if *limit <= 0 {
		log.Fatalf("invalid --limit %d: must be > 0", *limit)
	}

	db, err := config.ConnectMySQL()
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	stats, err := seedTransactionsFromLatestTreatments(db, *limit, *dryRun)
	if err != nil {
		log.Fatalf("seeding failed: %v", err)
	}

	fmt.Println("seed transactions completed")
	fmt.Printf("- treatments fetched: %d\n", stats.TreatmentsFetched)
	fmt.Printf("- already had transaction: %d\n", stats.AlreadyHasTx)
	fmt.Printf("- transactions created: %d\n", stats.Created)
	fmt.Printf("- created with missing pricing (amount=0): %d\n", stats.MissingPricing)
	if *dryRun {
		fmt.Println("- dry run mode: no records were inserted")
	}
}

func seedTransactionsFromLatestTreatments(db *gorm.DB, limit int, dryRun bool) (seederStats, error) {
	var stats seederStats

	var treatments []model.Treatment
	if err := db.Order("id DESC").Limit(limit).Find(&treatments).Error; err != nil {
		return stats, fmt.Errorf("failed to fetch latest treatments: %w", err)
	}
	stats.TreatmentsFetched = len(treatments)

	if len(treatments) == 0 {
		return stats, nil
	}

	treatmentIDs := make([]uint, 0, len(treatments))
	therapistIDsMap := make(map[uint]struct{})
	for _, t := range treatments {
		treatmentIDs = append(treatmentIDs, t.ID)
		therapistIDsMap[t.TherapistID] = struct{}{}
	}

	var existing []model.Transaction
	if err := db.Select("treatment_id").Where("treatment_id IN ?", treatmentIDs).Find(&existing).Error; err != nil {
		return stats, fmt.Errorf("failed to fetch existing transactions: %w", err)
	}

	hasTx := make(map[uint]bool, len(existing))
	for _, tx := range existing {
		hasTx[tx.TreatmentID] = true
	}

	therapistIDs := make([]uint, 0, len(therapistIDsMap))
	for therapistID := range therapistIDsMap {
		therapistIDs = append(therapistIDs, therapistID)
	}

	pricingByTherapist := make(map[uint]int64)
	if len(therapistIDs) > 0 {
		var pricings []model.Pricing
		if err := db.Where("therapist_id IN ?", therapistIDs).Order("id DESC").Find(&pricings).Error; err != nil {
			return stats, fmt.Errorf("failed to fetch pricing data: %w", err)
		}
		for _, p := range pricings {
			if _, exists := pricingByTherapist[p.TherapistID]; !exists {
				pricingByTherapist[p.TherapistID] = p.Price
			}
		}
	}

	toCreate := make([]model.Transaction, 0, len(treatments))
	for _, t := range treatments {
		if hasTx[t.ID] {
			stats.AlreadyHasTx++
			continue
		}

		amount, ok := pricingByTherapist[t.TherapistID]
		if !ok {
			amount = 0
			stats.MissingPricing++
		}

		toCreate = append(toCreate, model.Transaction{
			TreatmentID:   t.ID,
			TherapistID:   t.TherapistID,
			Amount:        amount,
			Remarks:       defaultRemarks,
			PaymentMethod: "cash",
			PaymentStatus: "unpaid",
		})
	}

	if dryRun || len(toCreate) == 0 {
		stats.Created = len(toCreate)
		return stats, nil
	}

	if err := db.Create(&toCreate).Error; err != nil {
		return stats, fmt.Errorf("failed to create transactions: %w", err)
	}

	stats.Created = len(toCreate)
	return stats, nil
}
