package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// PiPerformanceMonitor monitors system performance on Raspberry Pi
type PiPerformanceMonitor struct {
	mu                sync.RWMutex
	metrics           *PiMetrics
	alerts            []Alert
	thresholds        PiThresholds
	startTime         time.Time
	lastCollection    time.Time
	collectionInterval time.Duration
	alerting          bool
	webhookURL        string
}

// PiMetrics represents comprehensive Pi system metrics
type PiMetrics struct {
	Timestamp        time.Time     `json:"timestamp"`
	Uptime          time.Duration `json:"uptime"`
	
	// System Info
	System          SystemInfo    `json:"system"`
	
	// CPU Metrics
	CPU             CPUMetrics    `json:"cpu"`
	
	// Memory Metrics
	Memory          MemoryMetrics `json:"memory"`
	
	// Disk Metrics
	Disk            DiskMetrics   `json:"disk"`
	
	// Network Metrics
	Network         NetworkMetrics `json:"network"`
	
	// Go Runtime Metrics
	GoRuntime       GoRuntimeMetrics `json:"go_runtime"`
	
	// Application Metrics
	Application     ApplicationMetrics `json:"application"`
	
	// Pi-Specific Metrics
	PiSpecific      PiSpecificMetrics `json:"pi_specific"`
}

type SystemInfo struct {
	Hostname        string    `json:"hostname"`
	Platform        string    `json:"platform"`
	Architecture    string    `json:"architecture"`
	KernelVersion   string    `json:"kernel_version"`
	Temperature     float64   `json:"temperature_celsius"`
}

type CPUMetrics struct {
	UsagePercent    float64   `json:"usage_percent"`
	LoadAverage     []float64 `json:"load_average"`
	CoreCount       int       `json:"core_count"`
	CoreUsage       []float64 `json:"core_usage"`
	ThrottleCount   int64     `json:"throttle_count"`
	Frequency       float64   `json:"frequency_mhz"`
}

type MemoryMetrics struct {
	TotalMB         uint64    `json:"total_mb"`
	AvailableMB     uint64    `json:"available_mb"`
	UsedMB          uint64    `json:"used_mb"`
	UsagePercent    float64   `json:"usage_percent"`
	SwapTotalMB     uint64    `json:"swap_total_mb"`
	SwapUsedMB      uint64    `json:"swap_used_mb"`
	SwapPercent     float64   `json:"swap_percent"`
	BuffersCacheMB  uint64    `json:"buffers_cache_mb"`
}

type DiskMetrics struct {
	TotalGB         uint64    `json:"total_gb"`
	UsedGB          uint64    `json:"used_gb"`
	FreeGB          uint64    `json:"free_gb"`
	UsagePercent    float64   `json:"usage_percent"`
	IOReadMBps      float64   `json:"io_read_mbps"`
	IOWriteMBps     float64   `json:"io_write_mbps"`
	IOUtilPercent   float64   `json:"io_util_percent"`
}

type NetworkMetrics struct {
	BytesReceivedMB uint64    `json:"bytes_received_mb"`
	BytesSentMB     uint64    `json:"bytes_sent_mb"`
	PacketsReceived uint64    `json:"packets_received"`
	PacketsSent     uint64    `json:"packets_sent"`
	Errors          uint64    `json:"errors"`
	Drops           uint64    `json:"drops"`
}

type GoRuntimeMetrics struct {
	NumGoroutine    int       `json:"num_goroutine"`
	NumCPU          int       `json:"num_cpu"`
	NumCGOCall      int64     `json:"num_cgo_call"`
	MemoryAlloc     uint64    `json:"memory_alloc_mb"`
	MemorySys       uint64    `json:"memory_sys_mb"`
	MemoryHeap      uint64    `json:"memory_heap_mb"`
	MemoryStack     uint64    `json:"memory_stack_mb"`
	GCCount         uint32    `json:"gc_count"`
	GCPauseMs       uint64    `json:"gc_pause_ms"`
	GCTargetPercent int       `json:"gc_target_percent"`
	NextGCMB        uint64    `json:"next_gc_mb"`
}

