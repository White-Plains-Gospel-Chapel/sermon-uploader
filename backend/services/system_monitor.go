package services

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"
)

// SystemResourceMonitor monitors only resources used by sermon-uploader service
type SystemResourceMonitor struct {
	mu                sync.RWMutex
	logger            *slog.Logger
	discordService    DiscordLiveInterface
	messageID         string
	interval          time.Duration
	running           bool
	ctx               context.Context
	cancel            context.CancelFunc
	
	// Current metrics - only what we actually use
	cpuUsage          CPUMetrics
	memoryUsage       MemoryMetrics
	thermalMetrics    ThermalMetrics
	networkMetrics    NetworkMetrics
	diskMetrics       DiskMetrics
	
	// Historical data for trends
	cpuHistory        []float64
	memoryHistory     []float64
	tempHistory       []float64
	
	lastUpdate        time.Time
	sessionStart      time.Time
}

// CPUMetrics holds CPU usage information
type CPUMetrics struct {
	UsagePercent    float64   `json:"usage_percent"`
	LoadAvg1        float64   `json:"load_avg_1"`
	LoadAvg5        float64   `json:"load_avg_5"`
	LoadAvg15       float64   `json:"load_avg_15"`
	CoreCount       int       `json:"core_count"`
	Frequency       int64     `json:"frequency_mhz"`
	GoRoutines      int       `json:"goroutines"`
	LastUpdate      time.Time `json:"last_update"`
}

// MemoryMetrics holds memory usage information
type MemoryMetrics struct {
	TotalMB         float64   `json:"total_mb"`
	UsedMB          float64   `json:"used_mb"`
	FreeMB          float64   `json:"free_mb"`
	UsagePercent    float64   `json:"usage_percent"`
	AvailableMB     float64   `json:"available_mb"`
	BuffersMB       float64   `json:"buffers_mb"`
	CachedMB        float64   `json:"cached_mb"`
	SwapTotalMB     float64   `json:"swap_total_mb"`
	SwapUsedMB      float64   `json:"swap_used_mb"`
	GoAllocMB       float64   `json:"go_alloc_mb"`
	GoSysMB         float64   `json:"go_sys_mb"`
	GoGCCycles      uint32    `json:"go_gc_cycles"`
	LastUpdate      time.Time `json:"last_update"`
}

// ThermalMetrics holds temperature and thermal throttling information
type ThermalMetrics struct {
	CPUTempC        float64   `json:"cpu_temp_c"`
	GPUTempC        float64   `json:"gpu_temp_c"`
	IsThrottling    bool      `json:"is_throttling"`
	ThrottleEvents  int       `json:"throttle_events"`
	ThermalZone     string    `json:"thermal_zone"`
	CriticalTemp    float64   `json:"critical_temp_c"`
	LastUpdate      time.Time `json:"last_update"`
}

// PowerMetrics holds power consumption information for Pi 5
type PowerMetrics struct {
	VoltageV        float64   `json:"voltage_v"`
	CurrentA        float64   `json:"current_a"`
	PowerW          float64   `json:"power_w"`
	UnderVoltage    bool      `json:"under_voltage"`
	PowerGood       bool      `json:"power_good"`
	UsbPowerMode    string    `json:"usb_power_mode"`
	LastUpdate      time.Time `json:"last_update"`
}

// DiskMetrics holds disk usage information
type DiskMetrics struct {
	TotalGB         float64   `json:"total_gb"`
	UsedGB          float64   `json:"used_gb"`
	FreeGB          float64   `json:"free_gb"`
	UsagePercent    float64   `json:"usage_percent"`
	InodeUsage      float64   `json:"inode_usage_percent"`
	ReadIOPS        float64   `json:"read_iops"`
	WriteIOPS       float64   `json:"write_iops"`
	LastUpdate      time.Time `json:"last_update"`
}

// NetworkMetrics holds network interface statistics
type NetworkMetrics struct {
	Interface       string    `json:"interface"`
	BytesRx         uint64    `json:"bytes_rx"`
	BytesTx         uint64    `json:"bytes_tx"`
	PacketsRx       uint64    `json:"packets_rx"`
	PacketsTx       uint64    `json:"packets_tx"`
	ErrorsRx        uint64    `json:"errors_rx"`
	ErrorsTx        uint64    `json:"errors_tx"`
	DropRx          uint64    `json:"drop_rx"`
	DropTx          uint64    `json:"drop_tx"`
	Speed           int       `json:"speed_mbps"`
	IsUp            bool      `json:"is_up"`
	LastUpdate      time.Time `json:"last_update"`
}

