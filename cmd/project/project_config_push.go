package project

import (
	"encoding/json"
	"shopware-cli/shop"

	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var projectConfigPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Synchronizes your local config to the external shop",
	RunE: func(cmd *cobra.Command, _ []string) error {
		var cfg *shop.Config
		var err error

		autoApprove, _ := cmd.PersistentFlags().GetBool("auto-approve")

		if cfg, err = shop.ReadConfig(projectConfigPath); err != nil {
			return err
		}

		client, err := shop.NewShopClient(cmd.Context(), cfg, nil)
		if err != nil {
			return err
		}

		operation := &ConfigSyncOperation{
			Operations:     map[string]shop.SyncOperation{},
			SystemSettings: map[*string]map[string]interface{}{},
			ThemeSettings:  []ThemeSyncOperation{},
		}

		if cfg.Sync != nil {
			for _, applyer := range NewSyncApplyers() {
				if err := applyer.Push(cmd.Context(), client, cfg, operation); err != nil {
					return err
				}
			}
		}

		if !operation.HasChanges() {
			log.Infof("Configuration is up to date")
			return nil
		}

		if operation.Operations.HasChanges() {
			log.Println("Following entities will be written")

			for _, values := range operation.Operations {
				log.Printf("Action: %s, Entity: %s", values.Action, values.Entity)

				content, _ := json.Marshal(values.Payload)

				log.Printf("Payload: %s", string(content))
			}
		}

		if operation.SystemSettings.HasChanges() {
			log.Println("Following system_config changes will be applied")

			for key, values := range operation.SystemSettings {
				if len(values) == 0 {
					continue
				}

				var k string

				if key == nil {
					k = "default"
				} else {
					k = *key
				}

				log.Printf("Sales-Channel: %s", k)

				content, _ := json.Marshal(values)

				log.Printf("Payload: %s", string(content))
			}
		}

		if operation.ThemeSettings.HasChanges() {
			for _, themeOp := range operation.ThemeSettings {
				log.Printf("Updating theme: %s", themeOp.Name)

				content, _ := json.Marshal(themeOp.Settings)

				log.Printf("Payload: %s", string(content))
			}
		}

		if !autoApprove {
			p := promptui.Prompt{
				Label:     "You want to apply these changes to your Shop?",
				IsConfirm: true,
			}

			if _, err := p.Run(); err != nil {
				return err
			}
		}

		if err := client.Sync(cmd.Context(), operation.Operations); err != nil {
			return err
		}

		if operation.SystemSettings.HasChanges() {
			if err := client.UpdateSystemConfig(cmd.Context(), operation.SystemSettings.ToJson()); err != nil {
				return err
			}
		}

		if operation.ThemeSettings.HasChanges() {
			for _, themeOp := range operation.ThemeSettings {
				if err := client.SaveThemeConfiguration(cmd.Context(), themeOp.Id, shop.ThemeUpdateRequest{Config: themeOp.Settings}); err != nil {
					return err
				}
			}
		}

		log.Infof("Configuration has been applied to remote")

		return nil
	},
}

func init() {
	projectConfigCmd.AddCommand(projectConfigPushCmd)
	projectConfigPushCmd.PersistentFlags().Bool("auto-approve", false, "Skips the confirmation")
}