type ApplicationMetrics struct {
	ProcessID       int32     `json:"process_id"`
	CPUPercent      float64   `json:"cpu_percent"`
	MemoryMB        uint64    `json:"memory_mb"`
	MemoryPercent   float32   `json:"memory_percent"`
	FileDescriptors int32     `json:"file_descriptors"`
	Threads         int32     `json:"threads"`
	Connections     int       `json:"connections"`
	OpenFiles       int       `json:"open_files"`
}

type PiSpecificMetrics struct {
	ModelName       string    `json:"model_name"`
	SerialNumber    string    `json:"serial_number"`
	Revision        string    `json:"revision"`
	BootTime        time.Time `json:"boot_time"`
	ThermalState    string    `json:"thermal_state"`
	VoltageCore     float64   `json:"voltage_core"`
	VoltageSDRAM    float64   `json:"voltage_sdram"`
	ClockCore       int64     `json:"clock_core_hz"`
	ClockGPU        int64     `json:"clock_gpu_hz"`
	GPUMemoryMB     int       `json:"gpu_memory_mb"`
}

// PiThresholds defines alert thresholds for Pi hardware
type PiThresholds struct {
	CPUUsagePercent     float64   `json:"cpu_usage_percent"`
	MemoryUsagePercent  float64   `json:"memory_usage_percent"`
	DiskUsagePercent    float64   `json:"disk_usage_percent"`
	TemperatureCelsius  float64   `json:"temperature_celsius"`
	LoadAverage1Min     float64   `json:"load_average_1min"`
	GoroutineCount      int       `json:"goroutine_count"`
	HeapSizeMB          uint64    `json:"heap_size_mb"`
	GCPauseMs           uint64    `json:"gc_pause_ms"`
	FileDescriptors     int32     `json:"file_descriptors"`
	SwapUsagePercent    float64   `json:"swap_usage_percent"`
}

// Alert represents a performance alert
type Alert struct {
	Timestamp   time.Time `json:"timestamp"`
	Level       string    `json:"level"`     // INFO, WARNING, ERROR, CRITICAL
	Component   string    `json:"component"` // CPU, MEMORY, DISK, NETWORK, APP
	Metric      string    `json:"metric"`
	Value       float64   `json:"value"`
	Threshold   float64   `json:"threshold"`
	Message     string    `json:"message"`
	Resolved    bool      `json:"resolved"`
}

// NewPiPerformanceMonitor creates a new Pi performance monitor
func NewPiPerformanceMonitor(webhookURL string) *PiPerformanceMonitor {
	return &PiPerformanceMonitor{
		metrics: &PiMetrics{},
		alerts:  make([]Alert, 0),
		thresholds: PiThresholds{
			CPUUsagePercent:    80.0,
			MemoryUsagePercent: 85.0,
			DiskUsagePercent:   90.0,
			TemperatureCelsius: 75.0, // Pi thermal throttling starts around 80°C
			LoadAverage1Min:    4.0,  // For 4-core Pi
			GoroutineCount:     200,
			HeapSizeMB:         2048, // 2GB heap limit for Pi
			GCPauseMs:          50,   // 50ms GC pause threshold
			FileDescriptors:    800,  // Leave buffer from 1024 limit
			SwapUsagePercent:   50.0,
		},
		startTime:          time.Now(),
		collectionInterval: 30 * time.Second,
		alerting:          true,
		webhookURL:        webhookURL,
	}
}

// Start begins the performance monitoring
func (pm *PiPerformanceMonitor) Start(ctx context.Context) error {
	log.Println("Starting Pi Performance Monitor...")
	
	ticker := time.NewTicker(pm.collectionInterval)
	defer ticker.Stop()
	
	// Initial collection
	if err := pm.collectMetrics(); err != nil {
		return fmt.Errorf("initial metrics collection failed: %w", err)
	}
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Pi Performance Monitor shutting down...")
			return ctx.Err()
		case <-ticker.C:
			if err := pm.collectMetrics(); err != nil {
				log.Printf("Metrics collection error: %v", err)
			}
		}
	}
}

