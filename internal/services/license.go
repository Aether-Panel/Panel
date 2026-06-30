package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const LicenseAPIBaseURL = "https://prueba.skyhostingcloud.com/api/public/licenses"

type LicenseService struct {
	httpClient *http.Client
}

// Respuesta de la API de verificación de licencia (GET)
type LicenseVerifyResponse struct {
	Valid           bool   `json:"valid"`
	Status          string `json:"status"`
	IsInGracePeriod bool   `json:"isInGracePeriod"`
	License         struct {
		Key           string `json:"key"`
		Plan          string `json:"plan"`
		MaxServers    int    `json:"maxServers"`
		UsedServers   int    `json:"usedServers"`
		ExpiryDate    string `json:"expiryDate"`
		DaysRemaining int    `json:"daysRemaining"`
		BillingCycle  string `json:"billingCycle"`
		BoundServerId string `json:"boundServerId"`
		BoundServerIp string `json:"boundServerIp"`
	} `json:"license"`
	User struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
	Payment struct {
		HasCompletedPayment bool   `json:"hasCompletedPayment"`
		LastPaymentDate     string `json:"lastPaymentDate"`
	} `json:"payment"`
	Validation struct {
		Timestamp string `json:"timestamp"`
		IP        string `json:"ip"`
	} `json:"validation"`
}

// Request para vincular servidor (POST)
type LicenseBindRequest struct {
	LicenseKey string `json:"licenseKey"`
	ServerId   string `json:"serverId"`
	ServerIp   string `json:"serverIp"`
}

// Respuesta del bind de licencia (POST)
type LicenseBindResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// Estructura para permisos extraídos
type LicensePermissions struct {
	HasPlugins bool `json:"hasPlugins"`
}

func NewLicenseService() *LicenseService {
	return &LicenseService{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// VerifyLicense verifica una licencia usando GET /api/public/licenses/verify
func (ls *LicenseService) VerifyLicense(licenseKey string) (*LicenseVerifyResponse, error) {
	url := fmt.Sprintf("%s/verify?licenseKey=%s", LicenseAPIBaseURL, licenseKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := ls.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("license verification failed with status %d: %s", resp.StatusCode, string(body))
	}

	var licenseResp LicenseVerifyResponse
	if err := json.Unmarshal(body, &licenseResp); err != nil {
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	return &licenseResp, nil
}

// BindLicense vincula una licencia con un servidor usando POST /api/public/licenses/verify
func (ls *LicenseService) BindLicense(licenseKey, serverId, serverIp string) (*LicenseBindResponse, error) {
	url := fmt.Sprintf("%s/verify", LicenseAPIBaseURL)

	reqBody := LicenseBindRequest{
		LicenseKey: licenseKey,
		ServerId:   serverId,
		ServerIp:   serverIp,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := ls.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("license binding failed with status %d: %s", resp.StatusCode, string(body))
	}

	var bindResp LicenseBindResponse
	if err := json.Unmarshal(body, &bindResp); err != nil {
		// Si la respuesta no es JSON, asumimos éxito si el status es OK
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			return &LicenseBindResponse{Success: true}, nil
		}
		return nil, fmt.Errorf("error unmarshaling response: %w", err)
	}

	return &bindResp, nil
}

// ExtractPermissions extrae los permisos de la respuesta de verificación
// Por ahora solo extraemos el permiso de plugins, los demás se ignoran
func (ls *LicenseService) ExtractPermissions(verifyResp *LicenseVerifyResponse) *LicensePermissions {
	// Aquí puedes agregar lógica para extraer permisos de la respuesta
	// Por ahora, solo retornamos si tiene plugins basado en el plan
	hasPlugins := verifyResp.License.Plan == "professional" || verifyResp.License.Plan == "enterprise"

	return &LicensePermissions{
		HasPlugins: hasPlugins,
	}
}

// GetLicenseType determina el tipo de licencia (free/pro/enterprise) basado en la respuesta
func (ls *LicenseService) GetLicenseType(verifyResp *LicenseVerifyResponse) string {
	if !verifyResp.Valid {
		return "free"
	}

	plan := verifyResp.License.Plan
	switch plan {
	case "personal":
		return "free"
	case "professional":
		return "pro"
	case "enterprise":
		return "enterprise"
	default:
		// Por defecto, si es válida pero no reconocemos el plan, asumimos pro
		return "pro"
	}
}

var licenseServiceInstance *LicenseService

func GetLicenseService() *LicenseService {
	if licenseServiceInstance == nil {
		licenseServiceInstance = NewLicenseService()
	}
	return licenseServiceInstance
}
