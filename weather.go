package netatmo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/oauth2"
)

const (
	// DefaultBaseURL is Netatmo API URL
	baseURL = "https://api.netatmo.com/"
	// DefaultAuthURL is Netatmo OAuth2 token endpoint
	authURL = baseURL + "oauth2/token"
	// DefaultDeviceURL is Netatmo stations data endpoint
	deviceURL = baseURL + "api/getstationsdata"
)

// Config holds OAuth2 credentials and token state, persisted to TOML.
type Config struct {
	ClientID        string    `toml:"client_id"`
	ClientSecret    string    `toml:"client_secret"`
	AccessToken     string    `toml:"access_token"`
	RefreshToken    string    `toml:"refresh_token"`
	TokenValidUntil time.Time `toml:"token_valid_until"`

	path string     `toml:"-"`
	mu   sync.Mutex `toml:"-"`
}

// LoadConfig reads a TOML file at path into a Config.
func LoadConfig(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode TOML config %q: %w", path, err)
	}
	cfg.path = path
	return &cfg, nil
}

// saveConfig writes cfg back to its TOML file.
func saveConfig(cfg *Config) error {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()

	file, err := os.Create(cfg.path)
	if err != nil {
		return fmt.Errorf("failed to open config file for writing: %w", err)
	}
	defer file.Close()

	enc := toml.NewEncoder(file)
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("failed to encode config to TOML: %w", err)
	}
	return nil
}

// Client makes authenticated requests to the Netatmo API.
type Client struct {
	oauth      *oauth2.Config
	httpClient *http.Client
	Dc         *DeviceCollection
	cfg        *Config
}

// DeviceCollection holds the list of devices from Netatmo.
type DeviceCollection struct {
	Body struct {
		Devices []*Device `json:"devices"`
	}
}

// Device represents a station or module.
type Device struct {
	ID             string `json:"_id"`
	StationName    string `json:"station_name"`
	ModuleName     string `json:"module_name"`
	BatteryPercent *int32 `json:"battery_percent,omitempty"`
	WifiStatus     *int32 `json:"wifi_status,omitempty"`
	RFStatus       *int32 `json:"rf_status,omitempty"`
	Type           string
	DashboardData  DashboardData `json:"dashboard_data"`
	Place          Place         `json:"place"`
	LinkedModules  []*Device     `json:"modules"`
}

// DashboardData holds sensor measurements.
type DashboardData struct {
	Temperature      *float32 `json:"Temperature,omitempty"`
	MaxTemp          *float32 `json:"max_temp,omitempty"`
	MinTemp          *float32 `json:"min_temp,omitempty"`
	TempTrend        string   `json:"temp_trend,omitempty"`
	Humidity         *int32   `json:"Humidity,omitempty"`
	CO2              *int32   `json:"CO2,omitempty"`
	Noise            *int32   `json:"Noise,omitempty"`
	Pressure         *float32 `json:"Pressure,omitempty"`
	AbsolutePressure *float32 `json:"AbsolutePressure,omitempty"`
	PressureTrend    string   `json:"pressure_trend,omitempty"`
	Rain             *float32 `json:"Rain,omitempty"`
	Rain1Hour        *float32 `json:"sum_rain_1,omitempty"`
	Rain1Day         *float32 `json:"sum_rain_24,omitempty"`
	WindAngle        *int32   `json:"WindAngle,omitempty"`
	WindStrength     *int32   `json:"WindStrength,omitempty"`
	GustAngle        *int32   `json:"GustAngle,omitempty"`
	GustStrength     *int32   `json:"GustStrength,omitempty"`
	LastMeasure      *int64   `json:"time_utc"`
}

// Place holds geolocation and location details.
type Place struct {
	Altitude *int32   `json:"altitude,omitempty"`
	City     string   `json:"city,omitempty"`
	Country  string   `json:"country,omitempty"`
	Timezone string   `json:"timezone,omitempty"`
	Location Location `json:"location,omitempty"`
}

// Location holds latitude/longitude.
type Location struct {
	Longitude *float32
	Latitude  *float32
}

func (tp *Location) UnmarshalJSON(data []byte) error {
	a := []interface{}{&tp.Longitude, &tp.Latitude}
	return json.Unmarshal(data, &a)
}

// savingSource wraps the oauth2.TokenSource to save tokens on refresh.
type savingSource struct {
	src oauth2.TokenSource
	cfg *Config
}

func (s *savingSource) Token() (*oauth2.Token, error) {
	token, err := s.src.Token()
	if err != nil {
		return nil, err
	}
	s.cfg.mu.Lock()
	s.cfg.AccessToken = token.AccessToken
	s.cfg.RefreshToken = token.RefreshToken
	s.cfg.TokenValidUntil = token.Expiry
	s.cfg.mu.Unlock()

	if err := saveConfig(s.cfg); err != nil {
		return nil, fmt.Errorf("error saving config: %w", err)
	}
	return token, nil
}

