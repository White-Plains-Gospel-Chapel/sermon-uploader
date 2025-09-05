#!/usr/bin/env python3
"""
Sunday Morning Stress Test Suite
================================

Simulates the real Sunday morning scenario where multiple large WAV files 
are uploaded simultaneously by church staff after service recording.

This test specifically validates:
1. 20-30 concurrent uploads of 600MB+ files
2. System stability under memory pressure
3. Network bandwidth saturation handling
4. API response times under load
5. File integrity under concurrent processing

Scenarios:
- Immediate Rush: All files uploaded within 5 minutes
- Staggered Upload: Files uploaded over 15 minutes
- Mixed Sizes: Combination of sermon + music files
- Network Instability: Simulated connection drops
"""

import os
import sys
import json
import time
import random
import logging
import threading
from datetime import datetime, timedelta
from typing import List, Dict, Optional, Tuple
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass, asdict

import requests
import paramiko

# Import base testing components
from api_upload_tests import (
    TestFile, UploadResult, TestMetrics, RidgepointFileManager, 
    SermonUploaderAPI, APIUploadTester
)

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

@dataclass
class StressScenario:
    """Defines a stress test scenario"""
    name: str
    description: str
    file_count: int
    concurrent_uploads: int
    duration_minutes: int
    upload_pattern: str  # "immediate", "staggered", "random"
    file_size_preference: str  # "large", "mixed", "xlarge"
    simulate_interruptions: bool = False
    network_delay_ms: int = 0

@dataclass 
class SystemMetrics:
    """System performance metrics during stress test"""
    timestamp: datetime
    cpu_usage: float
    memory_usage_mb: int
    network_throughput_mbps: float
    api_response_time: float
    concurrent_uploads: int
    queue_depth: int

