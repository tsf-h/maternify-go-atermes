package main

import (
	"go-atermes/service"
	"log"

	"github.com/robfig/cron/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	DB *gorm.DB
)

func connectDB() error {
	dsn := "host=hsdb-rw.db user=maternify password=51pK0=`x.OpB dbname=maternify_production port=5432"
	//dsn := "host=localhost user=maternify password=51pK0=`x.OpB dbname=maternify_production port=15432"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}
	DB = db
	return nil
}

func extractJWT() {
	jwtService := service.NewJWTService(DB)

	if err := jwtService.ExtractJWTForAllCredentials(); err != nil {
		log.Printf("❌ JWT extraction failed: %v", err)
		return
	}
}

func initCron() {
	c := cron.New()
	c.AddFunc("5 8,15 * * *", func() {
		log.Println("Running scheduled JWT extraction")
		extractJWT()
	})
	c.Start()
}

func main() {
	if err := connectDB(); err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	log.Println("✅ Connected to database")

	initCron()

	extractJWT()
	select {}
}
