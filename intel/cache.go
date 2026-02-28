package intel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// cachedBrief is the on-disk representation — identical to Brief but
// with JSON tags so the time.Time round-trips correctly.
type cachedBrief struct {
	Summary      string        `json:"summary"`
	KeyThreats   []string      `json:"key_threats"`
	CountryRisks []CountryRisk `json:"country_risks"`
	GeneratedAt  time.Time     `json:"generated_at"`
	Model        string        `json:"model"`
}

func cacheFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cache", "watchtower")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "brief.json"), nil
}

// LoadCachedBrief reads the cached brief from disk.
// Returns nil (no error) if the file doesn't exist or is older than maxAge.
func LoadCachedBrief(maxAge time.Duration) (*Brief, error) {
	path, err := cacheFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var cb cachedBrief
	if err := json.Unmarshal(data, &cb); err != nil {
		// Corrupted cache — treat as missing
		return nil, nil
	}

	// Reject if too old
	if maxAge > 0 && time.Since(cb.GeneratedAt) > maxAge {
		return nil, nil
	}

	return &Brief{
		Summary:      cb.Summary,
		KeyThreats:   cb.KeyThreats,
		CountryRisks: cb.CountryRisks,
		GeneratedAt:  cb.GeneratedAt,
		Model:        cb.Model,
	}, nil
}

// SaveCachedBrief writes the brief to disk, silently ignoring errors
// (a cache write failure should never crash the app).
func SaveCachedBrief(b *Brief) {
	if b == nil {
		return
	}
	path, err := cacheFilePath()
	if err != nil {
		return
	}
	cb := cachedBrief{
		Summary:      b.Summary,
		KeyThreats:   b.KeyThreats,
		CountryRisks: b.CountryRisks,
		GeneratedAt:  b.GeneratedAt,
		Model:        b.Model,
	}
	data, err := json.MarshalIndent(cb, "", "  ")
	if err != nil {
		return
	}
	// Write atomically via a temp file then rename
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// ClearBriefCache deletes the cached brief file.
func ClearBriefCache() error {
	path, err := cacheFilePath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ─── Local Brief Caching ─────────────────────────────────────────────────────

// cachedLocalBrief is the on-disk representation for local brief
type cachedLocalBrief struct {
	Summary     string    `json:"summary"`
	GeneratedAt time.Time `json:"generated_at"`
	Model       string    `json:"model"`
}

func localCacheFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".cache", "watchtower")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "local_brief.json"), nil
}

// LoadCachedLocalBrief reads the cached local brief from disk.
func LoadCachedLocalBrief(maxAge time.Duration) (*LocalBrief, error) {
	path, err := localCacheFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var cb cachedLocalBrief
	if err := json.Unmarshal(data, &cb); err != nil {
		return nil, nil
	}

	if maxAge > 0 && time.Since(cb.GeneratedAt) > maxAge {
		return nil, nil
	}

	return &LocalBrief{
		Summary:     cb.Summary,
		GeneratedAt: cb.GeneratedAt,
		Model:       cb.Model,
	}, nil
}

// SaveCachedLocalBrief writes the local brief to disk
func SaveCachedLocalBrief(b *LocalBrief) {
	if b == nil {
		return
	}
	path, err := localCacheFilePath()
	if err != nil {
		return
	}
	cb := cachedLocalBrief{
		Summary:     b.Summary,
		GeneratedAt: b.GeneratedAt,
		Model:       b.Model,
	}
	data, err := json.MarshalIndent(cb, "", "  ")
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// ClearLocalBriefCache deletes the cached local brief file.
func ClearLocalBriefCache() error {
	path, err := localCacheFilePath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