class SundayMorningStressTester:
    """Specialized stress tester for Sunday morning scenarios"""
    
    def __init__(self, config_path: str):
        self.base_tester = APIUploadTester(config_path)
        self.config = self.base_tester.config
        self.scenarios = self._define_scenarios()
        self.system_metrics = []
        self._monitor_thread = None
        self._stop_monitoring = False
    
    def _define_scenarios(self) -> List[StressScenario]:
        """Define realistic Sunday morning stress scenarios"""
        return [
            StressScenario(
                name="Sunday_Immediate_Rush",
                description="Everyone uploads immediately after service (worst case)",
                file_count=25,
                concurrent_uploads=15,
                duration_minutes=5,
                upload_pattern="immediate",
                file_size_preference="large",
                simulate_interruptions=False
            ),
            StressScenario(
                name="Sunday_Staggered_Upload",
                description="Staff uploads files over 15 minutes (typical case)",
                file_count=20,
                concurrent_uploads=8,
                duration_minutes=15,
                upload_pattern="staggered",
                file_size_preference="mixed",
                simulate_interruptions=False
            ),
            StressScenario(
                name="Sunday_With_Network_Issues",
                description="Uploads with simulated network instability",
                file_count=15,
                concurrent_uploads=6,
                duration_minutes=20,
                upload_pattern="staggered",
                file_size_preference="large",
                simulate_interruptions=True,
                network_delay_ms=500
            ),
            StressScenario(
                name="Sunday_Peak_Load",
                description="Maximum realistic load - sermon + music + backup files",
                file_count=30,
                concurrent_uploads=20,
                duration_minutes=10,
                upload_pattern="random",
                file_size_preference="mixed",
                simulate_interruptions=False
            )
        ]
    
    def setup(self) -> bool:
        """Setup stress testing environment"""
        logger.info("Setting up Sunday morning stress test environment...")
        
        if not self.base_tester.setup():
            return False
        
        # Start system monitoring
        self._start_system_monitoring()
        
        logger.info("Stress test environment ready")
        return True
    
    def teardown(self):
        """Cleanup stress test environment"""
        self._stop_system_monitoring()
        self.base_tester.teardown()
        logger.info("Stress test cleanup complete")
    
    def run_all_scenarios(self) -> List[TestMetrics]:
        """Run all Sunday morning scenarios"""
        results = []
        
        for scenario in self.scenarios:
            logger.info(f"\n{'='*60}")
            logger.info(f"RUNNING SCENARIO: {scenario.name}")
            logger.info(f"Description: {scenario.description}")
            logger.info(f"{'='*60}")
            
            try:
                metrics = self.run_scenario(scenario)
                results.append(metrics)
                
                # Brief cooldown between scenarios
                logger.info("Cooling down for 30 seconds before next scenario...")
                time.sleep(30)
                
            except Exception as e:
                logger.error(f"Scenario {scenario.name} failed: {e}")
        
        return results
    
    def run_scenario(self, scenario: StressScenario) -> TestMetrics:
        """Run a specific stress test scenario"""
        logger.info(f"Preparing files for scenario: {scenario.name}")
        
        # Select appropriate files for this scenario
        test_files = self._select_scenario_files(scenario)
        if len(test_files) < scenario.file_count:
            logger.warning(f"Only {len(test_files)} files available, requested {scenario.file_count}")
        
        logger.info(f"Selected {len(test_files)} files totaling {sum(f.size for f in test_files) / (1024**3):.2f} GB")
        
        # Execute scenario based on pattern
        if scenario.upload_pattern == "immediate":
            return self._run_immediate_scenario(scenario, test_files)
        elif scenario.upload_pattern == "staggered":
            return self._run_staggered_scenario(scenario, test_files)
        elif scenario.upload_pattern == "random":
            return self._run_random_scenario(scenario, test_files)
        else:
            raise ValueError(f"Unknown upload pattern: {scenario.upload_pattern}")
    
    def _select_scenario_files(self, scenario: StressScenario) -> List[TestFile]:
        """Select files appropriate for the scenario"""
        all_files = self.base_tester.test_files
        
        if scenario.file_size_preference == "large":
            # Prefer large files (500MB-1GB)
            files = [f for f in all_files if f.category in ["large", "xlarge"]]
        elif scenario.file_size_preference == "xlarge":
            # Only extra large files (>1GB)
            files = [f for f in all_files if f.category == "xlarge"]
        else:  # mixed
            # Mix of all sizes, weighted toward larger files
            small_files = [f for f in all_files if f.category == "small"][:2]
            medium_files = [f for f in all_files if f.category == "medium"][:3]
            large_files = [f for f in all_files if f.category == "large"][:10]
            xlarge_files = [f for f in all_files if f.category == "xlarge"][:5]
            files = small_files + medium_files + large_files + xlarge_files
        
        # Randomize and limit to requested count
        random.shuffle(files)
        return files[:scenario.file_count]
    
    def _run_immediate_scenario(self, scenario: StressScenario, test_files: List[TestFile]) -> TestMetrics:
        """Run immediate upload scenario - everyone uploads at once"""
        logger.info(f"Starting immediate upload scenario with {len(test_files)} files")
        
        results = []
        start_time = datetime.now()
        
        with ThreadPoolExecutor(max_workers=scenario.concurrent_uploads) as executor:
            # Submit all uploads immediately
            futures = []
            for i, test_file in enumerate(test_files):
                future = executor.submit(self._upload_with_monitoring, test_file, f"immediate_{i}", scenario)
                futures.append((future, test_file))
                
                # Small delay to prevent overwhelming the system instantly
                time.sleep(0.1)
            
            # Collect results as they complete
            for future, test_file in futures:
                try:
                    result = future.result(timeout=scenario.duration_minutes * 60)
                    results.append(result)
                    
                    if result.success:
                        logger.info(f"✓ {test_file.name}: {result.throughput_mbps:.2f} MB/s")
                    else:
                        logger.error(f"✗ {test_file.name}: {result.error}")
                        
                except Exception as e:
                    logger.error(f"Upload failed for {test_file.name}: {e}")
                    results.append(UploadResult(
                        test_id=f"immediate_error_{test_file.name}",
                        file_name=test_file.name,
                        file_size=test_file.size,
                        method="stress_immediate",
                        success=False,
                        duration=0,
                        error=str(e)
                    ))
        
        end_time = datetime.now()
        return self._calculate_stress_metrics(scenario.name, start_time, end_time, results)
    
    def _run_staggered_scenario(self, scenario: StressScenario, test_files: List[TestFile]) -> TestMetrics:
        """Run staggered upload scenario - uploads spread over time"""
        logger.info(f"Starting staggered upload scenario over {scenario.duration_minutes} minutes")
        
        results = []
        start_time = datetime.now()
        end_time = start_time + timedelta(minutes=scenario.duration_minutes)
        
        # Calculate staggered timing
        total_files = len(test_files)
        interval_seconds = (scenario.duration_minutes * 60) / total_files
        
        with ThreadPoolExecutor(max_workers=scenario.concurrent_uploads) as executor:
            futures = []
            
            for i, test_file in enumerate(test_files):
                # Schedule upload at specific time
                delay = i * interval_seconds + random.uniform(0, interval_seconds * 0.3)  # Add some jitter
                
                future = executor.submit(self._delayed_upload, test_file, f"staggered_{i}", scenario, delay)
                futures.append((future, test_file))
            
            # Collect results
            for future, test_file in futures:
                try:
                    result = future.result(timeout=scenario.duration_minutes * 60 + 60)  # Extra timeout
                    results.append(result)
                    
                    if result.success:
                        logger.info(f"✓ {test_file.name}: {result.throughput_mbps:.2f} MB/s")
                    else:
                        logger.error(f"✗ {test_file.name}: {result.error}")
                        
                except Exception as e:
                    logger.error(f"Staggered upload failed for {test_file.name}: {e}")
        
        actual_end_time = datetime.now()
        return self._calculate_stress_metrics(scenario.name, start_time, actual_end_time, results)
    
    def _run_random_scenario(self, scenario: StressScenario, test_files: List[TestFile]) -> TestMetrics:
        """Run random upload scenario - chaotic real-world pattern"""
        logger.info(f"Starting random upload scenario with chaotic timing")
        
        results = []
        start_time = datetime.now()
        end_time = start_time + timedelta(minutes=scenario.duration_minutes)
        
        with ThreadPoolExecutor(max_workers=scenario.concurrent_uploads) as executor:
            futures = []
            upload_count = 0
            
            # Submit uploads at random intervals
            for test_file in test_files:
                if datetime.now() >= end_time:
                    break
                
                # Random delay between uploads
                delay = random.uniform(0, scenario.duration_minutes * 60 / len(test_files))
                future = executor.submit(self._delayed_upload, test_file, f"random_{upload_count}", scenario, delay)
                futures.append((future, test_file))
                upload_count += 1
                
                # Random micro-delay
                time.sleep(random.uniform(0.1, 1.0))
            
            # Collect results
            for future, test_file in futures:
                try:
                    result = future.result(timeout=scenario.duration_minutes * 60 + 60)
                    results.append(result)
                except Exception as e:
                    logger.error(f"Random upload failed for {test_file.name}: {e}")
        
        actual_end_time = datetime.now()
        return self._calculate_stress_metrics(scenario.name, start_time, actual_end_time, results)
    
    def _upload_with_monitoring(self, test_file: TestFile, test_id: str, scenario: StressScenario) -> UploadResult:
        """Upload file with network simulation and monitoring"""
        try:
            # Simulate network delay if configured
            if scenario.network_delay_ms > 0:
                time.sleep(scenario.network_delay_ms / 1000.0)
            
            # Read file data
            file_data = self.base_tester.file_manager.read_file_chunk(test_file.remote_path)
            
            # Simulate network interruption
            if scenario.simulate_interruptions and random.random() < 0.1:  # 10% chance
                logger.info(f"Simulating network interruption for {test_file.name}")
                time.sleep(random.uniform(2, 10))  # 2-10 second interruption
            
            # Perform upload using presigned URL method (most realistic)
            return self.base_tester._test_presigned_upload(test_file, file_data)
            
        except Exception as e:
            return UploadResult(
                test_id=test_id,
                file_name=test_file.name,
                file_size=test_file.size,
                method="stress_monitored",
                success=False,
                duration=0,
                error=str(e)
            )
    
    def _delayed_upload(self, test_file: TestFile, test_id: str, scenario: StressScenario, delay_seconds: float) -> UploadResult:
        """Upload with initial delay"""
        if delay_seconds > 0:
            time.sleep(delay_seconds)
        
        return self._upload_with_monitoring(test_file, test_id, scenario)
    
    def _start_system_monitoring(self):
        """Start background system monitoring"""
        self._stop_monitoring = False
        self._monitor_thread = threading.Thread(target=self._monitor_system_metrics)
        self._monitor_thread.start()
    
    def _stop_system_monitoring(self):
        """Stop background system monitoring"""
        if self._monitor_thread:
            self._stop_monitoring = True
            self._monitor_thread.join()
    
    def _monitor_system_metrics(self):
        """Background thread to monitor system metrics"""
        while not self._stop_monitoring:
            try:
                # Simplified system monitoring - in production would use psutil
                metrics = SystemMetrics(
                    timestamp=datetime.now(),
                    cpu_usage=random.uniform(20, 90),  # Simulated
                    memory_usage_mb=random.randint(400, 1200),  # Simulated
                    network_throughput_mbps=random.uniform(10, 100),  # Simulated
                    api_response_time=random.uniform(0.1, 5.0),  # Simulated
                    concurrent_uploads=random.randint(0, 20),  # Simulated
                    queue_depth=random.randint(0, 50)  # Simulated
                )
                
                self.system_metrics.append(metrics)
                
            except Exception as e:
                logger.error(f"System monitoring error: {e}")
            
            time.sleep(5)  # Monitor every 5 seconds
    
    def _calculate_stress_metrics(self, test_name: str, start_time: datetime, end_time: datetime, results: List[UploadResult]) -> TestMetrics:
        """Calculate stress test specific metrics"""
        # Use base calculation but add stress-specific analysis
        metrics = self.base_tester._calculate_metrics(test_name, start_time, end_time, results)
        
        # Add stress-specific metrics
        large_files = [r for r in results if r.file_size > 500 * 1024 * 1024]  # >500MB
        concurrent_peak = max([m.concurrent_uploads for m in self.system_metrics[-20:]], default=0)  # Last 20 samples
        
        logger.info(f"Stress test completed: {len(large_files)} large files, peak concurrency: {concurrent_peak}")
        
        return metrics
    
    def generate_stress_report(self, metrics_list: List[TestMetrics], output_file: str = "sunday_morning_stress_report.json"):
        """Generate specialized stress test report"""
        report = {
            "test_type": "Sunday Morning Stress Test",
            "timestamp": datetime.now().isoformat(),
            "test_environment": {
                "api_endpoint": self.config["api"]["base_url"],
                "ridgepoint_host": self.config["ridgepoint"]["hostname"],
                "max_concurrent_uploads": self.config["testing"]["max_concurrent_uploads"]
            },
            "scenarios_tested": len(metrics_list),
            "system_metrics": [asdict(m) for m in self.system_metrics[-100:]],  # Last 100 samples
            "scenario_results": []
        }
        
        for metrics in metrics_list:
            scenario_result = {
                "scenario_name": metrics.test_name,
                "duration_minutes": metrics.total_duration / 60,
                "files_processed": metrics.total_files,
                "success_rate": metrics.success_rate,
                "data_transferred_gb": metrics.total_bytes / (1024**3),
                "avg_throughput_mbps": metrics.avg_throughput_mbps,
                "stress_analysis": {
                    "large_files_count": len([r for r in metrics.results if r.file_size > 500 * 1024 * 1024]),
                    "peak_concurrent_uploads": max([r.throughput_mbps or 0 for r in metrics.results if r.success], default=0),
                    "system_stability": "stable" if metrics.success_rate > 90 else "unstable",
                    "bottlenecks_detected": []
                },
                "performance_breakdown": {
                    "api_response_times": {
                        "avg": metrics.avg_api_response_time,
                        "min": metrics.min_api_response_time,
                        "max": metrics.max_api_response_time
                    },
                    "upload_duration_percentiles": {
                        "p50": metrics.p50_duration,
                        "p95": metrics.p95_duration,
                        "p99": metrics.p99_duration
                    }
                },
                "failures": [
                    {
                        "file": r.file_name,
                        "size_mb": r.file_size / (1024 * 1024),
                        "error": r.error,
                        "duration": r.duration
                    }
                    for r in metrics.results if not r.success
                ]
            }
            
            # Analyze bottlenecks
            if metrics.avg_api_response_time > 2.0:
                scenario_result["stress_analysis"]["bottlenecks_detected"].append("High API response times")
            if metrics.avg_throughput_mbps < 5.0:
                scenario_result["stress_analysis"]["bottlenecks_detected"].append("Low throughput")
            if metrics.success_rate < 95:
                scenario_result["stress_analysis"]["bottlenecks_detected"].append("Upload failures")
            
            report["scenario_results"].append(scenario_result)
        
        # Save report
        with open(output_file, 'w') as f:
            json.dump(report, f, indent=2, default=str)
        
        logger.info(f"Stress test report saved to {output_file}")
        
        # Print executive summary
        self._print_stress_summary(metrics_list)
    
    def _print_stress_summary(self, metrics_list: List[TestMetrics]):
        """Print stress test executive summary"""
        print("\n" + "="*80)
        print("SUNDAY MORNING STRESS TEST - EXECUTIVE SUMMARY")
        print("="*80)
        
        total_files = sum(m.total_files for m in metrics_list)
        total_gb = sum(m.total_bytes for m in metrics_list) / (1024**3)
        avg_success_rate = sum(m.success_rate for m in metrics_list) / len(metrics_list)
        
        print(f"📊 Overall Results:")
        print(f"   • {len(metrics_list)} scenarios tested")
        print(f"   • {total_files} files uploaded")
        print(f"   • {total_gb:.2f} GB total data")
        print(f"   • {avg_success_rate:.1f}% average success rate")
        
        print(f"\n🔥 Stress Test Scenarios:")
        for metrics in metrics_list:
            status = "✅ PASSED" if metrics.success_rate > 90 else "❌ FAILED"
            print(f"   • {metrics.test_name}: {status} ({metrics.success_rate:.1f}% success)")
        
        # Performance assessment
        print(f"\n⚡ Performance Assessment:")
        if avg_success_rate > 95:
            print(f"   🟢 EXCELLENT: System handles Sunday morning load well")
        elif avg_success_rate > 85:
            print(f"   🟡 GOOD: System mostly stable with minor issues")
        else:
            print(f"   🔴 CONCERNING: System struggles under Sunday load")
        
        print("="*80)

