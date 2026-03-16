package data

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultGenerateYears = 100
)

var sizePattern = regexp.MustCompile(`(?i)^\s*(\d+(?:\.\d+)?)\s*([kmg]?b)?\s*$`)

type SyntheticGenerator struct {
	rng  *rand.Rand
	seed int64
}

type GenerateRequest struct {
	OutputPath     string
	TargetBytes    int64
	StartTime      time.Time
	Years          int
	TolerancePct   float64
	StationPrefix  string
	HeaderWithMeta bool
}

type GenerateResult struct {
	OutputPath   string
	TargetBytes  int64
	ActualBytes  int64
	RowsWritten  int64
	StationsUsed int
	Seed         int64
	InTolerance  bool
}

func NewSyntheticGenerator(seed int64) *SyntheticGenerator {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	return &SyntheticGenerator{
		rng:  rand.New(rand.NewSource(seed)),
		seed: seed,
	}
}

func (g *SyntheticGenerator) Seed() int64 {
	return g.seed
}

func ParseByteSize(value string) (int64, error) {
	m := sizePattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(m) != 3 {
		return 0, fmt.Errorf("invalid size %q (examples: 100MB, 1GB, 250MB)", value)
	}

	number, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size number %q: %w", m[1], err)
	}

	if number <= 0 {
		return 0, fmt.Errorf("size must be greater than 0")
	}

	unit := strings.ToLower(m[2])
	var multiplier float64
	switch unit {
	case "", "b":
		multiplier = 1
	case "kb":
		multiplier = 1024
	case "mb":
		multiplier = 1024 * 1024
	case "gb":
		multiplier = 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unsupported size unit %q", unit)
	}

	bytes := int64(number * multiplier)
	if bytes <= 0 {
		return 0, fmt.Errorf("size is too small")
	}

	return bytes, nil
}

func HumanByteSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)

	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.2fGB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.2fMB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.2fKB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

