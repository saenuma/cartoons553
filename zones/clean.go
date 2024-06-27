package main

import (
	"os"
	"strings"

	// "fmt"
	"encoding/json"

	"github.com/tidwall/pretty"
)

func main() {
	raw, err := os.ReadFile("raw_zones_output.csv")
	if err != nil {
		panic(err)
	}

	rawStrLines := strings.Split(string(raw), "\n")
	regions := make(map[string][]string)
	for _, line := range rawStrLines[1:] {
		partsOfLine := strings.Split(line, ",")
		if len(partsOfLine) != 2 {
			continue
		}
		zones, ok := regions[partsOfLine[1]]
		if !ok {
			tmpZones := []string{partsOfLine[0]}
			regions[partsOfLine[1]] = tmpZones
		} else {
			tmpZones := append(zones, partsOfLine[0])
			regions[partsOfLine[1]] = tmpZones
		}
	}

	jsonBytes, err := json.Marshal(regions)
	if err != nil {
		panic(err)
	}
	os.WriteFile("zones.json", pretty.Pretty(jsonBytes), 0777)
}
