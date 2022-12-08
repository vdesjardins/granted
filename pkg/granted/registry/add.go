package registry

import (
	"fmt"
	"os"

	"github.com/common-fate/clio"
	grantedConfig "github.com/common-fate/granted/pkg/config"

	"github.com/urfave/cli/v2"
)

var AddCommand = cli.Command{
	Name:        "add",
	Description: "Add a Profile Registry that you want to sync with aws config file",
	Usage:       "Provide git repository you want to sync with aws config file",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "name", Required: true, Usage: "name is used to uniquely identify profile registries", Aliases: []string{"n"}},
		&cli.StringFlag{Name: "url", Required: true, Usage: "git url for the remote repository", Aliases: []string{"u"}},
		&cli.StringFlag{Name: "path", Usage: "provide path if only the subfolder needs to be synced", Aliases: []string{"p"}},
		&cli.StringFlag{Name: "filename", Aliases: []string{"f"}, Usage: "provide filename if yml file is not granted.yml", DefaultText: "granted.yml"},
		&cli.IntFlag{Name: "priority", Usage: "profile registry will be sorted by priority descending", Value: 0},
		&cli.StringFlag{Name: "ref", Hidden: true},
		&cli.BoolFlag{Name: "prefix-all-profiles", Aliases: []string{"add-prefix"}, Usage: "provide this flag if you want to append registry name to all profiles"},
		&cli.StringSliceFlag{Name: "requiredKey", Aliases: []string{"r"}, Usage: "used to bypass the prompt or override user specific values"}},
	ArgsUsage: "<repository url> --name <registry_name> --url <git-url>",
	Action: func(c *cli.Context) error {
		gConf, err := grantedConfig.Load()
		if err != nil {
			return err
		}

		name := c.String("name")
		gitURL := c.String("url")
		path := c.String("path")
		configFileName := c.String("filename")
		ref := c.String("ref")
		prefixAllProfiles := c.Bool("prefix-all-profiles")
		prefixDuplicateProfiles := c.Bool("prefix-duplicate-profiles")
		requiredKey := c.StringSlice("requiredKey")
		priority := c.Int("priority")

		if _, ok := gConf.ProfileRegistry.Registries[name]; ok {
			clio.Errorf("profile registry with name '%s' already exists. Name is required to be unique. Try adding with different name.\n", name)

			return nil
		}

		registry := NewProfileRegistry(registryOptions{
			name:                    name,
			path:                    path,
			configFileName:          configFileName,
			url:                     gitURL,
			ref:                     ref,
			priority:                priority,
			prefixAllProfiles:       prefixAllProfiles,
			prefixDuplicateProfiles: prefixDuplicateProfiles,
		})

		repoDirPath, err := registry.getRegistryLocation()
		if err != nil {
			return err
		}

		if _, err = os.Stat(repoDirPath); err != nil {
			err = gitClone(gitURL, repoDirPath)
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
			err = gitPull(repoDirPath, false)
			if err != nil {
				return err
			}
		}

		err = registry.Parse()
		if err != nil {
			return err
		}

		err = registry.PromptRequiredKeys(requiredKey)
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

		isFirstSection := false
		allRegistries, err := GetProfileRegistries()
		if err != nil {
			return err
		}

		if len(allRegistries) == 0 {
			isFirstSection = true
		}

		if err := Sync(&registry, isFirstSection, ADD_COMMAND); err != nil {
			return err
		}

		gConf, err = grantedConfig.Load()
		if err != nil {
			return err
		}

		// we have verified that this registry is a valid one and sync is completed.
		// so save the repo url to config file.
		if gConf.ProfileRegistry.Registries != nil {
			gConf.ProfileRegistry.Registries[name] = registry.Config
		} else {
			reg := make(map[string]grantedConfig.Registry)

			reg[name] = registry.Config
			gConf.ProfileRegistry.Registries = reg
		}

		err = gConf.Save()
		if err != nil {
			return err
		}

		return nil
	},
}
