// Package filergateway provides client interfaces of the filer-gateway.
package filergateway

import (
	"fmt"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	"github.com/Donders-Institute/tg-toolset-golang/project/pkg/pdb"
)

// NewClient returns a new `filerGateway` instance with settings
// given by the `config` file.
func NewClient(config config.Configuration) (FilerGateway, error) {
	return FilerGateway{
		apiKey:  config.FilerGateway.APIKey,
		apiURL:  config.FilerGateway.APIURL,
		apiUser: config.FilerGateway.APIUser,
		apiPass: config.FilerGateway.APIPass,
	}, nil
}

// FilerGateway implements client interfaces of the FilerGateway.
type FilerGateway struct {
	apiKey  string
	apiURL  string
	apiUser string
	apiPass string
}

// UpdateProject updates or creates filer storage with information given by the `data`.
func (f *FilerGateway) UpdateProject(projectID string, data *pdb.DataProjectUpdate) error {

	return fmt.Errorf("not implemented")
}
