package knockrd

import (
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
)

type awsIPRanges struct {
	Prefixes []awsIPRangePrefix `json:"prefixes"`
}

type awsIPRangePrefix struct {
	IPPrefix           string `json:"ip_prefix"`
	Region             string `json:"region"`
	Service            string `json:"service"`
	NetworkBorderGroup string `json:"network_border_group"`
}

func fetchCloudFrontCIRDs() ([]string, error) {
	resp, err := http.Get("https://ip-ranges.amazonaws.com/ip-ranges.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get ip-ranges.json")
	}
	defer resp.Body.Close()
	var ranges awsIPRanges
	if err = json.NewDecoder(resp.Body).Decode(&ranges); err != nil {
		return nil, errors.Wrap(err, "failed to parse body of ip-ranges.json")
	}

	var cidrs []string
	for _, p := range ranges.Prefixes {
		if p.Service != "CLOUDFRONT" {
			continue
		}
		cidrs = append(cidrs, p.IPPrefix)
	}
	return cidrs, nil
}
