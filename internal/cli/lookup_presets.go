package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type lookupPresets struct {
	Subdomains []string `json:"subdomains"`
	Endings    []string `json:"endings"`
}

func loadLookupPresets() (lookupPresets, error) {
	path, err := configFile("lookup-presets.json")
	if err != nil {
		return lookupPresets{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return lookupPresets{}, nil
		}
		return lookupPresets{}, err
	}
	var presets lookupPresets
	if err := json.Unmarshal(data, &presets); err != nil {
		return lookupPresets{}, err
	}
	presets.Subdomains = normalizePresetSubdomains(presets.Subdomains)
	presets.Endings = normalizePresetEndings(presets.Endings)
	return presets, nil
}

func saveLookupPresets(presets lookupPresets) error {
	path, err := configFile("lookup-presets.json")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	presets.Subdomains = normalizePresetSubdomains(presets.Subdomains)
	presets.Endings = normalizePresetEndings(presets.Endings)
	data, err := json.MarshalIndent(presets, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func (a *App) manageLookupPresets() error {
	for {
		presets, err := loadLookupPresets()
		if err != nil {
			return err
		}
		options := []string{
			fmt.Sprintf("Add subdomains: %d saved", len(presets.Subdomains)),
			"Remove subdomain: Delete one saved name",
			"Clear subdomains: Delete all saved names",
			fmt.Sprintf("Add endings: %d saved", len(presets.Endings)),
			"Remove ending: Delete one saved ending",
			"Clear endings: Delete all saved endings",
			"Back",
		}
		index, err := selectOption("Lookup presets", options)
		if err != nil {
			return err
		}
		switch index {
		case 0:
			value, err := promptInput("Add subdomains", "Comma or space separated, empty entry means root domain", "")
			if err != nil {
				return err
			}
			presets.Subdomains = mergeUniqueStrings(presets.Subdomains, normalizePresetSubdomains(splitListAllowEmpty(value)))
		case 1:
			updated, err := removePresetEntry("Remove subdomain", presets.Subdomains)
			if err != nil {
				return err
			}
			presets.Subdomains = updated
		case 2:
			if ok, err := askConfirm("Clear subdomain presets?"); err != nil {
				return err
			} else if ok {
				presets.Subdomains = nil
			}
		case 3:
			value, err := promptInput("Add endings", "Comma or space separated, e.g. com net de", "")
			if err != nil {
				return err
			}
			presets.Endings = mergeUniqueStrings(presets.Endings, normalizePresetEndings(splitList(value)))
		case 4:
			updated, err := removePresetEntry("Remove ending", presets.Endings)
			if err != nil {
				return err
			}
			presets.Endings = updated
		case 5:
			if ok, err := askConfirm("Clear ending presets?"); err != nil {
				return err
			} else if ok {
				presets.Endings = nil
			}
		default:
			return nil
		}
		if err := saveLookupPresets(presets); err != nil {
			return err
		}
	}
}

func removePresetEntry(title string, values []string) ([]string, error) {
	if len(values) == 0 {
		renderTextPage(title, "No saved entries.")
		_ = waitForEnter()
		return values, nil
	}
	options := append([]string(nil), values...)
	options = append(options, "Back")
	index, err := selectOption(title, options)
	if err != nil {
		return values, err
	}
	if index == len(options)-1 {
		return values, nil
	}
	return append(values[:index], values[index+1:]...), nil
}

func normalizePresetSubdomains(values []string) []string {
	list := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "." || value == "-" {
			value = ""
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		list = append(list, value)
	}
	return list
}

func normalizePresetEndings(values []string) []string {
	list := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = normalizeEnding(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		list = append(list, value)
	}
	return list
}

func splitListAllowEmpty(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{""}
	}
	return splitList(value)
}

func mergeUniqueStrings(primary, secondary []string) []string {
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	list := make([]string, 0, len(primary)+len(secondary))
	for _, value := range append(primary, secondary...) {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		list = append(list, value)
	}
	return list
}

func askConfirm(title string) (bool, error) {
	index, err := selectOption(title, []string{
		"No: Go back",
		"Yes: Continue",
	})
	if err != nil {
		return false, err
	}
	return index == 1, nil
}