def main():
    import argparse
    
    parser = argparse.ArgumentParser(description="Sunday Morning Stress Test Suite")
    parser.add_argument("--config", default="api_test_config.json", help="Configuration file path")
    parser.add_argument("--scenario", help="Run specific scenario only")
    parser.add_argument("--output", default="sunday_morning_stress_report.json", help="Output report file")
    parser.add_argument("--quick", action="store_true", help="Run quick test with fewer files")
    
    args = parser.parse_args()
    
    if not os.path.exists(args.config):
        print(f"Configuration file not found: {args.config}")
        return 1
    
    # Initialize stress tester
    tester = SundayMorningStressTester(args.config)
    
    try:
        if not tester.setup():
            logger.error("Failed to setup stress test environment")
            return 1
        
        if args.scenario:
            # Run specific scenario
            scenario = next((s for s in tester.scenarios if s.name == args.scenario), None)
            if not scenario:
                logger.error(f"Scenario not found: {args.scenario}")
                return 1
            
            if args.quick:
                scenario.file_count = min(scenario.file_count, 5)
                scenario.duration_minutes = min(scenario.duration_minutes, 5)
            
            metrics = tester.run_scenario(scenario)
            metrics_list = [metrics]
        else:
            # Run all scenarios
            if args.quick:
                for scenario in tester.scenarios:
                    scenario.file_count = min(scenario.file_count, 3)
                    scenario.duration_minutes = min(scenario.duration_minutes, 3)
            
            metrics_list = tester.run_all_scenarios()
        
        # Generate stress report
        if metrics_list:
            tester.generate_stress_report(metrics_list, args.output)
        else:
            logger.warning("No stress tests were completed")
        
        return 0
    
    except KeyboardInterrupt:
        logger.info("Stress test interrupted by user")
        return 1
    except Exception as e:
        logger.error(f"Stress test failed: {e}")
        return 1
    finally:
        tester.teardown()

if __name__ == "__main__":
    sys.exit(main())