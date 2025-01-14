package registry

import (
	"fmt"
	"os"
	"path"

	"github.com/common-fate/clio"
	grantedConfig "github.com/common-fate/granted/pkg/config"

	"github.com/urfave/cli/v2"
)

var AddCommand = cli.Command{
	Name:        "add",
	Description: "Add a Profile Registry that you want to sync with aws config file",
	Usage:       "Provide git repository you want to sync with aws config file",
	ArgsUsage:   "<repository url> ...",
	Action: func(c *cli.Context) error {

		if c.Args().Len() < 1 {
			clio.Error("Repository argument is required. Try 'granted registry add <https://github.com/your-org/your-registry.git>'")
		}

		var repoURLs []string

		n := 0
		for n < c.Args().Len() {
			repoURLs = append(repoURLs, c.Args().Get(n))
			n++
		}

		gConf, err := grantedConfig.Load()
		if err != nil {
			return err
		}

		for index, repoURL := range repoURLs {
			clio.Debugf("parsing the provided url to get host, organization and repo name for %s", repoURL)
			url, err := parseGitURL(repoURL)
			if err != nil {
				return err
			}

			// skip if the git url is already present.
			if URLExists(gConf.ProfileRegistryURLS, url) {
				clio.Warnf("Already subscribed to '%s'. Skipping adding this registry. Use 'granted registry sync' cmd instead to sync the config files.", repoURL)

				continue
			}

			repoDirPath, err := getRegistryLocation(url)
			if err != nil {
				return err
			}

			if _, err = os.Stat(repoDirPath); err != nil {
				// directory doesn't exist; clone the repo
				if os.IsNotExist(err) {
					err = gitClone(url.GetURL(), repoDirPath)
					if err != nil {
						return err
					}

					// //if a specific ref is passed we will checkout that ref
					// if addFlags.String("ref") != "" {
					// 	fmt.Println("attempting to checkout branch" + addFlags.String("ref"))

					// 	err = checkoutRef(addFlags.String("ref"), repoDirPath)
					// 	if err != nil {
					// 		return err

					// 	}
					// }

				} else {
					// other error. Should not happen.
					return err
				}
			} else {
				// file exists; pull instead of clone.
				clio.Debugf("%s already exists; pulling instead of cloning. ", url.GetURL())
				if err = gitPull(repoDirPath, false); err != nil {
					return err
				}
			}

			//if a specific ref is passed we will checkout that ref
			// if addFlags.String("ref") != "" {
			// 	fmt.Println("attempting to checkout branch" + addFlags.String("ref"))
			// 	err = checkoutRef(addFlags.String("ref"), repoDirPath)
			// 	if err != nil {
			// 		return err

			// 	}
			// }

			// check if the fetched cloned repo contains granted.yml file.
			if err = parseClonedRepo(repoDirPath, url); err != nil {
				return err
			}

			// if there are no granted registry setup yet then
			// check if this is the first index
			isFirstSection := false
			if len(gConf.ProfileRegistryURLS) == 0 {
				if index == 0 {
					isFirstSection = true
				}
			}

			var r Registry
			_, err = r.Parse(repoDirPath, url)
			if err != nil {
				return err
			}

			awsConfigPath, err := getDefaultAWSConfigLocation()
			if err != nil {
				return err
			}

			if _, err := os.Stat(awsConfigPath); os.IsNotExist(err) {
				clio.Debugf("%s file does not exist. Creating an empty file\n", awsConfigPath)
				_, err := os.Create(awsConfigPath)
				if err != nil {
					return fmt.Errorf("unable to create : %s", err)
				}
			}

			// Sync clonned repo content with aws config file.
			if err := Sync(r, repoURL, repoDirPath, isFirstSection); err != nil {
				return err
			}

			// we have verified that this registry is a valid one and sync is completed.
			// so save the repo url to config file.
			gConf.ProfileRegistryURLS = append(gConf.ProfileRegistryURLS, repoURL)
			if err := gConf.Save(); err != nil {
				return err
			}
		}

		return nil
	},
}

func parseClonedRepo(folderpath string, url GitURL) error {
	var grantedConfigFilename string = "granted.yml"

	if url.Filename != "" {
		grantedConfigFilename = url.Filename
	}

	configFilePath := path.Join(folderpath, url.Subpath, grantedConfigFilename)

	clio.Debugf("checking for %s in %s", grantedConfigFilename, configFilePath)
	_, err := os.ReadFile(configFilePath)
	if err != nil {
		clio.Debug(err)
		return fmt.Errorf("unable to find `%s` file in %s", grantedConfigFilename, configFilePath)
	}

	return nil
}
