package data

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// noaa client for ghcn-daily
type NOAAClient struct {
	baseURL      string
	httpClient   *http.Client
	cacheDir     string
	cacheEnabled bool
	parallel     int
	rateLimiter  *RateLimiter
	minFreeSpace int64 // minimum free space in bytes
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	tokens   chan struct{}
	interval time.Duration
	stopChan chan struct{}
}

// NewRateLimiter creates a rate limiter with specified requests per second
func NewRateLimiter(requestsPerSecond float64) *RateLimiter {
	rl := &RateLimiter{
		tokens:   make(chan struct{}, 1),
		interval: time.Duration(float64(time.Second) / requestsPerSecond),
		stopChan: make(chan struct{}),
	}

	// Start token refill goroutine
	go rl.refill()

	// Initial token
	rl.tokens <- struct{}{}

	return rl
}

func (rl *RateLimiter) refill() {
	ticker := time.NewTicker(rl.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			select {
			case rl.tokens <- struct{}{}:
			default:
				// Bucket full, skip
			}
		case <-rl.stopChan:
			return
		}
	}
}

// Wait blocks until a token is available
func (rl *RateLimiter) Wait() {
	<-rl.tokens
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
	close(rl.stopChan)
}

// station represents a weather station
type Station struct {
	ID        string
	Name      string
	Country   string
	Latitude  float64
	Longitude float64
	Elevation float64
	State     string
	StartDate time.Time
	EndDate   time.Time
}

// weather record from noaa
type WeatherRecord struct {
	StationID   string
	Date        time.Time
	Element     string // tmax, tmin, prcp, etc.
	Value       float64
	QualityFlag string
}

// noaa client options
type NOAAOptions struct {
	BaseURL        string
	CacheDir       string
	CacheEnabled   bool
	Timeout        time.Duration
	Parallel       int
	RequestsPerSec float64 // Rate limit (default 1.0)
	MinFreeSpaceGB float64 // Minimum free space in GB (default 1.0)
}