// collectMetrics gathers all system and application metrics
func (pm *PiPerformanceMonitor) collectMetrics() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	start := time.Now()
	metrics := &PiMetrics{
		Timestamp: start,
		Uptime:    time.Since(pm.startTime),
	}
	
	// Collect system info
	if err := pm.collectSystemInfo(&metrics.System); err != nil {
		log.Printf("System info collection error: %v", err)
	}
	
	// Collect CPU metrics
	if err := pm.collectCPUMetrics(&metrics.CPU); err != nil {
		log.Printf("CPU metrics collection error: %v", err)
	}
	
	// Collect memory metrics
	if err := pm.collectMemoryMetrics(&metrics.Memory); err != nil {
		log.Printf("Memory metrics collection error: %v", err)
	}
	
	// Collect disk metrics
	if err := pm.collectDiskMetrics(&metrics.Disk); err != nil {
		log.Printf("Disk metrics collection error: %v", err)
	}
	
	// Collect network metrics
	if err := pm.collectNetworkMetrics(&metrics.Network); err != nil {
		log.Printf("Network metrics collection error: %v", err)
	}
	
	// Collect Go runtime metrics
	pm.collectGoRuntimeMetrics(&metrics.GoRuntime)
	
	// Collect application metrics
	if err := pm.collectApplicationMetrics(&metrics.Application); err != nil {
		log.Printf("Application metrics collection error: %v", err)
	}
	
	// Collect Pi-specific metrics
	if err := pm.collectPiSpecificMetrics(&metrics.PiSpecific); err != nil {
		log.Printf("Pi-specific metrics collection error: %v", err)
	}
	
	// Store metrics
	pm.metrics = metrics
	pm.lastCollection = start
	
	// Check thresholds and generate alerts
	if pm.alerting {
		pm.checkThresholds(metrics)
	}
	
	log.Printf("Metrics collection completed in %v", time.Since(start))
	return nil
}

// collectSystemInfo gathers basic system information
func (pm *PiPerformanceMonitor) collectSystemInfo(info *SystemInfo) error {
	hostInfo, err := host.Info()
	if err != nil {
		return err
	}
	
	info.Hostname = hostInfo.Hostname
	info.Platform = hostInfo.Platform
	info.Architecture = hostInfo.KernelArch
	info.KernelVersion = hostInfo.KernelVersion
	
	// Try to read Pi temperature
	if temp, err := pm.readPiTemperature(); err == nil {
		info.Temperature = temp
	}
	
	return nil
}

// collectCPUMetrics gathers CPU performance data
func (pm *PiPerformanceMonitor) collectCPUMetrics(cpuMetrics *CPUMetrics) error {
	// CPU usage percentage
	percent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return err
	}
	if len(percent) > 0 {
		cpuMetrics.UsagePercent = percent[0]
	}
	
	// Per-core CPU usage
	corePercent, err := cpu.Percent(time.Second, true)
	if err == nil {
		cpuMetrics.CoreUsage = corePercent
		cpuMetrics.CoreCount = len(corePercent)
	}
	
	// Load average
	if avg, err := host.LoadAvg(); err == nil {
		cpuMetrics.LoadAverage = []float64{avg.Load1, avg.Load5, avg.Load15}
	}
	
	// CPU info for frequency
	if cpuInfo, err := cpu.Info(); err == nil && len(cpuInfo) > 0 {
		cpuMetrics.Frequency = cpuInfo[0].Mhz
	}
	
	return nil
}

// collectMemoryMetrics gathers memory usage data
func (pm *PiPerformanceMonitor) collectMemoryMetrics(memMetrics *MemoryMetrics) error {
	vmem, err := mem.VirtualMemory()
	if err != nil {
		return err
	}
	
	memMetrics.TotalMB = vmem.Total / 1024 / 1024
	memMetrics.AvailableMB = vmem.Available / 1024 / 1024
	memMetrics.UsedMB = vmem.Used / 1024 / 1024
	memMetrics.UsagePercent = vmem.UsedPercent
	memMetrics.BuffersCacheMB = (vmem.Buffers + vmem.Cached) / 1024 / 1024
	
	// Swap information
	if swap, err := mem.SwapMemory(); err == nil {
		memMetrics.SwapTotalMB = swap.Total / 1024 / 1024
		memMetrics.SwapUsedMB = swap.Used / 1024 / 1024
		memMetrics.SwapPercent = swap.UsedPercent
	}
	
	return nil
}

// collectDiskMetrics gathers disk usage and I/O data
func (pm *PiPerformanceMonitor) collectDiskMetrics(diskMetrics *DiskMetrics) error {
	usage, err := disk.Usage("/")
	if err != nil {
		return err
	}
	
	diskMetrics.TotalGB = usage.Total / 1024 / 1024 / 1024
	diskMetrics.UsedGB = usage.Used / 1024 / 1024 / 1024
	diskMetrics.FreeGB = usage.Free / 1024 / 1024 / 1024
	diskMetrics.UsagePercent = usage.UsedPercent
	
	return nil
}