// NewClient initializes the Netatmo client with automatic token persistence.
func NewClient(cfg *Config) (*Client, error) {
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     oauth2.Endpoint{TokenURL: authURL},
	}

	// Seed the token (may be expired)
	seed := &oauth2.Token{
		AccessToken:  cfg.AccessToken,
		RefreshToken: cfg.RefreshToken,
		Expiry:       cfg.TokenValidUntil,
	}

	reuse := oauth2.ReuseTokenSource(seed, oauthCfg.TokenSource(context.Background(), seed))
	saving := &savingSource{src: reuse, cfg: cfg}

	client := &Client{
		oauth:      oauthCfg,
		httpClient: oauth2.NewClient(context.Background(), saving),
		Dc:         &DeviceCollection{},
		cfg:        cfg,
	}
	return client, nil
}

// doHTTPPostForm submits a POST form.
func (c *Client) doHTTPPostForm(urlStr string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequest("POST", urlStr, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.doHTTP(req)
}

// doHTTPGet submits a GET request.
func (c *Client) doHTTPGet(urlStr string, data url.Values) (*http.Response, error) {
	if data != nil {
		urlStr += "?" + data.Encode()
	}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	return c.doHTTP(req)
}

// doHTTP executes an *http.Request using the OAuth2 client.
func (c *Client) doHTTP(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// processHTTPResponse checks status and unmarshals JSON.
func processHTTPResponse(resp *http.Response, err error, holder interface{}) error {
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad HTTP status: %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(holder)
}

// Read retrieves station/module data.
func (c *Client) Read() (*DeviceCollection, error) {
	resp, err := c.doHTTPGet(deviceURL, url.Values{"app_type": {"app_station"}})
	if err = processHTTPResponse(resp, err, c.Dc); err != nil {
		return nil, err
	}
	return c.Dc, nil
}

// Devices returns the list of devices
func (dc *DeviceCollection) Devices() []*Device {
	return dc.Body.Devices
}

// Stations is an alias of Devices
func (dc *DeviceCollection) Stations() []*Device {
	return dc.Devices()
}

// Modules returns associated device module
func (d *Device) Modules() []*Device {
	list := append([]*Device(nil), d.LinkedModules...)
	return append(list, d)
}

// Data returns timestamp and the list of sensor value for this module
func (d *Device) Data() (int64, map[string]interface{}) {

	// return only populate field of DashboardData
	m := make(map[string]interface{})

	if d.DashboardData.Temperature != nil {
		m["Temperature"] = *d.DashboardData.Temperature
	}
	if d.DashboardData.MinTemp != nil {
		m["MinTemp"] = *d.DashboardData.MinTemp
	}
	if d.DashboardData.MaxTemp != nil {
		m["MaxTemp"] = *d.DashboardData.MaxTemp
	}
	if d.DashboardData.TempTrend != "" {
		m["TempTrend"] = d.DashboardData.TempTrend
	}
	if d.DashboardData.Humidity != nil {
		m["Humidity"] = *d.DashboardData.Humidity
	}
	if d.DashboardData.CO2 != nil {
		m["CO2"] = *d.DashboardData.CO2
	}
	if d.DashboardData.Noise != nil {
		m["Noise"] = *d.DashboardData.Noise
	}
	if d.DashboardData.Pressure != nil {
		m["Pressure"] = *d.DashboardData.Pressure
	}
	if d.DashboardData.AbsolutePressure != nil {
		m["AbsolutePressure"] = *d.DashboardData.AbsolutePressure
	}
	if d.DashboardData.PressureTrend != "" {
		m["PressureTrend"] = d.DashboardData.PressureTrend
	}
	if d.DashboardData.Rain != nil {
		m["Rain"] = *d.DashboardData.Rain
	}
	if d.DashboardData.Rain1Hour != nil {
		m["Rain1Hour"] = *d.DashboardData.Rain1Hour
	}
	if d.DashboardData.Rain1Day != nil {
		m["Rain1Day"] = *d.DashboardData.Rain1Day
	}
	if d.DashboardData.WindAngle != nil {
		m["WindAngle"] = *d.DashboardData.WindAngle
	}
	if d.DashboardData.WindStrength != nil {
		m["WindStrength"] = *d.DashboardData.WindStrength
	}
	if d.DashboardData.GustAngle != nil {
		m["GustAngle"] = *d.DashboardData.GustAngle
	}
	if d.DashboardData.GustAngle != nil {
		m["GustAngle"] = *d.DashboardData.GustAngle
	}
	if d.DashboardData.GustStrength != nil {
		m["GustStrength"] = *d.DashboardData.GustStrength
	}

	return *d.DashboardData.LastMeasure, m
}

// Info returns timestamp and the list of info value for this module
func (d *Device) Info() (int64, map[string]interface{}) {

	// return only populate field of DashboardData
	m := make(map[string]interface{})

	// Return data from module level
	if d.BatteryPercent != nil {
		m["BatteryPercent"] = *d.BatteryPercent
	}
	if d.WifiStatus != nil {
		m["WifiStatus"] = *d.WifiStatus
	}
	if d.RFStatus != nil {
		m["RFStatus"] = *d.RFStatus
	}

	return *d.DashboardData.LastMeasure, m
}
