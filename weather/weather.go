package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Conditions holds current weather data
type Conditions struct {
	City          string
	TempC         float64
	FeelsLikeC    float64
	Humidity      int
	WindSpeedKmh  float64
	WindDirection int
	Description   string
	Icon          string // emoji
	Visibility    float64
	UVIndex       float64
	IsDay         bool
	UpdatedAt     time.Time
}

// DayForecast holds a single day's forecast
type DayForecast struct {
	Date     time.Time
	MaxTempC float64
	MinTempC float64
	RainMM   float64
	Icon     string
	Desc     string
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// Fetch retrieves current weather and 5-day forecast using Open-Meteo
func Fetch(ctx context.Context, lat, lon float64, city string) (*Conditions, []DayForecast, error) {
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f"+
			"&current=temperature_2m,relative_humidity_2m,apparent_temperature,is_day,"+
			"weather_code,wind_speed_10m,wind_direction_10m,uv_index,visibility"+
			"&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_sum"+
			"&timezone=auto&forecast_days=10",
		lat, lon,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("open-meteo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("open-meteo HTTP %d", resp.StatusCode)
	}

	var raw struct {
		Current struct {
			Temperature2m       float64 `json:"temperature_2m"`
			RelativeHumidity2m  int     `json:"relative_humidity_2m"`
			ApparentTemperature float64 `json:"apparent_temperature"`
			IsDay               int     `json:"is_day"`
			WeatherCode         int     `json:"weather_code"`
			WindSpeed10m        float64 `json:"wind_speed_10m"`
			WindDirection10m    int     `json:"wind_direction_10m"`
			UVIndex             float64 `json:"uv_index"`
			Visibility          float64 `json:"visibility"`
		} `json:"current"`
		Daily struct {
			Time             []string  `json:"time"`
			WeatherCode      []int     `json:"weather_code"`
			Temperature2mMax []float64 `json:"temperature_2m_max"`
			Temperature2mMin []float64 `json:"temperature_2m_min"`
			PrecipitationSum []float64 `json:"precipitation_sum"`
		} `json:"daily"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("decoding weather: %w", err)
	}

	c := raw.Current
	icon, desc := wmoCodeToEmoji(c.WeatherCode, c.IsDay == 1)

	conditions := &Conditions{
		City:          city,
		TempC:         c.Temperature2m,
		FeelsLikeC:    c.ApparentTemperature,
		Humidity:      c.RelativeHumidity2m,
		WindSpeedKmh:  c.WindSpeed10m,
		WindDirection: c.WindDirection10m,
		Description:   desc,
		Icon:          icon,
		Visibility:    c.Visibility,
		UVIndex:       c.UVIndex,
		IsDay:         c.IsDay == 1,
		UpdatedAt:     time.Now(),
	}

	var forecasts []DayForecast
	for i, dateStr := range raw.Daily.Time {
		if i >= len(raw.Daily.WeatherCode) {
			break
		}
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		ico, dsc := wmoCodeToEmoji(raw.Daily.WeatherCode[i], true)
		rain := 0.0
		if i < len(raw.Daily.PrecipitationSum) {
			rain = raw.Daily.PrecipitationSum[i]
		}
		forecasts = append(forecasts, DayForecast{
			Date:     t,
			MaxTempC: raw.Daily.Temperature2mMax[i],
			MinTempC: raw.Daily.Temperature2mMin[i],
			RainMM:   rain,
			Icon:     ico,
			Desc:     dsc,
		})
	}

	return conditions, forecasts, nil
}

// WindDirectionStr converts degrees to compass direction
func WindDirectionStr(deg int) string {
	dirs := []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
	idx := ((deg + 22) % 360) / 45
	if idx >= len(dirs) {
		return "N"
	}
	return dirs[idx]
}

// wmoCodeToEmoji maps WMO weather codes to emoji + description
func wmoCodeToEmoji(code int, isDay bool) (string, string) {
	switch {
	case code == 0:
		if isDay {
			return "☀️", "Clear sky"
		}
		return "🌙", "Clear night"
	case code == 1:
		return "🌤️", "Mainly clear"
	case code == 2:
		return "⛅", "Partly cloudy"
	case code == 3:
		return "☁️", "Overcast"
	case code >= 45 && code <= 48:
		return "🌫️", "Fog"
	case code >= 51 && code <= 57:
		return "🌦️", "Drizzle"
	case code >= 61 && code <= 67:
		return "🌧️", "Rain"
	case code >= 71 && code <= 77:
		return "❄️", "Snow"
	case code >= 80 && code <= 82:
		return "🌦️", "Rain showers"
	case code == 95:
		return "⛈️", "Thunderstorm"
	case code >= 96 && code <= 99:
		return "⛈️", "Thunderstorm with hail"
	default:
		return "🌡️", "Unknown"
	}
}
