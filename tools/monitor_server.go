package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// MonitorServer provides HTTP API and web interface for Pi monitoring
type MonitorServer struct {
	monitor  *PiPerformanceMonitor
	router   *mux.Router
	upgrader websocket.Upgrader
	clients  map[*websocket.Conn]bool
	port     int
}

// NewMonitorServer creates a new monitoring web server
func NewMonitorServer(monitor *PiPerformanceMonitor, port int) *MonitorServer {
	return &MonitorServer{
		monitor: monitor,
		router:  mux.NewRouter(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins in development
			},
		},
		clients: make(map[*websocket.Conn]bool),
		port:    port,
	}
}

// setupRoutes configures HTTP routes
func (ms *MonitorServer) setupRoutes() {
	// API routes
	api := ms.router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/metrics", ms.handleMetrics).Methods("GET")
	api.HandleFunc("/alerts", ms.handleAlerts).Methods("GET")
	api.HandleFunc("/summary", ms.handleSummary).Methods("GET")
	api.HandleFunc("/thresholds", ms.handleGetThresholds).Methods("GET")
	api.HandleFunc("/thresholds", ms.handleSetThresholds).Methods("POST")
	api.HandleFunc("/health", ms.handleHealth).Methods("GET")
	
	// WebSocket for real-time updates
	ms.router.HandleFunc("/ws", ms.handleWebSocket)
	
	// Web interface
	ms.router.HandleFunc("/", ms.handleIndex).Methods("GET")
	ms.router.HandleFunc("/dashboard", ms.handleDashboard).Methods("GET")
	
	// Static files (would typically be served by nginx in production)
	ms.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
}

