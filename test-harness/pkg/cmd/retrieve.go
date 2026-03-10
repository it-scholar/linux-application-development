package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"weather-station-test/pkg/data"
)

var retrieveCmd = &cobra.Command{
	Use:   "retrieve",
	Short: "download weather data from noaa",
	Long: `download weather data from noaa global historical climatology network (ghcn-daily).

data source: https://www.ncei.noaa.gov/data/global-historical-climatology-network-daily/access/
coverage: 1750-present, 100k+ stations worldwide
includes: temperature, precipitation, wind, pressure

examples:
  # download data for specific station
  test-harness retrieve --station usw00014739 --start 2020-01-01 --end 2023-12-31

  # find and download stations near location
  test-harness retrieve --lat 52.5200 --lon 13.4050 --radius 100 --limit 50000

  # download all german stations
  test-harness retrieve --country de --start 2022-01-01

	# parallel download with caching
  test-harness retrieve --country us --parallel 8 --cache

  # overnight loading with rate limiting and disk space protection
  test-harness retrieve --country us --limit 100000 --rate-limit 0.5 --min-free-space 5.0 --cache`,
	RunE: runRetrieve,
}

var (
	retrieveFlags struct {
		station        string
		country        string
		lat            float64
		lon            float64
		radius         float64
		start          string
		end            string
		limit          int
		output         string
		format         string
		parallel       int
		cache          bool
		cacheDir       string
		rateLimit      float64
		minFreeSpaceGB float64
	}
)

func init() {
	rootCmd.AddCommand(retrieveCmd)

	retrieveCmd.Flags().StringVar(&retrieveFlags.station, "station", "", "station id (e.g., usw00014739)")
	retrieveCmd.Flags().StringVar(&retrieveFlags.country, "country", "", "country code (e.g., us, de, gb)")
	retrieveCmd.Flags().Float64Var(&retrieveFlags.lat, "lat", 0, "latitude for radius search")
	retrieveCmd.Flags().Float64Var(&retrieveFlags.lon, "lon", 0, "longitude for radius search")
	retrieveCmd.Flags().Float64Var(&retrieveFlags.radius, "radius", 50, "search radius in km")
	retrieveCmd.Flags().StringVar(&retrieveFlags.start, "start", "", "start date (yyyy-mm-dd)")
	retrieveCmd.Flags().StringVar(&retrieveFlags.end, "end", "", "end date (yyyy-mm-dd)")
	retrieveCmd.Flags().IntVar(&retrieveFlags.limit, "limit", 10000, "max records to download")
	retrieveCmd.Flags().StringVarP(&retrieveFlags.output, "output", "o", "./data/csv", "output directory")
	retrieveCmd.Flags().StringVar(&retrieveFlags.format, "format", "csv", "output format (csv|json)")
	retrieveCmd.Flags().IntVarP(&retrieveFlags.parallel, "parallel", "p", 4, "parallel downloads")
	retrieveCmd.Flags().BoolVar(&retrieveFlags.cache, "cache", false, "use local cache")
	retrieveCmd.Flags().StringVar(&retrieveFlags.cacheDir, "cache-dir", "", "cache directory (default ~/.cache/ws-test/noaa)")
	retrieveCmd.Flags().Float64Var(&retrieveFlags.rateLimit, "rate-limit", 1.0, "requests per second (default 1.0, 0 = unlimited)")
	retrieveCmd.Flags().Float64Var(&retrieveFlags.minFreeSpaceGB, "min-free-space", 1.0, "minimum free space in GB before stopping (default 1.0)")
}

