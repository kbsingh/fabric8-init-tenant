package openshift

import (
	"fmt"
	"strings"

	"github.com/fabric8io/fabric8-init-tenant/template"
)

const (
	varProjectName           = "PROJECT_NAME"
	varProjectDisplayName    = "PROJECT_DISPLAYNAME"
	varProjectDescription    = "PROJECT_DESCRIPTION"
	varProjectUser           = "PROJECT_USER"
	varProjectRequestingUser = "PROJECT_REQUESTING_USER"
	varProjectAdminUser      = "PROJECT_ADMIN_USER"
	varProjectNamespace      = "PROJECT_NAMESPACE"
)

// InitTenant initializes a new tenant in openshift
// Creates the new n-tuneim|develop,ment|staging and x-dsaas-* namespaces
// and install the required services/routes/deployment configurations to run
// e.g. Jenkins and Che
func InitTenant(config Config, username, usertoken string) error {
	err := do(config, username, usertoken)
	if err != nil {
		return err
	}
	return nil
}

func do(config Config, username, usertoken string) error {
	name := createName(username)

	vars := map[string]string{
		varProjectName:           name,
		varProjectDisplayName:    name + " Test Project",
		varProjectDescription:    name + " Test Project",
		varProjectUser:           username,
		varProjectRequestingUser: username,
		varProjectAdminUser:      config.MasterUser,
	}

	masterOpts := ApplyOptions{Config: config, Overwrite: true}
	userOpts := ApplyOptions{Config: config.WithToken(usertoken), Namespace: name, Overwrite: true}

	userProjectT, err := template.Asset("fabric8-online-team-openshift.yml")
	if err != nil {
		return err
	}

	projectT, err := template.Asset("fabric8-online-project-openshift.yml")
	if err != nil {
		return err
	}
	jenkinsT, err := template.Asset("fabric8-online-jenkins-openshift.yml")
	if err != nil {
		return err
	}
	cheT, err := template.Asset("fabric8-online-che-openshift.yml")
	if err != nil {
		return err
	}

	var channels []chan error
	err = executeNamespaceSync(string(userProjectT), vars, userOpts)
	if err != nil {
		return err
	}

	namespaces := []string{"%v-test", "%v-stage", "%v-run"}

	for _, pattern := range namespaces {
		lvars := clone(vars)
		lvars[varProjectName] = fmt.Sprintf(pattern, vars[varProjectName])
		lvars[varProjectDisplayName] = lvars[varProjectName]

		ns := executeNamespaceAsync(string(projectT), lvars, masterOpts)
		channels = append(channels, ns)
	}

	{
		lvars := clone(vars)
		lvars[varProjectName] = fmt.Sprintf("%v-jenkins", vars[varProjectName])
		lvars[varProjectNamespace] = vars[varProjectName]
		ns := executeNamespaceAsync(string(jenkinsT), lvars, masterOpts)
		channels = append(channels, ns)
	}
	{
		lvars := clone(vars)
		lvars[varProjectName] = fmt.Sprintf("%v-che", vars[varProjectName])
		ns := executeNamespaceAsync(string(cheT), lvars, masterOpts)
		channels = append(channels, ns)
	}

	var errors []error
	for _, channel := range channels {
		err := <-channel
		if err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) > 0 {
		return multiError{Errors: errors}
	}
	return nil
}

func createName(username string) string {
	return strings.Replace(strings.Split(username, "@")[0], ".", "-", -1)
}

func executeNamespaceSync(template string, vars map[string]string, opts ApplyOptions) error {
	t, err := Process(template, vars)
	if err != nil {
		return err
	}

	err = Apply(t, opts)
	if err != nil {
		return err
	}
	return nil
}

func executeNamespaceAsync(template string, vars map[string]string, opts ApplyOptions) chan error {
	ch := make(chan error)
	go func() {
		lopts := ApplyOptions{
			Config:    opts.Config,
			Namespace: vars[varProjectName],
		}

		t, err := Process(template, vars)
		if err != nil {
			ch <- err
		}

		err = Apply(t, lopts)
		if err != nil {
			ch <- err
		}

		ch <- nil
		close(ch)
	}()
	return ch
}

func clone(maps map[string]string) map[string]string {
	maps2 := make(map[string]string)
	for k2, v2 := range maps {
		maps2[k2] = v2
	}
	return maps2
}
