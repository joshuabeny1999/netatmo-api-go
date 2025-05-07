// api-test.go: simple CLI to exercise the Netatmo client (auto-persisting TOML)

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	netatmo "github.com/joshuabeny1999/netatmo-api-go/v2"
)

// Usage: go run api-test.go -f netatmo.toml
var fConfig = flag.String("f", "", "Netatmo configuration file (TOML)")

func main() {
	flag.Parse()
	if *fConfig == "" {
		fmt.Fprintln(os.Stderr, "Missing required argument -f")
		os.Exit(1)
	}

	// Load and persistable-config
	cfg, err := netatmo.LoadConfig(*fConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot load config %s: %v\n", *fConfig, err)
		os.Exit(1)
	}

	// Initialize client (will auto-refresh and save tokens)
	client, err := netatmo.NewClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize Netatmo client: %v\n", err)
		os.Exit(1)
	}

	// Fetch stations and modules
	dc, err := client.Read()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Netatmo Read error: %v\n", err)
		os.Exit(1)
	}

	// Print data
	now := time.Now().UTC().Unix()
	for _, station := range dc.Stations() {
		fmt.Printf("Station: %s\n", station.StationName)
		fmt.Printf(
			"\tCity: %s\n\tCountry: %s\n\tTimezone: %s\n\tLongitude: %v\n\tLatitude: %v\n\tAltitude: %v\n\n",
			station.Place.City,
			station.Place.Country,
			station.Place.Timezone,
			station.Place.Location.Longitude,
			station.Place.Location.Latitude,
			station.Place.Altitude,
		)

		for _, module := range station.Modules() {
			fmt.Printf("\tModule: %s\n", module.ModuleName)

			// Info (battery, wifi, etc)
			if module.DashboardData.LastMeasure != nil {
				ts, info := module.Info()
				for k, v := range info {
					fmt.Printf("\t\t%s: %v (age %ds)\n", k, v, now-ts)
				}
			}

			// Data (temperature, humidity, etc)
			if module.DashboardData.LastMeasure != nil {
				ts, data := module.Data()
				for k, v := range data {
					fmt.Printf("\t\t%s: %v (age %ds)\n", k, v, now-ts)
				}
			}
		}
	}
}