// create new noaa client
func NewNOAAClient(opts NOAAOptions) *NOAAClient {
	if opts.Parallel <= 0 {
		opts.Parallel = 4
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.RequestsPerSec <= 0 {
		opts.RequestsPerSec = 1.0 // Default: 1 request per second
	}
	if opts.MinFreeSpaceGB <= 0 {
		opts.MinFreeSpaceGB = 1.0 // Default: 1 GB minimum free space
	}

	client := &NOAAClient{
		baseURL:      opts.BaseURL,
		httpClient:   &http.Client{Timeout: opts.Timeout},
		cacheDir:     opts.CacheDir,
		cacheEnabled: opts.CacheEnabled,
		parallel:     opts.Parallel,
		rateLimiter:  NewRateLimiter(opts.RequestsPerSec),
		minFreeSpace: int64(opts.MinFreeSpaceGB * 1024 * 1024 * 1024),
	}

	return client
}

// get specific station by id
func (c *NOAAClient) GetStation(id string) (Station, error) {
	stations, err := c.loadStations()
	if err != nil {
		return Station{}, err
	}

	for _, s := range stations {
		if strings.EqualFold(s.ID, id) {
			return s, nil
		}
	}

	return Station{}, fmt.Errorf("station %s not found", id)
}

// isoToFIPS maps ISO 3166-1 alpha-2 country codes to FIPS 10-4 codes used in NOAA station IDs.
// Only entries that differ between the two standards are listed here.
var isoToFIPS = map[string]string{
	"de": "gm", // Germany
	"pl": "pl", // Poland (same)
	"tw": "ch", // Taiwan (listed under China in NOAA)
	"gb": "uk", // United Kingdom
	"kr": "ks", // South Korea
	"kp": "kn", // North Korea
	"bo": "bl", // Bolivia
	"br": "br", // Brazil (same)
	"by": "bo", // Belarus
	"cz": "ez", // Czech Republic
	"eg": "eg", // Egypt (same)
	"gr": "gr", // Greece (same)
	"hr": "hr", // Croatia (same)
	"hu": "hu", // Hungary (same)
	"il": "is", // Israel
	"ir": "ir", // Iran (same)
	"jo": "jo", // Jordan (same)
	"ke": "ke", // Kenya (same)
	"la": "la", // Laos (same)
	"lb": "le", // Lebanon
	"lk": "ce", // Sri Lanka
	"ly": "ly", // Libya (same)
	"ma": "mo", // Morocco
	"mm": "bm", // Myanmar/Burma
	"np": "np", // Nepal (same)
	"nz": "nz", // New Zealand (same)
	"ph": "rp", // Philippines
	"pk": "pk", // Pakistan (same)
	"ro": "ro", // Romania (same)
	"rs": "ri", // Serbia
	"sa": "sa", // Saudi Arabia (same)
	"sk": "lo", // Slovakia
	"sy": "sy", // Syria (same)
	"th": "th", // Thailand (same)
	"tz": "tz", // Tanzania (same)
	"ua": "up", // Ukraine
	"vn": "vm", // Vietnam
	"ye": "ym", // Yemen
	"zm": "za", // Zambia
	"zw": "zi", // Zimbabwe
}

// get all stations for a country
func (c *NOAAClient) GetStationsByCountry(countryCode string) ([]Station, error) {
	stations, err := c.loadStations()
	if err != nil {
		return nil, err
	}

	// resolve ISO code to FIPS if needed
	lookup := strings.ToLower(countryCode)
	if fips, ok := isoToFIPS[lookup]; ok {
		lookup = fips
	}

	var result []Station
	for _, s := range stations {
		if strings.EqualFold(s.Country, lookup) {
			result = append(result, s)
		}
	}

	return result, nil
}

// get stations within radius (km) of lat/lon
func (c *NOAAClient) GetStationsByRadius(lat, lon, radiusKM float64) ([]Station, error) {
	stations, err := c.loadStations()
	if err != nil {
		return nil, err
	}

	var result []Station
	for _, s := range stations {
		dist := haversine(lat, lon, s.Latitude, s.Longitude)
		if dist <= radiusKM {
			result = append(result, s)
		}
	}

	return result, nil
}

// checkAvailableSpace checks if there's enough free space at the given path
func (c *NOAAClient) checkAvailableSpace(path string) error {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return fmt.Errorf("checking disk space: %w", err)
	}

	// Calculate available space
	available := int64(stat.Bavail) * int64(stat.Bsize)

	if available < c.minFreeSpace {
		return fmt.Errorf("insufficient disk space: %d MB available, %d MB required",
			available/(1024*1024), c.minFreeSpace/(1024*1024))
	}

	return nil
}

// download station data
func (c *NOAAClient) DownloadStation(ctx context.Context, station Station) ([]WeatherRecord, error) {
	cacheFile := ""
	if c.cacheEnabled {
		cacheFile = filepath.Join(c.cacheDir, fmt.Sprintf("%s.csv", station.ID))
		if data, err := c.loadFromCache(cacheFile); err == nil {
			return data, nil
		}
	}

	// Check disk space before downloading
	if c.cacheEnabled {
		if err := c.checkAvailableSpace(c.cacheDir); err != nil {
			return nil, err
		}
	}

	// Apply rate limiting
	c.rateLimiter.Wait()

	url := fmt.Sprintf("%s/%s.csv", c.baseURL, station.ID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading station data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %s", resp.Status)
	}

	// save to cache if enabled
	if c.cacheEnabled && cacheFile != "" {
		os.MkdirAll(filepath.Dir(cacheFile), 0755)
		f, err := os.Create(cacheFile)
		if err == nil {
			defer f.Close()
			io.Copy(f, resp.Body)
			// reload from cache
			return c.loadFromCache(cacheFile)
		}
	}

	return c.parseCSV(resp.Body)
}

// parse station list from noaa
func (c *NOAAClient) loadStations() ([]Station, error) {
	cacheFile := ""
	if c.cacheEnabled {
		cacheFile = filepath.Join(c.cacheDir, "stations.txt")
		if stations, err := c.loadStationsFromCache(cacheFile); err == nil {
			return stations, nil
		}
	}

	url := "https://www.ncei.noaa.gov/pub/data/ghcn/daily/ghcnd-stations.txt"

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("downloading station list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %s", resp.Status)
	}

	stations, err := c.parseStations(resp.Body)
	if err != nil {
		return nil, err
	}

	// cache station list
	if c.cacheEnabled && cacheFile != "" {
		os.MkdirAll(filepath.Dir(cacheFile), 0755)
		c.saveStationsToCache(cacheFile, stations)
	}

	return stations, nil
}

