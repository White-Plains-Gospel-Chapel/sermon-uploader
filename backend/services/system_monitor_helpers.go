package services

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// calculateCPUUsage reads /proc/stat and calculates CPU usage percentage
func (s *SystemResourceMonitor) calculateCPUUsage() float64 {
	file, err := os.Open("/proc/stat")
	if err != nil {
		s.logger.Warn("Failed to read /proc/stat", "error", err)
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0
	}

	// Parse first line (overall CPU stats)
	fields := strings.Fields(scanner.Text())
	if len(fields) < 8 || fields[0] != "cpu" {
		return 0
	}

	// Convert fields to integers
	var cpuTimes []int64
	for i := 1; i < len(fields) && i < 8; i++ {
		val, err := strconv.ParseInt(fields[i], 10, 64)
		if err != nil {
			return 0
		}
		cpuTimes = append(cpuTimes, val)
	}

	// Calculate total and idle time
	var totalTime, idleTime int64
	for i, time := range cpuTimes {
		totalTime += time
		if i == 3 { // idle time is the 4th field
			idleTime = time
		}
	}

	if totalTime == 0 {
		return 0
	}

	// Calculate usage percentage
	usage := float64(totalTime-idleTime) / float64(totalTime) * 100
	return usage
}

// readLoadAverages reads system load averages from /proc/loadavg
func (s *SystemResourceMonitor) readLoadAverages() (float64, float64, float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		s.logger.Warn("Failed to read /proc/loadavg", "error", err)
		return 0, 0, 0
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0
	}

	load1, _ := strconv.ParseFloat(fields[0], 64)
	load5, _ := strconv.ParseFloat(fields[1], 64)
	load15, _ := strconv.ParseFloat(fields[2], 64)

	return load1, load5, load15
}

// getCPUFrequency reads current CPU frequency
func (s *SystemResourceMonitor) getCPUFrequency() int64 {
	data, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq")
	if err != nil {
		// Fallback to cpuinfo_cur_freq
		data, err = os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_cur_freq")
		if err != nil {
			return 0
		}
	}

	freq, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}

	return freq / 1000 // Convert from kHz to MHz
}

// readMemInfo reads memory information from /proc/meminfo
func (s *SystemResourceMonitor) readMemInfo() map[string]float64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		s.logger.Warn("Failed to read /proc/meminfo", "error", err)
		return make(map[string]float64)
	}
	defer file.Close()

	memInfo := make(map[string]float64)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}

		memInfo[key] = value
	}

	return memInfo
}

// readTemperature reads temperature from thermal zone file
func (s *SystemResourceMonitor) readTemperature(path string) float64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	temp, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return 0
	}

	return temp / 1000 // Convert from millicelsius to celsius
}

// checkThrottling checks if the CPU is being thermally throttled
func (s *SystemResourceMonitor) checkThrottling() bool {
	// Check Pi-specific throttling status
	data, err := os.ReadFile("/sys/devices/platform/soc/soc:firmware/get_throttled")
	if err != nil {
		// Alternative method using vcgencmd
		cmd := exec.Command("vcgencmd", "get_throttled")
		output, err := cmd.Output()
		if err != nil {
			return false
		}

		// Parse vcgencmd output (throttled=0x0 means no throttling)
		if strings.Contains(string(output), "throttled=0x0") {
			return false
		}
		return true
	}

	throttled, err := strconv.ParseInt(strings.TrimSpace(string(data)), 0, 64)
	if err != nil {
		return false
	}

	// Bit 0: under-voltage detected
	// Bit 1: arm frequency capped
	// Bit 2: currently throttled
	return (throttled & 0x7) != 0
}

// readPowerValue reads power-related values from sysfs
func (s *SystemResourceMonitor) readPowerValue(path string) float64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}

	value, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return 0
	}

	return value
}

// getUSBPowerMode determines the USB power mode
func (s *SystemResourceMonitor) getUSBPowerMode() string {
	// Check USB power configuration
	data, err := os.ReadFile("/sys/devices/platform/soc/3f980000.usb/power_mode")
	if err != nil {
		return "unknown"
	}

	return strings.TrimSpace(string(data))
}

// getDiskUsage gets disk usage statistics for a path
func (s *SystemResourceMonitor) getDiskUsage(path string) map[string]float64 {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		s.logger.Warn("Failed to get disk usage", "path", path, "error", err)
		return make(map[string]float64)
	}

	// Calculate sizes in GB
	blockSize := float64(stat.Bsize)
	totalBlocks := float64(stat.Blocks)
	freeBlocks := float64(stat.Bavail)
	
	totalGB := (totalBlocks * blockSize) / (1024 * 1024 * 1024)
	freeGB := (freeBlocks * blockSize) / (1024 * 1024 * 1024)
	usedGB := totalGB - freeGB

	// Calculate inode usage
	totalInodes := float64(stat.Files)
	freeInodes := float64(stat.Ffree)
	inodeUsage := ((totalInodes - freeInodes) / totalInodes) * 100

	return map[string]float64{
		"total":       totalGB,
		"used":        usedGB,
		"free":        freeGB,
		"inode_usage": inodeUsage,
	}
}

