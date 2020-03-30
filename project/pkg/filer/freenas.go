package filer

type FreeNas struct {
}

func (filer FreeNas) CreateProject(projectID string, quotaGiB int) error {
	return nil
}

func (filer FreeNas) CreateHome(username, groupname string, quotaGiB int) error {
	return nil
}

func (filer FreeNas) SetProjectQuota(projectID string, quotaGiB int) error {
	return nil
}

func (filer FreeNas) SetHomeQuota(username, groupname string, quotaGiB int) error {
	return nil
}