// parse station list from noaa format
func (c *NOAAClient) parseStations(r io.Reader) ([]Station, error) {
	var stations []Station
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 85 {
			continue
		}

		station := Station{
			ID:        strings.TrimSpace(line[0:11]),
			Latitude:  parseFloat(line[12:20]),
			Longitude: parseFloat(line[21:30]),
			Elevation: parseFloat(line[31:37]),
			State:     strings.TrimSpace(line[38:40]),
			Name:      strings.TrimSpace(line[41:71]),
		}

		// extract country from station id
		if len(station.ID) >= 2 {
			station.Country = station.ID[0:2]
		}

		// parse start/end dates if present (positions 82-90 and 91-99)
		if len(line) > 90 {
			startStr := strings.TrimSpace(line[82:90])
			if startStr != "" {
				station.StartDate, _ = time.Parse("20060102", startStr)
			}
		}
		if len(line) > 98 {
			endStr := strings.TrimSpace(line[91:99])
			if endStr != "" {
				station.EndDate, _ = time.Parse("20060102", endStr)
			}
		}

		stations = append(stations, station)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parsing stations: %w", err)
	}

	return stations, nil
}

// parse csv data from noaa
// handles the full NOAA CSV format with all measurement columns
func (c *NOAAClient) parseCSV(r io.Reader) ([]WeatherRecord, error) {
	reader := csv.NewReader(r)

	// read header to identify measurement columns
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	// build map of measurement column indices
	// measurement columns are at even indices (6, 8, 10, ...)
	// their corresponding _ATTRIBUTES columns are at odd indices (7, 9, 11, ...)
	type measurementCol struct {
		name      string
		valueIdx  int
		attribIdx int
	}

	var measurements []measurementCol
	for i := 6; i < len(header)-1; i += 2 {
		colName := header[i]
		// skip _ATTRIBUTES columns as values
		if !strings.HasSuffix(colName, "_ATTRIBUTES") && i+1 < len(header) {
			measurements = append(measurements, measurementCol{
				name:      colName,
				valueIdx:  i,
				attribIdx: i + 1,
			})
		}
	}

	var records []WeatherRecord
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows
		}

		if len(row) < 6 {
			continue
		}

		stationID := row[0]

		// parse date
		var date time.Time
		if d, err := time.Parse("2006-01-02", row[1]); err == nil {
			date = d
		} else {
			continue // skip rows with invalid dates
		}

		// extract each measurement
		for _, m := range measurements {
			if m.valueIdx >= len(row) {
				continue
			}

			valueStr := strings.TrimSpace(row[m.valueIdx])
			if valueStr == "" || valueStr == "NA" || valueStr == "9999" {
				continue // skip empty or missing values
			}

			// parse value (already in proper units, not tenths)
			value, err := strconv.ParseFloat(valueStr, 64)
			if err != nil {
				continue
			}

			// get quality flag from attributes column
			// NOAA attributes format: mflag,qflag,sflag (comma-separated)
			qualityFlag := ""
			if m.attribIdx < len(row) {
				attribStr := strings.TrimSpace(row[m.attribIdx])
				// Parse attributes: mflag,qflag,sflag
				// Extract just the qflag (2nd position, index 1)
				parts := strings.Split(attribStr, ",")
				if len(parts) >= 2 && len(parts[1]) > 0 {
					qualityFlag = parts[1]
				}
			}

			record := WeatherRecord{
				StationID:   stationID,
				Date:        date,
				Element:     m.name,
				Value:       value,
				QualityFlag: qualityFlag,
			}

			records = append(records, record)
		}
	}

	return records, nil
}

// cache helpers
func (c *NOAAClient) loadFromCache(path string) ([]WeatherRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return c.parseCSV(f)
}

func (c *NOAAClient) loadStationsFromCache(path string) ([]Station, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return c.parseStations(f)
}

func (c *NOAAClient) saveStationsToCache(path string, stations []Station) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, s := range stations {
		fmt.Fprintf(f, "%-11s %8.4f %9.4f %6.1f %-2s %-30s\n",
			s.ID, s.Latitude, s.Longitude, s.Elevation, s.State, s.Name)
	}

	return nil
}