// Start starts the monitoring web server
func (ms *MonitorServer) Start(ctx context.Context) error {
	ms.setupRoutes()
	
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", ms.port),
		Handler:      ms.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	// Start WebSocket broadcast routine
	go ms.broadcastMetrics(ctx)
	
	// Start server in goroutine
	go func() {
		log.Printf("Monitor server starting on port %d", ms.port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()
	
	// Wait for context cancellation
	<-ctx.Done()
	
	// Shutdown server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	return server.Shutdown(shutdownCtx)
}

// API Handlers

func (ms *MonitorServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := ms.monitor.GetCurrentMetrics()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (ms *MonitorServer) handleAlerts(w http.ResponseWriter, r *http.Request) {
	hoursParam := r.URL.Query().Get("hours")
	hours := 24 // Default to 24 hours
	
	if hoursParam != "" {
		if h, err := strconv.Atoi(hoursParam); err == nil {
			hours = h
		}
	}
	
	alerts := ms.monitor.GetRecentAlerts(time.Duration(hours) * time.Hour)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alerts)
}

func (ms *MonitorServer) handleSummary(w http.ResponseWriter, r *http.Request) {
	summary := ms.monitor.GetSummary()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

func (ms *MonitorServer) handleGetThresholds(w http.ResponseWriter, r *http.Request) {
	// Return current thresholds (would need to add getter method to monitor)
	thresholds := PiThresholds{
		CPUUsagePercent:    80.0,
		MemoryUsagePercent: 85.0,
		DiskUsagePercent:   90.0,
		TemperatureCelsius: 75.0,
		LoadAverage1Min:    4.0,
		GoroutineCount:     200,
		HeapSizeMB:         2048,
		GCPauseMs:          50,
		FileDescriptors:    800,
		SwapUsagePercent:   50.0,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(thresholds)
}

func (ms *MonitorServer) handleSetThresholds(w http.ResponseWriter, r *http.Request) {
	var thresholds PiThresholds
	if err := json.NewDecoder(r.Body).Decode(&thresholds); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	ms.monitor.SetThresholds(thresholds)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (ms *MonitorServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now(),
		"uptime":    time.Since(time.Now()).String(), // Would track actual uptime
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// WebSocket handler for real-time updates
func (ms *MonitorServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ms.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()
	
	ms.clients[conn] = true
	defer delete(ms.clients, conn)
	
	// Keep connection alive and handle client messages
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}
	}
}

// broadcastMetrics sends metrics to all WebSocket clients
func (ms *MonitorServer) broadcastMetrics(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // Broadcast every 5 seconds
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics := ms.monitor.GetCurrentMetrics()
			summary := ms.monitor.GetSummary()
			
			broadcast := map[string]interface{}{
				"type":    "metrics_update",
				"metrics": metrics,
				"summary": summary,
			}
			
			ms.broadcastToClients(broadcast)
		}
	}
}

// broadcastToClients sends data to all connected WebSocket clients
func (ms *MonitorServer) broadcastToClients(data interface{}) {
	for client := range ms.clients {
		err := client.WriteJSON(data)
		if err != nil {
			log.Printf("WebSocket write error: %v", err)
			client.Close()
			delete(ms.clients, client)
		}
	}
}

// Web Interface Handlers

func (ms *MonitorServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/dashboard", http.StatusPermanentRedirect)
}

func (ms *MonitorServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Pi Performance Monitor</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { text-align: center; margin-bottom: 30px; }
        .metrics-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
        .metric-card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .metric-title { font-weight: bold; margin-bottom: 10px; color: #333; }
        .metric-value { font-size: 24px; margin-bottom: 5px; }
        .metric-unit { color: #666; font-size: 14px; }
        .status-good { color: #4CAF50; }
        .status-warn { color: #FF9800; }
        .status-critical { color: #F44336; }
        .progress-bar { width: 100%; height: 20px; background: #eee; border-radius: 10px; overflow: hidden; margin: 10px 0; }
        .progress-fill { height: 100%; transition: width 0.3s ease; }
        .progress-good { background: #4CAF50; }
        .progress-warn { background: #FF9800; }
        .progress-critical { background: #F44336; }
        .alerts-section { margin-top: 30px; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .alert-item { padding: 10px; margin: 5px 0; border-left: 4px solid; border-radius: 4px; }
        .alert-warning { border-color: #FF9800; background: #FFF3E0; }
        .alert-critical { border-color: #F44336; background: #FFEBEE; }
        .timestamp { color: #666; font-size: 12px; }
        #status { font-size: 18px; padding: 10px 20px; border-radius: 5px; display: inline-block; margin-bottom: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🥧 Raspberry Pi Performance Monitor</h1>
            <div id="status">Loading...</div>
            <div class="timestamp">Last Updated: <span id="lastUpdate">-</span></div>
        </div>
        
        <div class="metrics-grid">
            <div class="metric-card">
                <div class="metric-title">CPU Usage</div>
                <div class="metric-value" id="cpuUsage">-</div>
                <div class="progress-bar">
                    <div class="progress-fill" id="cpuProgress"></div>
                </div>
                <div>Load Average: <span id="loadAvg">-</span></div>
                <div>Cores: <span id="cpuCores">-</span></div>
            </div>
            
            <div class="metric-card">
                <div class="metric-title">Memory Usage</div>
                <div class="metric-value" id="memoryUsage">-</div>
                <div class="progress-bar">
                    <div class="progress-fill" id="memoryProgress"></div>
                </div>
                <div>Used: <span id="memoryUsed">-</span> MB / <span id="memoryTotal">-</span> MB</div>
                <div>Swap: <span id="swapUsage">-</span>%</div>
            </div>
            
            <div class="metric-card">
                <div class="metric-title">Disk Usage</div>
                <div class="metric-value" id="diskUsage">-</div>
                <div class="progress-bar">
                    <div class="progress-fill" id="diskProgress"></div>
                </div>
                <div>Used: <span id="diskUsed">-</span> GB / <span id="diskTotal">-</span> GB</div>
                <div>Free: <span id="diskFree">-</span> GB</div>
            </div>
            
            <div class="metric-card">
                <div class="metric-title">System Temperature</div>
                <div class="metric-value" id="temperature">-</div>
                <div class="progress-bar">
                    <div class="progress-fill" id="tempProgress"></div>
                </div>
                <div>Thermal State: <span id="thermalState">-</span></div>
            </div>
            
            <div class="metric-card">
                <div class="metric-title">Go Runtime</div>
                <div>Goroutines: <span class="metric-value" id="goroutines">-</span></div>
                <div>Heap: <span id="heapSize">-</span> MB</div>
                <div>GC Pause: <span id="gcPause">-</span> ms</div>
                <div>GC Count: <span id="gcCount">-</span></div>
            </div>
            
            <div class="metric-card">
                <div class="metric-title">Network</div>
                <div>RX: <span id="netRx">-</span> MB</div>
                <div>TX: <span id="netTx">-</span> MB</div>
                <div>Errors: <span id="netErrors">-</span></div>
                <div>Drops: <span id="netDrops">-</span></div>
            </div>
        </div>
        
        <div class="alerts-section">
            <h2>Recent Alerts</h2>
            <div id="alertsList">Loading alerts...</div>
        </div>
    </div>
    
    <script>
        let ws;
        
        function connectWebSocket() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            ws = new WebSocket(protocol + '//' + window.location.host + '/ws');
            
            ws.onmessage = function(event) {
                const data = JSON.parse(event.data);
                if (data.type === 'metrics_update') {
                    updateMetrics(data.metrics, data.summary);
                }
            };
            
            ws.onclose = function() {
                setTimeout(connectWebSocket, 5000); // Reconnect after 5 seconds
            };
            
            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
            };
        }
        
        function updateMetrics(metrics, summary) {
            document.getElementById('lastUpdate').textContent = new Date().toLocaleTimeString();
            
            // Update status
            const statusEl = document.getElementById('status');
            statusEl.textContent = '●  ' + summary.status.toUpperCase();
            statusEl.className = 'status-' + (summary.status === 'healthy' ? 'good' : 
                                            summary.status === 'warning' ? 'warn' : 'critical');
            
            // Update CPU metrics
            updateMetric('cpuUsage', metrics.cpu.usage_percent, '%', 80, 90);
            updateProgressBar('cpuProgress', metrics.cpu.usage_percent, 80, 90);
            document.getElementById('loadAvg').textContent = metrics.cpu.load_average ? 
                metrics.cpu.load_average.map(x => x.toFixed(2)).join(', ') : '-';
            document.getElementById('cpuCores').textContent = metrics.cpu.core_count || '-';
            
            // Update memory metrics
            updateMetric('memoryUsage', metrics.memory.usage_percent, '%', 85, 90);
            updateProgressBar('memoryProgress', metrics.memory.usage_percent, 85, 90);
            document.getElementById('memoryUsed').textContent = metrics.memory.used_mb || '-';
            document.getElementById('memoryTotal').textContent = metrics.memory.total_mb || '-';
            document.getElementById('swapUsage').textContent = (metrics.memory.swap_percent || 0).toFixed(1);
            
            // Update disk metrics
            updateMetric('diskUsage', metrics.disk.usage_percent, '%', 80, 90);
            updateProgressBar('diskProgress', metrics.disk.usage_percent, 80, 90);
            document.getElementById('diskUsed').textContent = metrics.disk.used_gb || '-';
            document.getElementById('diskTotal').textContent = metrics.disk.total_gb || '-';
            document.getElementById('diskFree').textContent = metrics.disk.free_gb || '-';
            
            // Update temperature
            updateMetric('temperature', metrics.system.temperature, '°C', 70, 80);
            updateProgressBar('tempProgress', metrics.system.temperature, 70, 80, 100);
            document.getElementById('thermalState').textContent = metrics.pi_specific.thermal_state || '-';
            
            // Update Go runtime
            document.getElementById('goroutines').textContent = metrics.go_runtime.num_goroutine || '-';
            document.getElementById('heapSize').textContent = metrics.go_runtime.memory_heap || '-';
            document.getElementById('gcPause').textContent = metrics.go_runtime.gc_pause_ms || '-';
            document.getElementById('gcCount').textContent = metrics.go_runtime.gc_count || '-';
            
            // Update network
            document.getElementById('netRx').textContent = metrics.network.bytes_received_mb || '-';
            document.getElementById('netTx').textContent = metrics.network.bytes_sent_mb || '-';
            document.getElementById('netErrors').textContent = metrics.network.errors || '-';
            document.getElementById('netDrops').textContent = metrics.network.drops || '-';
        }
        
        function updateMetric(elementId, value, unit, warnThreshold, criticalThreshold) {
            const el = document.getElementById(elementId);
            if (value !== undefined && value !== null) {
                el.textContent = value.toFixed(1) + unit;
                el.className = 'metric-value ' + getStatusClass(value, warnThreshold, criticalThreshold);
            }
        }
        
        function updateProgressBar(elementId, value, warnThreshold, criticalThreshold, max = 100) {
            const el = document.getElementById(elementId);
            const percentage = Math.min((value / max) * 100, 100);
            el.style.width = percentage + '%';
            el.className = 'progress-fill ' + getProgressClass(value, warnThreshold, criticalThreshold);
        }
        
        function getStatusClass(value, warnThreshold, criticalThreshold) {
            if (value >= criticalThreshold) return 'status-critical';
            if (value >= warnThreshold) return 'status-warn';
            return 'status-good';
        }
        
        function getProgressClass(value, warnThreshold, criticalThreshold) {
            if (value >= criticalThreshold) return 'progress-critical';
            if (value >= warnThreshold) return 'progress-warn';
            return 'progress-good';
        }
        
        function loadAlerts() {
            fetch('/api/alerts?hours=24')
                .then(response => response.json())
                .then(alerts => {
                    const alertsList = document.getElementById('alertsList');
                    if (alerts.length === 0) {
                        alertsList.innerHTML = '<p>No alerts in the last 24 hours.</p>';
                        return;
                    }
                    
                    alertsList.innerHTML = alerts.map(alert => 
                        '<div class="alert-item alert-' + alert.level.toLowerCase() + '">' +
                        '<strong>' + alert.level + ':</strong> ' + alert.message + '<br>' +
                        '<small>' + alert.component + ' - ' + alert.metric + ' = ' + 
                        alert.value.toFixed(2) + ' (threshold: ' + alert.threshold.toFixed(2) + ')</small><br>' +
                        '<small class="timestamp">' + new Date(alert.timestamp).toLocaleString() + '</small>' +
                        '</div>'
                    ).join('');
                })
                .catch(error => {
                    console.error('Error loading alerts:', error);
                    document.getElementById('alertsList').innerHTML = '<p>Error loading alerts.</p>';
                });
        }
        
        // Initialize
        connectWebSocket();
        loadAlerts();
        setInterval(loadAlerts, 60000); // Reload alerts every minute
    </script>
</body>
</html>
    `
	
	t, err := template.New("dashboard").Parse(tmpl)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, nil)
}

// Main function to run the performance monitor with web server
func main() {
	// Configuration
	webhookURL := os.Getenv("DISCORD_WEBHOOK_URL") // Optional Discord notifications
	port := 8080
	if portStr := os.Getenv("MONITOR_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}
	
	// Create performance monitor
	monitor := NewPiPerformanceMonitor(webhookURL)
	
	// Create web server
	server := NewMonitorServer(monitor, port)
	
	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Start monitor and server
	go func() {
		if err := monitor.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("Monitor error: %v", err)
		}
	}()
	
	go func() {
		if err := server.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("Server error: %v", err)
		}
	}()
	
	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received, stopping monitor...")
	
	// Cancel context to stop all goroutines
	cancel()
	
	// Give some time for graceful shutdown
	time.Sleep(2 * time.Second)
	log.Println("Pi Performance Monitor stopped")
}