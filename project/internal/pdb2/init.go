package pdb2

import (
	"context"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	"github.com/Khan/genqlient/graphql"
)

// GetProject queries PDB2 to get the metadata of a project referred by `number`, using GraphQL.
func GetProject(config config.CoreAPIConfiguration, number string) (*getProjectResponse, error) {

	c1, err := oauth2HttpClient(
		config.AuthClientID,
		config.AuthClientSecret,
		config.AuthURL,
	)

	if err != nil {
		return nil, err
	}

	return getProject(
		context.Background(),
		graphql.NewClient(config.CoreAPIURL, c1),
		number,
	)
}

//go:generate go run github.com/Khan/genqlient genqlient.yaml
