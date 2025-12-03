package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
)

type AuthData struct {
	JWT       string            `json:"jwt"`
	Cookies   map[string]string `json:"cookies"`
	Timestamp int64             `json:"timestamp"`
}

type Extractor struct {
	// supplied from caller (JWTService)
	browser playwright.Browser
	context playwright.BrowserContext
	page    playwright.Page

	baseURL    string
	username   string
	password   string
	totpSecret string
	totpToken  string

	authData *AuthData
	debug    bool

	mu sync.Mutex
}

///////////////////////////////////////////////////////////////////////
/// Constructor (NO browser/context inside Extractor)
///////////////////////////////////////////////////////////////////////

func NewExtractor(
	browser playwright.Browser,
	context playwright.BrowserContext,
	page playwright.Page,
	baseURL string,
	username string,
	password string,
	totpSecret string,
	totpToken string,
) *Extractor {

	return &Extractor{
		browser:    browser,
		context:    context,
		page:       page,
		baseURL:    baseURL,
		username:   username,
		password:   password,
		totpSecret: totpSecret,
		totpToken:  totpToken,
		authData: &AuthData{
			Cookies: make(map[string]string),
		},
		debug: true,
	}
}

///////////////////////////////////////////////////////////////////////
/// Extract performs the full authentication flow
///////////////////////////////////////////////////////////////////////

func (e *Extractor) Extract() error {
	if e.debug {
		log.Println("Starting authentication extraction (no internal browser launch)...")
	}

	// Setup interception and listeners
	e.setupRequestInterception(e.context)
	e.setupResponseListening(e.page)

	// Run login flow
	if err := e.executeLoginFlow(e.page); err != nil {
		return err
	}

	// Extract cookies
	if err := e.extractCookies(e.context); err != nil {
		return err
	}

	time.Sleep(2 * time.Second)

	if e.authData.JWT == "" {
		log.Println("⚠ No JWT captured during login flow")
	}

	log.Println("Authentication extraction completed")
	return nil
}

///////////////////////////////////////////////////////////////////////
/// Request Interception
///////////////////////////////////////////////////////////////////////

func (e *Extractor) setupRequestInterception(context playwright.BrowserContext) {
	context.Route("**/*", func(route playwright.Route) {
		request := route.Request()
		url := request.URL()

		if strings.Contains(url, "api.atermes.nl") || strings.Contains(url, "/api/") {
			headers := request.Headers()

			// both lowercase and uppercase
			if auth, ok := headers["authorization"]; ok {
				e.tryExtractJWT(fmt.Sprintf("%v", auth))
			}
			if auth, ok := headers["Authorization"]; ok {
				e.tryExtractJWT(fmt.Sprintf("%v", auth))
			}
		}

		route.Continue()
	})
}

func (e *Extractor) tryExtractJWT(authHeader string) {
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if len(token) > 50 {
		e.mu.Lock()
		e.authData.JWT = token
		e.mu.Unlock()

		if e.debug {
			log.Println("🔥 JWT captured from Authorization header")
		}
	}
}

///////////////////////////////////////////////////////////////////////
/// Response listener
///////////////////////////////////////////////////////////////////////

func (e *Extractor) setupResponseListening(page playwright.Page) {
	page.On("response", func(response playwright.Response) {
		go func(resp playwright.Response) {
			url := resp.URL()

			if !strings.Contains(url, "/api/") &&
				!strings.Contains(url, "/token") &&
				!strings.Contains(url, "/auth") {
				return
			}

			body, err := resp.Body()
			if err != nil || len(body) == 0 {
				return
			}

			bodyStr := string(body)
			e.extractTokenFromJSON(bodyStr)

		}(response)
	})
}

// /////////////////////////////////////////////////////////////////////
// / Login flow
// /////////////////////////////////////////////////////////////////////
func (e *Extractor) executeLoginFlow(page playwright.Page) error {
	log.Println("Step 1: Goto baseURL...")

	_, err := page.Goto(e.baseURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(30000),
	})
	if err != nil {
		return fmt.Errorf("failed to goto base URL: %w", err)
	}

	time.Sleep(1 * time.Second)

	url := page.URL()
	if !strings.Contains(url, "b2clogin.com") && !strings.Contains(url, "/Account/Login") {
		log.Println("Already logged in")
		return nil
	}

	log.Println("Step 2: Login page...")

	if !strings.Contains(url, "b2clogin.com") {
		_, err := page.Goto(e.baseURL + "/Account/Login")
		if err != nil {
			return fmt.Errorf("goto login failed: %w", err)
		}
	}

	time.Sleep(1 * time.Second)

	log.Println("Filling email...")
	if err := e.fillEmail(page); err != nil {
		return err
	}

	log.Println("Filling password...")
	if err := e.fillPassword(page); err != nil {
		return err
	}

	// **FIX: Druk Enter in het password veld**
	log.Println("Submitting form with Enter key...")
	if err := e.submitLoginForm(page); err != nil {
		return err
	}

	// Wacht langer op 2FA pagina
	time.Sleep(5 * time.Second)

	log.Println("Handling 2FA...")
	if err := e.handle2FA(page); err != nil {
		return err
	}

	log.Println("Waiting for redirect back to Atermes...")

	for i := 0; i < 30; i++ {
		if strings.Contains(page.URL(), e.baseURL) &&
			!strings.Contains(page.URL(), "b2clogin.com") {
			break
		}
		time.Sleep(1 * time.Second)
	}

	return nil
}

