package pdb2

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/Donders-Institute/tg-toolset-golang/pkg/config"
	"github.com/Khan/genqlient/graphql"
)

// GetProjects queries PDB2 to get metadata of all projects, using GraphQL.
func GetProjects(config config.CoreAPIConfiguration) (*getProjectsResponse, error) {

	c1, err := oauth2HttpClient(
		config.AuthClientID,
		config.AuthClientSecret,
		config.AuthURL,
	)

	if err != nil {
		return nil, err
	}

	return getProjects(
		context.Background(),
		graphql.NewClient(config.CoreAPIURL, c1),
	)
}

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

	resp, err := getProject(
		context.Background(),
		graphql.NewClient(config.CoreAPIURL, c1),
		number,
	)

	if err != nil {
		return nil, err
	}

	if resp.Project.Number != number {
		return nil, fmt.Errorf("project not found, number: %s", number)
	}

	return resp, nil

}

// GetUsers queries PDB2 to get metadata of all users, using GraphQL.
func GetUsers(config config.CoreAPIConfiguration) (*getUsersResponse, error) {

	c1, err := oauth2HttpClient(
		config.AuthClientID,
		config.AuthClientSecret,
		config.AuthURL,
	)

	if err != nil {
		return nil, err
	}

	return getUsers(
		context.Background(),
		graphql.NewClient(config.CoreAPIURL, c1),
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

	resp, err := getUser(
		context.Background(),
		graphql.NewClient(config.CoreAPIURL, c1),
		username,
	)

	if err != nil {
		return nil, err
	}

	if resp.User.Username != username {
		return nil, fmt.Errorf("user not found, username: %s", username)
	}

	return resp, nil
}

// GetLabs queries PDB2 to get the IDs of certain lab modality.
func GetLabs(config config.CoreAPIConfiguration, modality *regexp.Regexp) ([]string, error) {

	c1, err := oauth2HttpClient(
		config.AuthClientID,
		config.AuthClientSecret,
		config.AuthURL,
	)

	if err != nil {
		return nil, err
	}

	resp, err := getLabs(
		context.Background(),
		graphql.NewClient(config.CoreAPIURL, c1),
	)

	if err != nil {
		return nil, err
	}

	labs := []string{}
	for _, l := range resp.Labs {
		if modality.Match([]byte(l.Modality.Id)) {
			labs = append(labs, l.Id)
		}
	}

	return labs, nil
}

// GetBookings queries PDB2 to get metadata of all bookings on the lab `resource` between `start` and `end`, using GraphQL.
func GetBookingEvents(config config.CoreAPIConfiguration, resources []string, start, end time.Time) (*getBookingEventsResponse, error) {

	c1, err := oauth2HttpClient(
		config.AuthClientID,
		config.AuthClientSecret,
		config.AuthURL,
	)

	if err != nil {
		return nil, err
	}

	// construct resources input
	inputResources := make([]ResourceID, len(resources))
	for i, r := range resources {
		inputResources[i].Id = r
		inputResources[i].Type = ResourceTypeLab
	}

	return getBookingEvents(
		context.Background(),
		graphql.NewClient(config.CoreAPIURL, c1),
		start,
		end,
		inputResources,
	)
}

func LabResource(resource getBookingEventsBookingEventsBookingEventResource) (*getBookingEventsBookingEventsBookingEventResourceLab, error) {
	if lab, ok := resource.(*getBookingEventsBookingEventsBookingEventResourceLab); ok {
		return lab, nil
	}
	return nil, fmt.Errorf("not a lab resource: %s", resource.GetTypename())
}

//go:generate go run github.com/Khan/genqlient genqlient.yaml