// collectNetworkMetrics gathers network statistics
func (pm *PiPerformanceMonitor) collectNetworkMetrics(netMetrics *NetworkMetrics) error {
	stats, err := net.IOCounters(false)
	if err != nil || len(stats) == 0 {
		return err
	}
	
	netMetrics.BytesReceivedMB = stats[0].BytesRecv / 1024 / 1024
	netMetrics.BytesSentMB = stats[0].BytesSent / 1024 / 1024
	netMetrics.PacketsReceived = stats[0].PacketsRecv
	netMetrics.PacketsSent = stats[0].PacketsSent
	netMetrics.Errors = stats[0].Errin + stats[0].Errout
	netMetrics.Drops = stats[0].Dropin + stats[0].Dropout
	
	return nil
}

// collectGoRuntimeMetrics gathers Go runtime statistics
func (pm *PiPerformanceMonitor) collectGoRuntimeMetrics(goMetrics *GoRuntimeMetrics) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	goMetrics.NumGoroutine = runtime.NumGoroutine()
	goMetrics.NumCPU = runtime.NumCPU()
	goMetrics.NumCGOCall = runtime.NumCgoCall()
	goMetrics.MemoryAlloc = m.Alloc / 1024 / 1024
	goMetrics.MemorySys = m.Sys / 1024 / 1024
	goMetrics.MemoryHeap = m.HeapAlloc / 1024 / 1024
	goMetrics.MemoryStack = m.StackInuse / 1024 / 1024
	goMetrics.GCCount = m.NumGC
	goMetrics.GCPauseMs = m.PauseNs[(m.NumGC+255)%256] / 1000000
	goMetrics.GCTargetPercent = int(debug.SetGCPercent(-1))
	debug.SetGCPercent(goMetrics.GCTargetPercent) // Restore original
	goMetrics.NextGCMB = m.NextGC / 1024 / 1024
}

// collectApplicationMetrics gathers application-specific metrics
func (pm *PiPerformanceMonitor) collectApplicationMetrics(appMetrics *ApplicationMetrics) error {
	pid := int32(os.Getpid())
	proc, err := process.NewProcess(pid)
	if err != nil {
		return err
	}
	
	appMetrics.ProcessID = pid
	
	if cpu, err := proc.CPUPercent(); err == nil {
		appMetrics.CPUPercent = cpu
	}
	
	if memInfo, err := proc.MemoryInfo(); err == nil {
		appMetrics.MemoryMB = memInfo.RSS / 1024 / 1024
	}
	
	if memPercent, err := proc.MemoryPercent(); err == nil {
		appMetrics.MemoryPercent = memPercent
	}
	
	if numFDs, err := proc.NumFDs(); err == nil {
		appMetrics.FileDescriptors = numFDs
	}
	
	if numThreads, err := proc.NumThreads(); err == nil {
		appMetrics.Threads = numThreads
	}
	
	if connections, err := proc.Connections(); err == nil {
		appMetrics.Connections = len(connections)
	}
	
	if openFiles, err := proc.OpenFiles(); err == nil {
		appMetrics.OpenFiles = len(openFiles)
	}
	
	return nil
}

// collectPiSpecificMetrics gathers Raspberry Pi specific data
func (pm *PiPerformanceMonitor) collectPiSpecificMetrics(piMetrics *PiSpecificMetrics) error {
	// Read Pi model information
	if model, err := pm.readPiModel(); err == nil {
		piMetrics.ModelName = model
	}
	
	// Read Pi serial number
	if serial, err := pm.readPiSerial(); err == nil {
		piMetrics.SerialNumber = serial
	}
	
	// Read Pi revision
	if revision, err := pm.readPiRevision(); err == nil {
		piMetrics.Revision = revision
	}
	
	// System boot time
	if bootTime, err := host.BootTime(); err == nil {
		piMetrics.BootTime = time.Unix(int64(bootTime), 0)
	}
	
	// Thermal state
	piMetrics.ThermalState = pm.getThermalState()
	
	return nil
}