// **Nieuwe functie: Submit form met Enter**
func (e *Extractor) submitLoginForm(page playwright.Page) error {
	// Probeer Enter te drukken in password veld
	passwordSelectors := []string{
		"input[type='password']",
		"input#password",
	}

	for _, sel := range passwordSelectors {
		loc := page.Locator(sel)
		if count, _ := loc.Count(); count > 0 {
			// Druk Enter
			err := loc.Press("Enter")
			if err == nil {
				return nil
			}
		}
	}

	// Fallback: probeer de button te klikken
	return e.clickLoginButton(page)
}

// Bestaande functie blijft als fallback
func (e *Extractor) clickLoginButton(page playwright.Page) error {
	selectors := []string{
		"button#next",
		"button[type='submit']",
		"button:has-text('Aanmelden')",
		"button:has-text('Sign in')",
		"button:has-text('Login')",
		"input[type='submit']",
	}

	for _, sel := range selectors {
		loc := page.Locator(sel)
		if count, _ := loc.Count(); count > 0 {
			return loc.Click()
		}
	}
	return fmt.Errorf("login button not found")
}

///////////////////////////////////////////////////////////////////////
/// Form helpers
///////////////////////////////////////////////////////////////////////

func (e *Extractor) fillEmail(page playwright.Page) error {
	selectors := []string{
		"input[type='email']",
		"input#email",
		"input#signInName",
	}

	for _, sel := range selectors {
		loc := page.Locator(sel)
		if count, _ := loc.Count(); count > 0 {
			return loc.Fill(e.username)
		}
	}
	return fmt.Errorf("email input not found")
}

func (e *Extractor) fillPassword(page playwright.Page) error {
	selectors := []string{
		"input[type='password']",
		"input#password",
	}

	for _, sel := range selectors {
		loc := page.Locator(sel)
		if count, _ := loc.Count(); count > 0 {
			return loc.Fill(e.password)
		}
	}
	return fmt.Errorf("password input not found")
}

func (e *Extractor) handle2FA(page playwright.Page) error {
	selectors := []string{
		"input#otpCode",
		"input[name='code']",
		"input#totpCode",
	}
	var locator playwright.Locator

	for i := 0; i < 20; i++ {
		for _, sel := range selectors {
			loc := page.Locator(sel)
			if count, _ := loc.Count(); count > 0 {
				locator = loc
				goto FOUND
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

FOUND:

	if locator == nil {
		return fmt.Errorf("2FA input field not found after waiting 10 seconds")
	}

	totpCode := e.totpToken

	if err := locator.Fill(totpCode); err != nil {
		return fmt.Errorf("failed to fill 2FA code: %w", err)
	}

	// Druk Enter om het formulier te submitten
	time.Sleep(500 * time.Millisecond)
	if err := locator.Press("Enter"); err != nil {
		log.Println("Warning: failed to press Enter on 2FA field:", err)
	}

	return nil
}

///////////////////////////////////////////////////////////////////////
/// Cookie + JSON token parsing
///////////////////////////////////////////////////////////////////////

func (e *Extractor) extractCookies(context playwright.BrowserContext) error {
	cookies, err := context.Cookies()
	if err != nil {
		return err
	}

	for _, c := range cookies {
		e.authData.Cookies[c.Name] = c.Value
	}
	return nil
}

func (e *Extractor) extractTokenFromJSON(str string) {
	var data map[string]interface{}
	if json.Unmarshal([]byte(str), &data) != nil {
		return
	}

	for _, key := range []string{"token", "access_token", "jwt", "id_token"} {
		if v, ok := data[key]; ok {
			if s, ok := v.(string); ok && len(s) > 20 {
				e.mu.Lock()
				e.authData.JWT = s
				e.mu.Unlock()
				return
			}
		}
	}
}

///////////////////////////////////////////////////////////////////////
/// Export
///////////////////////////////////////////////////////////////////////

func (e *Extractor) GetAuthData() *AuthData {
	e.authData.Timestamp = time.Now().Unix()
	return e.authData
}
