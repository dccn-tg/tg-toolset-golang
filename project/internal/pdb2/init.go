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

// GetUser queries PDB2 to get the metadata of a user referred by `username`, using GraphQL.
func GetUser(config config.CoreAPIConfiguration, username string) (*getUserResponse, error) {

	c1, err := oauth2HttpClient(
		config.AuthClientID,
		config.AuthClientSecret,
		config.AuthURL,
	)

	if err != nil {
		return nil, err
	}

	return getUser(
		context.Background(),
		graphql.NewClient(config.CoreAPIURL, c1),
		username,
	)
}

//go:generate go run github.com/Khan/genqlient genqlient.yaml