// readPiTemperature reads CPU temperature from Pi
func (pm *PiPerformanceMonitor) readPiTemperature() (float64, error) {
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return 0, err
	}
	
	var temp int64
	if _, err := fmt.Sscanf(string(data), "%d", &temp); err != nil {
		return 0, err
	}
	
	return float64(temp) / 1000.0, nil
}

// readPiModel reads Pi model information
func (pm *PiPerformanceMonitor) readPiModel() (string, error) {
	data, err := os.ReadFile("/proc/device-tree/model")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// readPiSerial reads Pi serial number
func (pm *PiPerformanceMonitor) readPiSerial() (string, error) {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "", err
	}
	
	// Parse serial from cpuinfo
	lines := string(data)
	if idx := fmt.Sprintf("%s", lines); idx != "" {
		// Simplified serial extraction - would need proper parsing
		return "pi_serial", nil
	}
	return "", fmt.Errorf("serial not found")
}

// readPiRevision reads Pi hardware revision
func (pm *PiPerformanceMonitor) readPiRevision() (string, error) {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "", err
	}
	
	// Parse revision from cpuinfo
	lines := string(data)
	if idx := fmt.Sprintf("%s", lines); idx != "" {
		// Simplified revision extraction - would need proper parsing
		return "pi_revision", nil
	}
	return "", fmt.Errorf("revision not found")
}

// getThermalState determines current thermal throttling state
func (pm *PiPerformanceMonitor) getThermalState() string {
	temp, err := pm.readPiTemperature()
	if err != nil {
		return "unknown"
	}
	
	switch {
	case temp < 60:
		return "normal"
	case temp < 70:
		return "warm"
	case temp < 80:
		return "hot"
	default:
		return "critical"
	}
}

// checkThresholds evaluates metrics against thresholds and generates alerts
func (pm *PiPerformanceMonitor) checkThresholds(metrics *PiMetrics) {
	// CPU usage check
	if metrics.CPU.UsagePercent > pm.thresholds.CPUUsagePercent {
		pm.addAlert("WARNING", "CPU", "usage_percent", metrics.CPU.UsagePercent, 
			pm.thresholds.CPUUsagePercent, "High CPU usage detected")
	}
	
	// Memory usage check
	if metrics.Memory.UsagePercent > pm.thresholds.MemoryUsagePercent {
		pm.addAlert("WARNING", "MEMORY", "usage_percent", metrics.Memory.UsagePercent,
			pm.thresholds.MemoryUsagePercent, "High memory usage detected")
	}
	
	// Disk usage check
	if metrics.Disk.UsagePercent > pm.thresholds.DiskUsagePercent {
		pm.addAlert("WARNING", "DISK", "usage_percent", metrics.Disk.UsagePercent,
			pm.thresholds.DiskUsagePercent, "High disk usage detected")
	}
	
	// Temperature check
	if metrics.System.Temperature > pm.thresholds.TemperatureCelsius {
		level := "WARNING"
		if metrics.System.Temperature > 80 {
			level = "CRITICAL"
		}
		pm.addAlert(level, "SYSTEM", "temperature", metrics.System.Temperature,
			pm.thresholds.TemperatureCelsius, "High CPU temperature detected")
	}
	
	// Load average check
	if len(metrics.CPU.LoadAverage) > 0 && metrics.CPU.LoadAverage[0] > pm.thresholds.LoadAverage1Min {
		pm.addAlert("WARNING", "CPU", "load_average", metrics.CPU.LoadAverage[0],
			pm.thresholds.LoadAverage1Min, "High system load detected")
	}
	
	// Goroutine count check
	if metrics.GoRuntime.NumGoroutine > pm.thresholds.GoroutineCount {
		pm.addAlert("WARNING", "APP", "goroutine_count", float64(metrics.GoRuntime.NumGoroutine),
			float64(pm.thresholds.GoroutineCount), "High goroutine count detected")
	}
	
	// Heap size check
	if metrics.GoRuntime.MemoryHeap > pm.thresholds.HeapSizeMB {
		pm.addAlert("WARNING", "APP", "heap_size", float64(metrics.GoRuntime.MemoryHeap),
			float64(pm.thresholds.HeapSizeMB), "Large heap size detected")
	}
	
	// GC pause check
	if metrics.GoRuntime.GCPauseMs > pm.thresholds.GCPauseMs {
		pm.addAlert("WARNING", "APP", "gc_pause", float64(metrics.GoRuntime.GCPauseMs),
			float64(pm.thresholds.GCPauseMs), "Long GC pause detected")
	}
	
	// File descriptor check
	if metrics.Application.FileDescriptors > pm.thresholds.FileDescriptors {
		pm.addAlert("WARNING", "APP", "file_descriptors", float64(metrics.Application.FileDescriptors),
			float64(pm.thresholds.FileDescriptors), "High file descriptor usage detected")
	}
	
	// Swap usage check
	if metrics.Memory.SwapPercent > pm.thresholds.SwapUsagePercent {
		pm.addAlert("WARNING", "MEMORY", "swap_usage", metrics.Memory.SwapPercent,
			pm.thresholds.SwapUsagePercent, "High swap usage detected")
	}
}