func (g *SyntheticGenerator) GenerateCSV(req GenerateRequest) (GenerateResult, error) {
	if req.OutputPath == "" {
		return GenerateResult{}, fmt.Errorf("output path is required")
	}
	if req.TargetBytes <= 0 {
		return GenerateResult{}, fmt.Errorf("target size must be greater than 0")
	}
	if req.Years <= 0 {
		req.Years = DefaultGenerateYears
	}
	if req.StartTime.IsZero() {
		req.StartTime = time.Date(1926, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if req.StartTime.Location() != time.UTC {
		req.StartTime = req.StartTime.UTC()
	}
	if req.TolerancePct <= 0 {
		req.TolerancePct = 5.0
	}
	if req.StationPrefix == "" {
		req.StationPrefix = "FAKE"
	}

	rowsPerStation := hourlyRows(req.StartTime, req.StartTime.AddDate(req.Years, 0, 0))
	if rowsPerStation <= 0 {
		return GenerateResult{}, fmt.Errorf("invalid row window")
	}

	header := "timestamp,station_id,temperature_c,humidity_pct,pressure_hpa,wind_speed_mps,wind_dir_deg,rain_mm\n"

	// Estimate baseline row length from one sample row.
	sampleLine := g.buildLine(req.StartTime, req.StationPrefix+"0001", weatherParams{stationIndex: 0, climateOffset: 0, phaseOffset: 0})
	baseRowLen := len(sampleLine)
	if baseRowLen <= 0 {
		return GenerateResult{}, fmt.Errorf("could not estimate row length")
	}

	available := req.TargetBytes - int64(len(header))
	if available <= 0 {
		return GenerateResult{}, fmt.Errorf("target size too small for CSV header")
	}

	minOneStation := int64(baseRowLen) * rowsPerStation
	if available < minOneStation {
		return GenerateResult{}, fmt.Errorf(
			"requested size %s is too small for 100 years hourly data (minimum about %s)",
			HumanByteSize(req.TargetBytes),
			HumanByteSize(minOneStation+int64(len(header))),
		)
	}

	approxStations := float64(available) / float64(minOneStation)
	floorStations := int(math.Floor(approxStations))
	if floorStations < 1 {
		floorStations = 1
	}
	ceilStations := int(math.Ceil(approxStations))
	if ceilStations < 1 {
		ceilStations = 1
	}

	stations := floorStations
	floorDelta := absInt64(available - int64(floorStations)*minOneStation)
	ceilDelta := absInt64(available - int64(ceilStations)*minOneStation)
	if ceilDelta < floorDelta {
		stations = ceilStations
	}

	totalRows := rowsPerStation * int64(stations)

	if err := os.MkdirAll(filepath.Dir(req.OutputPath), 0755); err != nil {
		return GenerateResult{}, fmt.Errorf("creating output directory: %w", err)
	}

	f, err := os.Create(req.OutputPath)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	writer := bufio.NewWriterSize(f, 1<<20)
	written := int64(0)

	n, err := writer.WriteString(header)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("writing CSV header: %w", err)
	}
	written += int64(n)

	stationParams := make([]weatherParams, stations)
	for i := 0; i < stations; i++ {
		stationParams[i] = weatherParams{
			stationIndex:  i,
			climateOffset: g.rng.Float64()*10 - 5,
			phaseOffset:   g.rng.Float64() * 2 * math.Pi,
		}
	}

	for s := 0; s < stations; s++ {
		stationID := fmt.Sprintf("%s%04d", req.StationPrefix, s+1)
		for ts := req.StartTime; ts.Before(req.StartTime.AddDate(req.Years, 0, 0)); ts = ts.Add(time.Hour) {
			line := g.buildLine(ts, stationID, stationParams[s])
			n, err := writer.WriteString(line)
			if err != nil {
				return GenerateResult{}, fmt.Errorf("writing CSV row: %w", err)
			}
			written += int64(n)
		}
	}

	if err := writer.Flush(); err != nil {
		return GenerateResult{}, fmt.Errorf("flushing output: %w", err)
	}

	result := GenerateResult{
		OutputPath:   req.OutputPath,
		TargetBytes:  req.TargetBytes,
		ActualBytes:  written,
		RowsWritten:  totalRows,
		StationsUsed: stations,
		Seed:         g.seed,
	}

	toleranceBytes := int64(float64(req.TargetBytes) * (req.TolerancePct / 100.0))
	delta := result.ActualBytes - req.TargetBytes
	if delta < 0 {
		delta = -delta
	}
	result.InTolerance = delta <= toleranceBytes

	return result, nil
}

type weatherParams struct {
	stationIndex  int
	climateOffset float64
	phaseOffset   float64
}

func hourlyRows(start, end time.Time) int64 {
	rows := int64(0)
	for ts := start; ts.Before(end); ts = ts.Add(time.Hour) {
		rows++
	}
	return rows
}

func (g *SyntheticGenerator) buildLine(ts time.Time, stationID string, p weatherParams) string {
	metrics := g.generateMetrics(ts, p)

	return fmt.Sprintf("%s,%s,%.2f,%.2f,%.2f,%.2f,%d,%.2f\n",
		ts.UTC().Format(time.RFC3339),
		stationID,
		metrics.temperatureC,
		metrics.humidityPct,
		metrics.pressureHPA,
		metrics.windSpeedMPS,
		metrics.windDirDeg,
		metrics.rainMM,
	)
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

type weatherMetrics struct {
	temperatureC  float64
	humidityPct   float64
	pressureHPA   float64
	windSpeedMPS  float64
	windDirDeg    int
	rainMM        float64
	qualityWeight float64
}

func (g *SyntheticGenerator) generateMetrics(ts time.Time, p weatherParams) weatherMetrics {
	yearDayNorm := float64(ts.YearDay()) / 365.25
	hourNorm := float64(ts.Hour()) / 24.0

	seasonal := math.Sin(2*math.Pi*(yearDayNorm-0.25) + p.phaseOffset)
	diurnal := math.Sin(2 * math.Pi * (hourNorm - 0.2))

	tempNoise := g.rng.NormFloat64() * 1.2
	temperature := 14.5 + p.climateOffset + 11.5*seasonal + 5.8*diurnal + tempNoise

	humidityNoise := g.rng.NormFloat64() * 4.0
	humidity := clamp(72.0-0.9*(temperature-15)-8.0*diurnal+humidityNoise, 12.0, 100.0)

	pressureNoise := g.rng.NormFloat64() * 1.7
	pressure := 1013.25 + 6.5*math.Sin(2*math.Pi*yearDayNorm+p.phaseOffset*0.35) + pressureNoise

	windBase := 4.0 + 2.1*math.Sin(2*math.Pi*yearDayNorm+1.1) + math.Abs(g.rng.NormFloat64())*1.8
	windSpeed := clamp(windBase, 0.0, 42.0)
	windDirection := int(math.Mod(float64(ts.Hour()*13+p.stationIndex*41)+g.rng.Float64()*35.0, 360.0))

	rainChance := clamp(0.08+0.004*(humidity-55.0)+0.03*(1.0+seasonal), 0.02, 0.72)
	rain := 0.0
	if g.rng.Float64() < rainChance {
		rain = clamp(math.Abs(g.rng.NormFloat64())*2.8+0.2, 0.1, 75.0)
	}

	return weatherMetrics{
		temperatureC:  temperature,
		humidityPct:   humidity,
		pressureHPA:   pressure,
		windSpeedMPS:  windSpeed,
		windDirDeg:    windDirection,
		rainMM:        rain,
		qualityWeight: 1.0,
	}
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
