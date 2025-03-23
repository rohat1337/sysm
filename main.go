package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
)

// Pagination variables
var (
	processesPerPage = 10
	currentPage      = 0
	totalProcesses   = 0
)

// Fetch CPU usage stats
func getCPUUsage() string {
	percent, err := cpu.Percent(0, true)
	if err != nil {
		return fmt.Sprintf("Error fetching CPU stats: %v", err)
	}
	cpuStats := "CPU Usage:\n"
	for i, p := range percent {
		cpuStats += fmt.Sprintf("Core %d: %.2f%%\n", i, p)
	}
	return cpuStats
}

// Fetch Memory usage stats
func getMemoryUsage() string {
	v, err := mem.VirtualMemory()
	if err != nil {
		return fmt.Sprintf("Error fetching memory stats: %v", err)
	}
	return fmt.Sprintf("Memory Usage: %.2f%%\nTotal: %v MB\nUsed: %v MB\nFree: %v MB",
		v.UsedPercent, v.Total/1024/1024, v.Used/1024/1024, v.Free/1024/1024)
}

// Fetch Disk usage stats
func getDiskUsage() string {
	d, err := disk.Usage("/")
	if err != nil {
		return fmt.Sprintf("Error fetching disk stats: %v", err)
	}
	return fmt.Sprintf("Disk Usage: %.2f%%\nTotal: %v GB\nUsed: %v GB\nFree: %v GB",
		d.UsedPercent, d.Total/1024/1024/1024, d.Used/1024/1024/1024, d.Free/1024/1024/1024)
}

// Fetch a paginated process list
func getProcessList() ([]string, int) {
	procs, err := process.Processes()
	if err != nil {
		return []string{fmt.Sprintf("Error fetching processes: %v", err)}, 0
	}

	totalProcesses = len(procs)
	start := currentPage * processesPerPage
	end := start + processesPerPage

	if start >= totalProcesses {
		currentPage = 0 // Reset to first page if out of bounds
		start = 0
		end = processesPerPage
	}

	if end > totalProcesses {
		end = totalProcesses
	}

	var processList []string
	for _, proc := range procs[start:end] {
		name, _ := proc.Name()
		cpuUsage, _ := proc.CPUPercent()
		memUsage, _ := proc.MemoryPercent()
		processList = append(processList, fmt.Sprintf("PID %d | %s | CPU: %.2f%% | Mem: %.2f%%", proc.Pid, name, cpuUsage, memUsage))
	}

	return processList, totalProcesses
}

func main() {
	// Create new tview app
	app := tview.NewApplication()

	// Create a flex layout
	flex := tview.NewFlex().SetDirection(tview.FlexColumn)

	// CPU, Memory, Disk stats
	statsTextView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText("Loading stats...")

	// Process List Table
	processTable := tview.NewTable().SetBorders(true)

	// Function to update stats
	updateStats := func() {
		cpuStats := getCPUUsage()
		memStats := getMemoryUsage()
		diskStats := getDiskUsage()

		statsTextView.SetText(fmt.Sprintf("[yellow]CPU:[white]\n%s[yellow]Memory:[white]\n%s\n[yellow]Disk:[white]\n%s",
			cpuStats, memStats, diskStats))
		app.QueueUpdateDraw(func() {})
	}

	// Function to update process list with pagination
	updateProcessTable := func() {
		processTable.Clear()
		processList, total := getProcessList()

		// Add header
		processTable.SetCell(0, 0, tview.NewTableCell("[yellow]PID").SetAlign(tview.AlignCenter))
		processTable.SetCell(0, 1, tview.NewTableCell("[yellow]Name").SetAlign(tview.AlignCenter))
		processTable.SetCell(0, 2, tview.NewTableCell("[yellow]CPU %").SetAlign(tview.AlignCenter))
		processTable.SetCell(0, 3, tview.NewTableCell("[yellow]Mem %").SetAlign(tview.AlignCenter))

		// Add processes
		for i, proc := range processList {
			// Split process string manually
			parts := strings.Split(proc, " | ")
			if len(parts) < 4 {
				continue // Skip invalid entries
			}

			processTable.SetCell(i+1, 0, tview.NewTableCell(parts[0]).SetAlign(tview.AlignCenter))
			processTable.SetCell(i+1, 1, tview.NewTableCell(parts[1]).SetAlign(tview.AlignLeft))
			processTable.SetCell(i+1, 2, tview.NewTableCell(parts[2]).SetAlign(tview.AlignCenter))
			processTable.SetCell(i+1, 3, tview.NewTableCell(parts[3]).SetAlign(tview.AlignCenter))
		}

		// Footer for pagination info
		paginationInfo := fmt.Sprintf("[yellow]Page %d/%d | %d Processes Total", currentPage+1, (total/processesPerPage)+1, total)
		processTable.SetCell(len(processList)+1, 0, tview.NewTableCell(paginationInfo).SetAlign(tview.AlignLeft).SetSelectable(false))

		app.QueueUpdateDraw(func() {})
	}

	// Set up auto-refresh for stats and process list
	go func() {
		for {
			updateStats()
			updateProcessTable()
			time.Sleep(1 * time.Second) // Refresh every 2 seconds
		}
	}()

	// Handle SIGTERM and SIGINT for graceful exit
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalChannel
		log.Println("Shutting down...")
		app.Stop()
		os.Exit(0)
	}()

	// Layout UI
	flex.AddItem(statsTextView, 0, 1, false) // System stats
	flex.AddItem(processTable, 0, 1, true)   // Process list
	// Keyboard input handling for pagination
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRight: // Next page
			if (currentPage+1)*processesPerPage < totalProcesses {
				currentPage++
			}
		case tcell.KeyLeft: // Previous page
			if currentPage > 0 {
				currentPage--
			}
		case tcell.KeyCtrlQ: // Quit application
			app.Stop()
			os.Exit(0)
		}
		return event
	})

	// Run the application
	if err := app.SetRoot(flex, true).Run(); err != nil {
		log.Fatalf("Error running app: %v", err)
	}
}