// haversine distance in km
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371 // earth radius in km

	φ1 := lat1 * math.Pi / 180
	φ2 := lat2 * math.Pi / 180
	Δφ := (lat2 - lat1) * math.Pi / 180
	Δλ := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*
			math.Sin(Δλ/2)*math.Sin(Δλ/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

// downloader handles downloading multiple stations
type Downloader struct {
	client  *NOAAClient
	options DownloaderOptions
}

type DownloaderOptions struct {
	OutputDir string
	Format    string
	Parallel  int
	StartDate time.Time
	EndDate   time.Time
	Limit     int
}

type DownloadResult struct {
	StationsProcessed int
	RecordsDownloaded int
	FilesCreated      int
	TotalBytes        int64
	Duration          time.Duration
	Errors            []error
}

func NewDownloader(client *NOAAClient, opts DownloaderOptions) *Downloader {
	return &Downloader{
		client:  client,
		options: opts,
	}
}

func (d *Downloader) Download(ctx context.Context, stations []Station) (DownloadResult, error) {
	start := time.Now()
	result := DownloadResult{}

	// create worker pool
	var wg sync.WaitGroup
	stationChan := make(chan Station, len(stations))
	resultChan := make(chan stationDownloadResult, len(stations))

	// start workers
	workerCount := d.options.Parallel
	if workerCount > len(stations) {
		workerCount = len(stations)
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go d.downloadWorker(ctx, &wg, stationChan, resultChan)
	}

	// send stations to workers
	for _, station := range stations {
		stationChan <- station
	}
	close(stationChan)

	// wait for workers
	wg.Wait()
	close(resultChan)

	// collect results
	for r := range resultChan {
		result.StationsProcessed++
		result.RecordsDownloaded += r.recordCount
		result.TotalBytes += r.bytes
		if r.err != nil {
			result.Errors = append(result.Errors, r.err)
		} else {
			result.FilesCreated++
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

type stationDownloadResult struct {
	station     Station
	recordCount int
	bytes       int64
	err         error
}

func (d *Downloader) downloadWorker(ctx context.Context, wg *sync.WaitGroup, stations <-chan Station, results chan<- stationDownloadResult) {
	defer wg.Done()

	for station := range stations {
		records, err := d.client.DownloadStation(ctx, station)
		if err != nil {
			results <- stationDownloadResult{station: station, err: err}
			continue
		}

		// filter by date
		if !d.options.StartDate.IsZero() || !d.options.EndDate.IsZero() {
			records = d.filterByDate(records)
		}

		// write to file
		filepath := filepath.Join(d.options.OutputDir, fmt.Sprintf("%s.csv", station.ID))
		bytes, err := d.writeCSV(records, filepath)
		if err != nil {
			results <- stationDownloadResult{station: station, err: err}
			continue
		}

		results <- stationDownloadResult{
			station:     station,
			recordCount: len(records),
			bytes:       bytes,
		}
	}
}

func (d *Downloader) filterByDate(records []WeatherRecord) []WeatherRecord {
	var filtered []WeatherRecord
	for _, r := range records {
		if !d.options.StartDate.IsZero() && r.Date.Before(d.options.StartDate) {
			continue
		}
		if !d.options.EndDate.IsZero() && r.Date.After(d.options.EndDate) {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func (d *Downloader) writeCSV(records []WeatherRecord, filepath string) (int64, error) {
	f, err := os.Create(filepath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// Write in NOAA GHCN-Daily format expected by ingestion service
	// Format: station_id,date,element,value,mflag,qflag,sflag
	for _, r := range records {
		// Format date as YYYYMMDD
		dateStr := r.Date.Format("20060102")

		// Use QualityFlag as qflag, leave mflag and sflag empty
		mflag := ""
		qflag := r.QualityFlag
		sflag := ""

		// Format: station_id,date,element,value,mflag,qflag,sflag
		line := fmt.Sprintf("%s,%s,%s,%.1f,%s,%s,%s\n",
			r.StationID, dateStr, r.Element, r.Value, mflag, qflag, sflag)
		f.WriteString(line)
	}

	info, _ := f.Stat()
	return info.Size(), nil
}
