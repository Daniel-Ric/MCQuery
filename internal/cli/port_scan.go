package cli

import (
	"fmt"
	"time"

	"UWP-TCP-Con/internal/ping"
)

var commonBedrockPorts = []int{19132, 19133, 19134, 19135, 19136, 19137, 19138, 19139, 19140}
var commonJavaPorts = []int{25565, 25566, 25567, 25568, 25569, 25570, 25575}

func (a *App) executePortScan() error {
	host, err := a.askHost()
	if err != nil {
		return err
	}
	entries, err := a.collectPortScanTargets(host)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return renderTextPageAndWait("Port scan", "No ports selected.")
	}

	var runResults []batchRunResult
	progress := &batchProgress{total: len(entries)}
	resultText, err := withControlledSpinner("Port scan", func(frame int, control *spinnerControl) string {
		return fmt.Sprintf("Host: %s\n%s", host, progress.Render(frame, control))
	}, 120*time.Millisecond, func(control *spinnerControl) (string, error) {
		runResults = a.runBatchEntries(control, entries, progress)
		return formatBatchResults("Port scan", runResults, nil, control.IsCancelled()), nil
	})
	if err != nil {
		return err
	}

	exportText := formatBatchResults("Port scan", runResults, nil, false)
	records := batchExportRecords("port_scan", runResults)
	if a.settings.SaveResults {
		path, err := a.saveExport("Port scan", exportText, records)
		if err != nil {
			resultText = appendWarningText(resultText, "Result export failed", err)
		} else {
			resultText += fmt.Sprintf("\nSaved result: %s", path)
		}
	}
	return renderTextPageAndWait("Port scan", resultText)
}

func (a *App) collectPortScanTargets(host string) ([]batchEntry, error) {
	options := []string{
		fmt.Sprintf("Bedrock common: %s", portListText(commonBedrockPorts)),
		fmt.Sprintf("Java common: %s", portListText(commonJavaPorts)),
		"Both common: Bedrock and Java defaults",
		"Custom ports: Enter your own list",
	}
	index, err := selectOption("Port profile", options)
	if err != nil {
		return nil, err
	}

	switch index {
	case 0:
		return buildPortScanEntries(host, ping.EditionBedrock, commonBedrockPorts), nil
	case 1:
		return buildPortScanEntries(host, ping.EditionJava, commonJavaPorts), nil
	case 2:
		entries := buildPortScanEntries(host, ping.EditionBedrock, commonBedrockPorts)
		entries = append(entries, buildPortScanEntries(host, ping.EditionJava, commonJavaPorts)...)
		return entries, nil
	default:
		edition, err := a.askEdition()
		if err != nil {
			return nil, err
		}
		var errMsg string
		for {
			value, err := promptInput("Ports", "Comma or space separated, e.g. 19132 19133 25565", errMsg)
			if err != nil {
				return nil, err
			}
			ports, err := parsePortList(value)
			if err != nil {
				errMsg = err.Error()
				continue
			}
			return buildPortScanEntries(host, edition, ports), nil
		}
	}
}

func buildPortScanEntries(host string, edition ping.Edition, ports []int) []batchEntry {
	entries := make([]batchEntry, 0, len(ports))
	for _, port := range ports {
		entries = append(entries, batchEntry{
			Name:    fmt.Sprintf("%s:%d", host, port),
			Edition: edition,
			Host:    host,
			Port:    port,
		})
	}
	return entries
}