// addAlert creates a new alert
func (pm *PiPerformanceMonitor) addAlert(level, component, metric string, value, threshold float64, message string) {
	alert := Alert{
		Timestamp: time.Now(),
		Level:     level,
		Component: component,
		Metric:    metric,
		Value:     value,
		Threshold: threshold,
		Message:   message,
		Resolved:  false,
	}
	
	pm.alerts = append(pm.alerts, alert)
	
	// Log alert
	log.Printf("ALERT [%s] %s: %s (%.2f > %.2f)", level, component, message, value, threshold)
	
	// Send webhook notification if configured
	if pm.webhookURL != "" {
		go pm.sendWebhookAlert(alert)
	}
}

// sendWebhookAlert sends alert to webhook URL
func (pm *PiPerformanceMonitor) sendWebhookAlert(alert Alert) {
	payload := map[string]interface{}{
		"alert":     alert,
		"metrics":   pm.GetCurrentMetrics(),
		"timestamp": time.Now(),
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal alert payload: %v", err)
		return
	}
	
	resp, err := http.Post(pm.webhookURL, "application/json", 
		fmt.Sprintf("%s", jsonData))
	if err != nil {
		log.Printf("Failed to send webhook alert: %v", err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		log.Printf("Webhook returned status %d", resp.StatusCode)
	}
}

// GetCurrentMetrics returns the latest metrics (thread-safe)
func (pm *PiPerformanceMonitor) GetCurrentMetrics() *PiMetrics {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	metricsCopy := *pm.metrics
	return &metricsCopy
}

// GetRecentAlerts returns recent alerts
func (pm *PiPerformanceMonitor) GetRecentAlerts(since time.Duration) []Alert {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	cutoff := time.Now().Add(-since)
	var recent []Alert
	
	for _, alert := range pm.alerts {
		if alert.Timestamp.After(cutoff) {
			recent = append(recent, alert)
		}
	}
	
	return recent
}

// GetSummary returns a performance summary
func (pm *PiPerformanceMonitor) GetSummary() map[string]interface{} {
	metrics := pm.GetCurrentMetrics()
	recentAlerts := pm.GetRecentAlerts(24 * time.Hour)
	
	summary := map[string]interface{}{
		"status":           "healthy",
		"uptime_hours":     metrics.Uptime.Hours(),
		"cpu_usage":        metrics.CPU.UsagePercent,
		"memory_usage":     metrics.Memory.UsagePercent,
		"disk_usage":       metrics.Disk.UsagePercent,
		"temperature":      metrics.System.Temperature,
		"goroutines":       metrics.GoRuntime.NumGoroutine,
		"heap_size_mb":     metrics.GoRuntime.MemoryHeap,
		"gc_pause_ms":      metrics.GoRuntime.GCPauseMs,
		"alerts_24h":       len(recentAlerts),
		"last_collection":  pm.lastCollection,
	}
	
	// Determine overall status
	if metrics.System.Temperature > 80 {
		summary["status"] = "critical"
	} else if len(recentAlerts) > 10 || metrics.CPU.UsagePercent > 90 || metrics.Memory.UsagePercent > 90 {
		summary["status"] = "warning"
	}
	
	return summary
}

// SetThresholds updates alert thresholds
func (pm *PiPerformanceMonitor) SetThresholds(thresholds PiThresholds) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.thresholds = thresholds
}

// EnableAlerting enables or disables alerting
func (pm *PiPerformanceMonitor) EnableAlerting(enable bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.alerting = enable
}