// SystemLoadMetrics holds system load information
type SystemLoadMetrics struct {
	ProcessCount    int       `json:"process_count"`
	ThreadCount     int       `json:"thread_count"`
	Uptime          float64   `json:"uptime_hours"`
	BootTime        time.Time `json:"boot_time"`
	LastUpdate      time.Time `json:"last_update"`
}

// NewSystemResourceMonitor creates a new comprehensive system monitor
func NewSystemResourceMonitor(logger *slog.Logger, discordService DiscordLiveInterface, interval time.Duration) *SystemResourceMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &SystemResourceMonitor{
		logger:         logger,
		discordService: discordService,
		interval:       interval,
		ctx:            ctx,
		cancel:         cancel,
		sessionStart:   time.Now(),
		cpuHistory:     make([]float64, 0, 60),     // Keep 1 hour at 1-minute intervals
		memoryHistory:  make([]float64, 0, 60),
		tempHistory:    make([]float64, 0, 60),
	}
}

// Start begins monitoring system resources
func (s *SystemResourceMonitor) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("system monitor already running")
	}
	s.running = true
	s.mu.Unlock()

	// Initialize Discord message
	if s.discordService != nil {
		if err := s.initializeDiscordMessage(); err != nil {
			s.logger.Warn("Failed to initialize Discord system monitoring message", 
				slog.String("error", err.Error()))
		}
	}

	// Start monitoring goroutine
	go s.monitorLoop()
	
	s.logger.Info("System resource monitoring started",
		slog.Duration("interval", s.interval))
	
	return nil
}

// Stop stops system monitoring
func (s *SystemResourceMonitor) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.running {
		return
	}
	
	s.running = false
	s.cancel()
	
	s.logger.Info("System resource monitoring stopped")
}

// monitorLoop is the main monitoring loop
func (s *SystemResourceMonitor) monitorLoop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.collectAllMetrics()
			s.updateDiscordMessage()
		}
	}
}

// collectAllMetrics gathers all system metrics
func (s *SystemResourceMonitor) collectAllMetrics() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	now := time.Now()
	
	// Collect all metrics
	s.cpuUsage = s.collectCPUMetrics()
	s.memoryUsage = s.collectMemoryMetrics()
	s.thermalMetrics = s.collectThermalMetrics()
	s.diskMetrics = s.collectDiskMetrics()
	s.networkMetrics = s.collectNetworkMetrics()
	
	// Update historical data
	s.updateHistoricalData()
	
	s.lastUpdate = now
}

// collectCPUMetrics gathers CPU usage information
func (s *SystemResourceMonitor) collectCPUMetrics() CPUMetrics {
	// Read /proc/stat for CPU usage
	cpuPercent := s.calculateCPUUsage()
	
	// Read load averages
	loadAvg1, loadAvg5, loadAvg15 := s.readLoadAverages()
	
	// Get CPU frequency
	frequency := s.getCPUFrequency()
	
	return CPUMetrics{
		UsagePercent: cpuPercent,
		LoadAvg1:     loadAvg1,
		LoadAvg5:     loadAvg5,
		LoadAvg15:    loadAvg15,
		CoreCount:    runtime.NumCPU(),
		Frequency:    frequency,
		GoRoutines:   runtime.NumGoroutine(),
		LastUpdate:   time.Now(),
	}
}

// collectMemoryMetrics gathers memory usage information
func (s *SystemResourceMonitor) collectMemoryMetrics() MemoryMetrics {
	// Read /proc/meminfo
	memInfo := s.readMemInfo()
	
	// Get Go memory stats
	var goStats runtime.MemStats
	runtime.ReadMemStats(&goStats)
	
	return MemoryMetrics{
		TotalMB:      memInfo["MemTotal"] / 1024,
		UsedMB:       (memInfo["MemTotal"] - memInfo["MemAvailable"]) / 1024,
		FreeMB:       memInfo["MemFree"] / 1024,
		UsagePercent: ((memInfo["MemTotal"] - memInfo["MemAvailable"]) / memInfo["MemTotal"]) * 100,
		AvailableMB:  memInfo["MemAvailable"] / 1024,
		BuffersMB:    memInfo["Buffers"] / 1024,
		CachedMB:     memInfo["Cached"] / 1024,
		SwapTotalMB:  memInfo["SwapTotal"] / 1024,
		SwapUsedMB:   (memInfo["SwapTotal"] - memInfo["SwapFree"]) / 1024,
		GoAllocMB:    float64(goStats.Alloc) / 1024 / 1024,
		GoSysMB:      float64(goStats.Sys) / 1024 / 1024,
		GoGCCycles:   goStats.NumGC,
		LastUpdate:   time.Now(),
	}
}