// readNetworkStats reads network interface statistics
func (s *SystemResourceMonitor) readNetworkStats(interface_ string) map[string]interface{} {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		s.logger.Warn("Failed to read /proc/net/dev", "error", err)
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	
	// Skip header lines
	scanner.Scan()
	scanner.Scan()

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		
		if len(fields) < 17 {
			continue
		}

		// Extract interface name
		interfaceName := strings.TrimSuffix(fields[0], ":")
		if interfaceName != interface_ {
			continue
		}

		// Parse statistics
		bytesRx, _ := strconv.ParseUint(fields[1], 10, 64)
		packetsRx, _ := strconv.ParseUint(fields[2], 10, 64)
		errorsRx, _ := strconv.ParseUint(fields[3], 10, 64)
		dropRx, _ := strconv.ParseUint(fields[4], 10, 64)

		bytesTx, _ := strconv.ParseUint(fields[9], 10, 64)
		packetsTx, _ := strconv.ParseUint(fields[10], 10, 64)
		errorsTx, _ := strconv.ParseUint(fields[11], 10, 64)
		dropTx, _ := strconv.ParseUint(fields[12], 10, 64)

		// Check if interface is up
		isUp := s.isInterfaceUp(interface_)

		return map[string]interface{}{
			"interface":  interface_,
			"bytes_rx":   bytesRx,
			"bytes_tx":   bytesTx,
			"packets_rx": packetsRx,
			"packets_tx": packetsTx,
			"errors_rx":  errorsRx,
			"errors_tx":  errorsTx,
			"drop_rx":    dropRx,
			"drop_tx":    dropTx,
			"is_up":      isUp,
		}
	}

	return nil
}

// isInterfaceUp checks if a network interface is up
func (s *SystemResourceMonitor) isInterfaceUp(interface_ string) bool {
	flagsPath := fmt.Sprintf("/sys/class/net/%s/flags", interface_)
	data, err := os.ReadFile(flagsPath)
	if err != nil {
		return false
	}

	flags, err := strconv.ParseUint(strings.TrimSpace(string(data)), 0, 64)
	if err != nil {
		return false
	}

	// IFF_UP flag is bit 0
	return (flags & 0x1) != 0
}

// getProcessCount gets the number of running processes
func (s *SystemResourceMonitor) getProcessCount() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			// Check if directory name is numeric (PID)
			if _, err := strconv.Atoi(entry.Name()); err == nil {
				count++
			}
		}
	}

	return count
}

// getSystemUptime gets system uptime in hours
func (s *SystemResourceMonitor) getSystemUptime() float64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}

	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0
	}

	uptime, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}

	return uptime / 3600 // Convert seconds to hours
}

// getBootTime gets system boot time
func (s *SystemResourceMonitor) getBootTime() time.Time {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return time.Time{}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "btime ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				bootTime, err := strconv.ParseInt(fields[1], 10, 64)
				if err == nil {
					return time.Unix(bootTime, 0)
				}
			}
		}
	}

	return time.Time{}
}

// updateHistoricalData maintains historical metrics for trends
func (s *SystemResourceMonitor) updateHistoricalData() {
	// Add current values to history
	s.cpuHistory = append(s.cpuHistory, s.cpuUsage.UsagePercent)
	s.memoryHistory = append(s.memoryHistory, s.memoryUsage.UsagePercent)
	s.tempHistory = append(s.tempHistory, s.thermalMetrics.CPUTempC)

	// Keep only last 60 readings (1 hour at 1-minute intervals)
	if len(s.cpuHistory) > 60 {
		s.cpuHistory = s.cpuHistory[1:]
	}
	if len(s.memoryHistory) > 60 {
		s.memoryHistory = s.memoryHistory[1:]
	}
	if len(s.tempHistory) > 60 {
		s.tempHistory = s.tempHistory[1:]
	}
}

// calculateTrends calculates trend information for metrics
func (s *SystemResourceMonitor) calculateTrends() map[string]string {
	trends := make(map[string]string)

	// CPU trend
	if len(s.cpuHistory) >= 5 {
		recent := s.cpuHistory[len(s.cpuHistory)-5:]
		if recent[len(recent)-1] > recent[0]+10 {
			trends["cpu"] = "🔺 Increasing"
		} else if recent[len(recent)-1] < recent[0]-10 {
			trends["cpu"] = "🔻 Decreasing"  
		} else {
			trends["cpu"] = "➡️ Stable"
		}
	} else {
		trends["cpu"] = "📊 Collecting"
	}

	// Memory trend
	if len(s.memoryHistory) >= 5 {
		recent := s.memoryHistory[len(s.memoryHistory)-5:]
		if recent[len(recent)-1] > recent[0]+5 {
			trends["memory"] = "🔺 Increasing"
		} else if recent[len(recent)-1] < recent[0]-5 {
			trends["memory"] = "🔻 Decreasing"
		} else {
			trends["memory"] = "➡️ Stable"
		}
	} else {
		trends["memory"] = "📊 Collecting"
	}

	// Temperature trend
	if len(s.tempHistory) >= 5 {
		recent := s.tempHistory[len(s.tempHistory)-5:]
		if recent[len(recent)-1] > recent[0]+3 {
			trends["temperature"] = "🔺 Rising"
		} else if recent[len(recent)-1] < recent[0]-3 {
			trends["temperature"] = "🔻 Cooling"
		} else {
			trends["temperature"] = "➡️ Stable"
		}
	} else {
		trends["temperature"] = "📊 Collecting"
	}

	return trends
}