func runRetrieve(cmd *cobra.Command, args []string) error {
	// setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Warn("received interrupt signal, shutting down...")
		cancel()
	}()

	// parse dates
	var startDate, endDate time.Time
	var err error

	if retrieveFlags.start != "" {
		startDate, err = time.Parse("2006-01-02", retrieveFlags.start)
		if err != nil {
			logger.Error("invalid start date format, expected yyyy-mm-dd", "error", err)
			return err
		}
	}

	if retrieveFlags.end != "" {
		endDate, err = time.Parse("2006-01-02", retrieveFlags.end)
		if err != nil {
			logger.Error("invalid end date format, expected yyyy-mm-dd", "error", err)
			return err
		}
	}

	// setup cache directory
	cacheDir := retrieveFlags.cacheDir
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			logger.Error("could not determine home directory", "error", err)
			return err
		}
		cacheDir = filepath.Join(home, ".cache", "ws-test", "noaa")
	}

	// create output directory
	if err := os.MkdirAll(retrieveFlags.output, 0755); err != nil {
		logger.Error("could not create output directory", "error", err)
		return err
	}

	// create noaa client
	client := data.NewNOAAClient(data.NOAAOptions{
		BaseURL:        "https://www.ncei.noaa.gov/data/global-historical-climatology-network-daily/access/",
		CacheDir:       cacheDir,
		CacheEnabled:   retrieveFlags.cache,
		Timeout:        30 * time.Second,
		Parallel:       retrieveFlags.parallel,
		RequestsPerSec: retrieveFlags.rateLimit,
		MinFreeSpaceGB: retrieveFlags.minFreeSpaceGB,
	})

	// print header
	logger.Info("=== noaa weather data retrieval ===")
	logger.Info("configuration",
		"output_directory", retrieveFlags.output,
		"parallel_downloads", retrieveFlags.parallel,
		"cache_enabled", retrieveFlags.cache,
		"rate_limit", fmt.Sprintf("%.1f req/s", retrieveFlags.rateLimit),
		"min_free_space", fmt.Sprintf("%.1f GB", retrieveFlags.minFreeSpaceGB),
	)

	if retrieveFlags.cache {
		logger.Info("cache configuration", "cache_dir", cacheDir)
	}
	if !startDate.IsZero() {
		logger.Info("date filter", "start", startDate.Format("2006-01-02"))
	}
	if !endDate.IsZero() {
		logger.Info("date filter", "end", endDate.Format("2006-01-02"))
	}

	// determine retrieval mode
	var stations []data.Station

	switch {
	case retrieveFlags.station != "":
		logger.Info("searching for station", "station_id", retrieveFlags.station)
		station, err := client.GetStation(retrieveFlags.station)
		if err != nil {
			logger.Error("station not found", "station_id", retrieveFlags.station, "error", err)
			return err
		}
		stations = append(stations, station)
		logger.Info("found station",
			"name", station.Name,
			"id", station.ID,
			"lat", station.Latitude,
			"lon", station.Longitude,
		)

	case retrieveFlags.country != "":
		logger.Info("searching for stations in country", "country", retrieveFlags.country)
		stations, err = client.GetStationsByCountry(retrieveFlags.country)
		if err != nil {
			logger.Error("failed to get stations", "country", retrieveFlags.country, "error", err)
			return err
		}

	case retrieveFlags.lat != 0 && retrieveFlags.lon != 0:
		logger.Info("searching for stations by location",
			"lat", retrieveFlags.lat,
			"lon", retrieveFlags.lon,
			"radius_km", retrieveFlags.radius,
		)
		stations, err = client.GetStationsByRadius(retrieveFlags.lat, retrieveFlags.lon, retrieveFlags.radius)
		if err != nil {
			logger.Error("failed to get stations by radius", "error", err)
			return err
		}

	default:
		logger.Error("must specify either --station, --country, or --lat/--lon")
		return nil
	}

	if len(stations) == 0 {
		logger.Error("no stations found matching criteria")
		return nil
	}

	logger.Info("found stations", "count", len(stations))

	// limit number of stations if needed
	if len(stations) > retrieveFlags.limit {
		logger.Warn("limiting station count", "original", len(stations), "limit", retrieveFlags.limit)
		stations = stations[:retrieveFlags.limit]
	}

	// show station summary
	if len(stations) <= 10 {
		logger.Info("stations to download")
		for _, s := range stations {
			logger.Info("  station",
				"id", s.ID,
				"name", s.Name,
				"lat", s.Latitude,
				"lon", s.Longitude,
			)
		}
	}

	// download data
	downloader := data.NewDownloader(client, data.DownloaderOptions{
		OutputDir: retrieveFlags.output,
		Format:    retrieveFlags.format,
		Parallel:  retrieveFlags.parallel,
		StartDate: startDate,
		EndDate:   endDate,
		Limit:     retrieveFlags.limit,
	})

	results, err := downloader.Download(ctx, stations)
	if err != nil {
		logger.Error("download failed", "error", err)
		return err
	}

	// print summary
	logger.Info("=== download summary ===",
		"stations_processed", results.StationsProcessed,
		"records_downloaded", results.RecordsDownloaded,
		"files_created", results.FilesCreated,
		"total_size_mb", float64(results.TotalBytes)/(1024*1024),
		"duration", results.Duration.Round(time.Second),
	)

	if len(results.Errors) > 0 {
		logger.Warn("download completed with errors", "error_count", len(results.Errors))
		for _, err := range results.Errors {
			logger.Error("  download error", "error", err)
		}
	}

	logger.Info("output saved", "directory", retrieveFlags.output)

	return nil
}