// collectThermalMetrics gathers temperature and thermal information
func (s *SystemResourceMonitor) collectThermalMetrics() ThermalMetrics {
	cpuTemp := s.readTemperature("/sys/class/thermal/thermal_zone0/temp")
	gpuTemp := s.readTemperature("/sys/class/thermal/thermal_zone1/temp")
	
	// Check throttling status
	isThrottling := s.checkThrottling()
	
	return ThermalMetrics{
		CPUTempC:     cpuTemp,
		GPUTempC:     gpuTemp,
		IsThrottling: isThrottling,
		ThermalZone:  "thermal_zone0",
		CriticalTemp: 85.0, // Pi 5 critical temp
		LastUpdate:   time.Now(),
	}
}

// collectPowerMetrics gathers power consumption information
func (s *SystemResourceMonitor) collectPowerMetrics() PowerMetrics {
	// Pi 5 specific power monitoring
	voltage := s.readPowerValue("/sys/devices/platform/rpi-poe-power-supply/power_supply/rpi-poe/voltage_now")
	current := s.readPowerValue("/sys/devices/platform/rpi-poe-power-supply/power_supply/rpi-poe/current_now")
	
	// Calculate power (voltage * current)
	power := (voltage / 1000000) * (current / 1000000) // Convert from microvolts/microamps
	
	// Check under-voltage condition
	underVoltage := voltage < 4.8 // 4.8V threshold
	
	return PowerMetrics{
		VoltageV:     voltage / 1000000, // Convert from microvolts
		CurrentA:     current / 1000000, // Convert from microamps  
		PowerW:       power,
		UnderVoltage: underVoltage,
		PowerGood:    voltage > 4.8,
		UsbPowerMode: s.getUSBPowerMode(),
		LastUpdate:   time.Now(),
	}
}

// collectDiskMetrics gathers disk usage information
func (s *SystemResourceMonitor) collectDiskMetrics() DiskMetrics {
	// Use statvfs equivalent for Go
	usage := s.getDiskUsage("/")
	
	return DiskMetrics{
		TotalGB:      usage["total"],
		UsedGB:       usage["used"],
		FreeGB:       usage["free"],
		UsagePercent: (usage["used"] / usage["total"]) * 100,
		InodeUsage:   usage["inode_usage"],
		LastUpdate:   time.Now(),
	}
}

// collectNetworkMetrics gathers network interface statistics
func (s *SystemResourceMonitor) collectNetworkMetrics() NetworkMetrics {
	// Read primary network interface stats
	netStats := s.readNetworkStats("eth0") // Raspberry Pi ethernet
	if netStats == nil {
		netStats = s.readNetworkStats("wlan0") // Fallback to WiFi
	}
	
	if netStats == nil {
		return NetworkMetrics{LastUpdate: time.Now()}
	}
	
	return NetworkMetrics{
		Interface:  netStats["interface"].(string),
		BytesRx:    netStats["bytes_rx"].(uint64),
		BytesTx:    netStats["bytes_tx"].(uint64),
		PacketsRx:  netStats["packets_rx"].(uint64),
		PacketsTx:  netStats["packets_tx"].(uint64),
		ErrorsRx:   netStats["errors_rx"].(uint64),
		ErrorsTx:   netStats["errors_tx"].(uint64),
		IsUp:       netStats["is_up"].(bool),
		LastUpdate: time.Now(),
	}
}

// collectSystemLoadMetrics gathers system load information
func (s *SystemResourceMonitor) collectSystemLoadMetrics() SystemLoadMetrics {
	processCount := s.getProcessCount()
	uptime := s.getSystemUptime()
	bootTime := s.getBootTime()
	
	return SystemLoadMetrics{
		ProcessCount: processCount,
		Uptime:       uptime,
		BootTime:     bootTime,
		LastUpdate:   time.Now(),
	}
}

// Helper methods for data collection will be implemented...
// (This file will be quite long with all the Linux system reading functions)

// GetCurrentMetrics returns the current system metrics snapshot
func (s *SystemResourceMonitor) GetCurrentMetrics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return map[string]interface{}{
		"cpu":      s.cpuUsage,
		"memory":   s.memoryUsage,
		"thermal":  s.thermalMetrics,
		"disk":     s.diskMetrics,
		"network":  s.networkMetrics,
		"updated":  s.lastUpdate,
	}
}