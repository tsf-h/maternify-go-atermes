package service

import (
	"fmt"
	"go-atermes/auth"
	"go-atermes/entity"
	"log"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
)

type JWTService struct {
	db *gorm.DB
}

func NewJWTService(db *gorm.DB) *JWTService {
	return &JWTService{db: db}
}

///////////////////////////////////////////////////////////////////////
/// Helpers
///////////////////////////////////////////////////////////////////////

func (s *JWTService) GetFirstTenant() (string, error) {
	var env entity.AtermesEnvironment

	err := s.db.
		Where("enabled = ?", true).
		Order("tenant ASC").
		First(&env).Error

	if err != nil {
		return "", fmt.Errorf("no enabled tenant found: %w", err)
	}

	return env.Tenant, nil
}

func (s *JWTService) getAllCredentials() ([]entity.AtermesCredentials, error) {
	var creds []entity.AtermesCredentials
	if err := s.db.Find(&creds).Error; err != nil {
		return nil, err
	}
	return creds, nil
}

func (s *JWTService) getAllEnabledEnvironments() ([]entity.AtermesEnvironment, error) {
	var envs []entity.AtermesEnvironment
	if err := s.db.Preload("Credentials").Where("enabled = ?", true).Find(&envs).Error; err != nil {
		return nil, err
	}
	return envs, nil
}

func (s *JWTService) updateCredentialsJWT(id string, jwt string) error {
	now := time.Now()

	return s.db.Model(&entity.AtermesCredentials{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"jwt":     jwt,
			"jwt_set": &now,
		}).Error
}

///////////////////////////////////////////////////////////////////////
/// Extract ONE credential (each with its own browser)
///////////////////////////////////////////////////////////////////////

func (s *JWTService) extractOneCredential(c *entity.AtermesCredentials, baseURL string) error {
	log.Printf("🔐 Processing %s", c.Email)

	// Start Playwright runtime (NO idle browser)
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("failed to start Playwright: %w", err)
	}
	defer pw.Stop()

	// Launch dedicated browser for this credential
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		Args: []string{
			"--no-sandbox",
			"--disable-dev-shm-usage",
			"--disable-gpu",
		},
	})
	if err != nil {
		return fmt.Errorf("could not launch browser: %w", err)
	}
	defer browser.Close()

	// Create isolated browser context
	context, err := browser.NewContext()
	if err != nil {
		return fmt.Errorf("failed creating context: %w", err)
	}
	defer context.Close()

	// Create page — required for Extractor
	page, err := context.NewPage()
	if err != nil {
		return fmt.Errorf("failed creating page: %w", err)
	}

	// Generate totp code here
	totpCode, err := totp.GenerateCode(c.TwoFactorKey, time.Now())
	if err != nil {
		return fmt.Errorf("failed to generate TOTP for %s: %w", c.Email, err)
	}

	// Build Extractor using supplied browser/context/page
	extractor := auth.NewExtractor(
		browser,
		context,
		page,
		baseURL,
		c.Email,
		c.Password,
		c.TwoFactorKey,
		totpCode,
	)

	// Run authentication flow
	if err := extractor.Extract(); err != nil {
		return fmt.Errorf("extract failed for %s: %w", c.Email, err)
	}

	authData := extractor.GetAuthData()

	if authData.JWT == "" {
		return fmt.Errorf("no JWT found for %s", c.Email)
	}

	// Save in DB
	if err := s.updateCredentialsJWT(c.ID.String(), authData.JWT); err != nil {
		return fmt.Errorf("failed storing JWT: %w", err)
	}

	log.Printf("✅ Stored JWT for %s", c.Email)
	return nil
}

///////////////////////////////////////////////////////////////////////
/// Sequential extraction for ALL credentials
///////////////////////////////////////////////////////////////////////

func (s *JWTService) ExtractJWTForAllCredentials() error {
	log.Println("🚀 Starting sequential JWT extraction...")

	envs, err := s.getAllEnabledEnvironments()
	if err != nil {
		return err
	}

	if len(envs) == 0 {
		return fmt.Errorf("no enabled environments found")
	}

	// Group environments by credentials ID to process each credential only once
	credentialMap := make(map[string]*entity.AtermesCredentials)
	credentialToTenant := make(map[string]string)

	for _, env := range envs {
		credID := env.Credentials.ID.String()
		if _, exists := credentialMap[credID]; !exists {
			credentialMap[credID] = &env.Credentials
			credentialToTenant[credID] = env.Tenant
		}
	}

	log.Printf("Found %d unique credentials across %d environments", len(credentialMap), len(envs))

	i := 0
	for credID, cred := range credentialMap {
		i++
		tenant := credentialToTenant[credID]
		baseURL := fmt.Sprintf("https://%s.atermes.nl", tenant)
		log.Printf("\n[%d/%d] Processing %s on tenant %s", i, len(credentialMap), cred.Email, tenant)

		if err := s.extractOneCredential(cred, baseURL); err != nil {
			log.Printf("❌ Failed for %s on %s: %v", cred.Email, tenant, err)
			continue
		}

		time.Sleep(1 * time.Second) // optional: prevent rate limiting
	}

	log.Println("🎉 All credentials processed")
	return nil
}
