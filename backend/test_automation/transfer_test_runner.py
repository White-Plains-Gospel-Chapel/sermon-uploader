#!/usr/bin/env python3

"""
Pi-to-Pi Transfer Test Runner
Comprehensive testing of MinIO optimizations with API integration
"""

import asyncio
import aiohttp
import hashlib
import json
import logging
import os
import psutil
import subprocess
import sys
import time
import traceback
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional, Tuple

# Configuration
PI1_HOST = "192.168.1.195"
PI2_HOST = "192.168.1.127" 
PI1_USER = "gaius"
PI2_USER = "gaius"
TEST_FILES_BASE = "/home/gaius/data/sermon-test-wavs"
API_BASE = "http://192.168.1.127:8000"
RESULTS_DIR = Path("./test_results")
TIMESTAMP = datetime.now().strftime("%Y%m%d_%H%M%S")

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler(RESULTS_DIR / f'transfer_test_{TIMESTAMP}.log'),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger(__name__)

# Create results directory
RESULTS_DIR.mkdir(exist_ok=True)

class TestFile:
    def __init__(self, path: str, category: str, expected_size: int = 0):
        self.path = path
        self.category = category
        self.expected_size = expected_size
        self.hash = None
        self.actual_size = None
        
    def __repr__(self):
        return f"TestFile({self.path}, {self.category}, {self.expected_size})"

class TransferResult:
    def __init__(self):
        self.success = False
        self.file_path = ""
        self.file_size = 0
        self.transfer_time = 0.0
        self.upload_speed = 0.0
        self.http_status = 0
        self.error_message = ""
        self.hash_verified = False
        self.timestamp = datetime.now()
        
    def to_dict(self):
        return {
            'success': self.success,
            'file_path': self.file_path,
            'file_size': self.file_size,
            'transfer_time': self.transfer_time,
            'upload_speed': self.upload_speed,
            'http_status': self.http_status,
            'error_message': self.error_message,
            'hash_verified': self.hash_verified,
            'timestamp': self.timestamp.isoformat()
        }

class SystemMetrics:
    def __init__(self, host: str):
        self.host = host
        self.timestamp = datetime.now()
        self.memory_used = 0
        self.memory_total = 0
        self.cpu_percent = 0.0
        self.disk_used = 0
        self.disk_total = 0
        self.network_connections = 0
        
    def to_dict(self):
        return {
            'host': self.host,
            'timestamp': self.timestamp.isoformat(),
            'memory_used': self.memory_used,
            'memory_total': self.memory_total,
            'memory_percent': (self.memory_used / self.memory_total * 100) if self.memory_total > 0 else 0,
            'cpu_percent': self.cpu_percent,
            'disk_used': self.disk_used,
            'disk_total': self.disk_total,
            'disk_percent': (self.disk_used / self.disk_total * 100) if self.disk_total > 0 else 0,
            'network_connections': self.network_connections
        }

class PiTestSuite:
    def __init__(self):
        self.session = None
        self.results: List[TransferResult] = []
        self.system_metrics: List[SystemMetrics] = []
        
        # Define test file categories
        self.test_files = {
            'small': [
                TestFile(f"{TEST_FILES_BASE}/Users/gaius/Documents/WPGC web/sermon-uploader/test-uploads/small_test_5sec.wav", "small"),
                TestFile(f"{TEST_FILES_BASE}/Users/gaius/Documents/WPGC web/sermon-uploader/test-uploads/medium_test_30sec.wav", "small"),
                TestFile(f"{TEST_FILES_BASE}/Users/gaius/Documents/WPGC web/sermon-uploader/test-uploads/large_test_3min.wav", "small"),
            ],
            'medium': [
                TestFile(f"{TEST_FILES_BASE}/Users/gaius/Desktop/Bobby Thomas.wav", "medium"),
                TestFile(f"{TEST_FILES_BASE}/Users/gaius/Desktop/Br. Thomas George.wav", "medium"),
                TestFile(f"{TEST_FILES_BASE}/Users/gaius/Documents/WPGC web/sermon-uploader/test-uploads/sermon_60min.wav", "medium"),
            ],
            'large': [
                TestFile(f"{TEST_FILES_BASE}/Users/gaius/Documents/WPGC web/sermon-uploader/test-uploads/sermon_80min_test_1.wav", "large"),
                TestFile(f"{TEST_FILES_BASE}/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_031_773MB.wav", "large"),
                TestFile(f"{TEST_FILES_BASE}/generated-1gb/sermon_batch_001_1GB.wav", "large"),
            ],
            'stress': [
                TestFile(f"{TEST_FILES_BASE}/Users/gaius/Documents/WPGC web/sermon-uploader/stress-test-files/sermon_{i:03d}_{size}MB.wav", "stress")
                for i, size in enumerate([688, 758, 724, 768, 656, 636, 682, 748, 746, 657], 1)
            ]
        }
    
    async def __aenter__(self):
        self.session = aiohttp.ClientSession(
            timeout=aiohttp.ClientTimeout(total=3600),  # 1 hour timeout
            connector=aiohttp.TCPConnector(limit=10, limit_per_host=5)
        )
        return self
    
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.session:
            await self.session.close()
    
    def check_connectivity(self) -> bool:
        """Check connectivity to both Pi systems and the API"""
        logger.info("Checking system connectivity...")
        
        # Test Pi1 SSH
        try:
            result = subprocess.run([
                "ssh", "-o", "ConnectTimeout=5", "-o", "BatchMode=yes",
                f"{PI1_USER}@{PI1_HOST}", "echo 'Pi1 connected'"
            ], capture_output=True, timeout=10, text=True)
            if result.returncode != 0:
                logger.error(f"Cannot connect to Pi1 ({PI1_HOST}): {result.stderr}")
                return False
        except Exception as e:
            logger.error(f"Pi1 connection error: {e}")
            return False
        
        # Test Pi2 SSH
        try:
            result = subprocess.run([
                "ssh", "-o", "ConnectTimeout=5", "-o", "BatchMode=yes", 
                f"{PI2_USER}@{PI2_HOST}", "echo 'Pi2 connected'"
            ], capture_output=True, timeout=10, text=True)
            if result.returncode != 0:
                logger.error(f"Cannot connect to Pi2 ({PI2_HOST}): {result.stderr}")
                return False
        except Exception as e:
            logger.error(f"Pi2 connection error: {e}")
            return False
        
        # Test API
        try:
            result = subprocess.run([
                "curl", "-s", "-m", "5", f"{API_BASE}/health"
            ], capture_output=True, timeout=10)
            if result.returncode != 0:
                logger.error(f"Backend API not responding at {API_BASE}")
                return False
        except Exception as e:
            logger.error(f"API connection error: {e}")
            return False
        
        logger.info("All systems connected successfully")
        return True
    
    def get_file_info(self, file_path: str) -> Tuple[Optional[int], Optional[str]]:
        """Get file size and hash from Pi1"""
        try:
            # Get file size
            size_result = subprocess.run([
                "ssh", f"{PI1_USER}@{PI1_HOST}",
                f"stat -c%s '{file_path}' 2>/dev/null || echo '0'"
            ], capture_output=True, text=True, timeout=30)
            
            file_size = int(size_result.stdout.strip())
            if file_size == 0:
                logger.warning(f"File not found or empty: {file_path}")
                return None, None
            
            # Get file hash
            hash_result = subprocess.run([
                "ssh", f"{PI1_USER}@{PI1_HOST}",
                f"sha256sum '{file_path}'"
            ], capture_output=True, text=True, timeout=60)
            
            if hash_result.returncode == 0:
                file_hash = hash_result.stdout.split()[0]
                return file_size, file_hash
            else:
                logger.warning(f"Could not get hash for {file_path}")
                return file_size, None
                
        except Exception as e:
            logger.error(f"Error getting file info for {file_path}: {e}")
            return None, None
    
    def get_system_metrics(self, host: str) -> SystemMetrics:
        """Collect system metrics from a Pi"""
        metrics = SystemMetrics(host)
        
        try:
            # Get memory info
            result = subprocess.run([
                "ssh", f"{PI1_USER}@{host}",
                "free -b | grep '^Mem:'"
            ], capture_output=True, text=True, timeout=10)
            
            if result.returncode == 0:
                mem_line = result.stdout.strip().split()
                metrics.memory_total = int(mem_line[1])
                metrics.memory_used = int(mem_line[2])
            
            # Get CPU load
            result = subprocess.run([
                "ssh", f"{PI1_USER}@{host}",
                "uptime | awk '{print $(NF-2)}' | sed 's/,//'"
            ], capture_output=True, text=True, timeout=10)
            
            if result.returncode == 0:
                metrics.cpu_percent = float(result.stdout.strip())
            
            # Get disk usage
            result = subprocess.run([
                "ssh", f"{PI1_USER}@{host}",
                "df -B1 / | tail -n 1"
            ], capture_output=True, text=True, timeout=10)
            
            if result.returncode == 0:
                disk_line = result.stdout.strip().split()
                metrics.disk_total = int(disk_line[1])
                metrics.disk_used = int(disk_line[2])
            
            # Get network connections
            result = subprocess.run([
                "ssh", f"{PI1_USER}@{host}",
                "netstat -tn | grep :8000 | wc -l"
            ], capture_output=True, text=True, timeout=10)
            
            if result.returncode == 0:
                metrics.network_connections = int(result.stdout.strip())
        
        except Exception as e:
            logger.warning(f"Error collecting metrics from {host}: {e}")
        
        return metrics
    
    async def transfer_file_via_api(self, file_path: str, test_name: str) -> TransferResult:
        """Transfer a single file via the API and measure performance"""
        result = TransferResult()
        result.file_path = file_path
        
        logger.info(f"Starting transfer: {test_name}")
        logger.info(f"File: {os.path.basename(file_path)}")
        
        try:
            # Get file info
            file_size, file_hash = self.get_file_info(file_path)
            if file_size is None:
                result.error_message = "File not found or empty"
                return result
            
            result.file_size = file_size
            logger.info(f"File size: {file_size / 1024 / 1024:.1f} MB")
            
            # Collect baseline metrics
            baseline_metrics = self.get_system_metrics(PI2_HOST)
            self.system_metrics.append(baseline_metrics)
            
            # Copy file to local temp for upload
            temp_file = f"/tmp/{os.path.basename(file_path)}_{int(time.time())}"
            
            logger.info("Copying file from Pi1 to local temp...")
            scp_result = subprocess.run([
                "scp", f"{PI1_USER}@{PI1_HOST}:{file_path}", temp_file
            ], timeout=300)  # 5 minute timeout for copy
            
            if scp_result.returncode != 0:
                result.error_message = "Failed to copy file from Pi1"
                return result
            
            # Upload via API
            start_time = time.time()
            logger.info("Uploading via API...")
            
            with open(temp_file, 'rb') as f:
                data = aiohttp.FormData()
                data.add_field('file',
                             f,
                             filename=os.path.basename(file_path),
                             content_type='audio/wav')
                
                async with self.session.post(f"{API_BASE}/upload", data=data) as response:
                    end_time = time.time()
                    transfer_time = end_time - start_time
                    
                    result.transfer_time = transfer_time
                    result.http_status = response.status
                    result.upload_speed = file_size / transfer_time if transfer_time > 0 else 0
                    
                    if response.status in [200, 201]:
                        result.success = True
                        response_data = await response.text()
                        logger.info(f"Upload successful: {response.status}")
                    else:
                        result.error_message = f"HTTP {response.status}: {await response.text()}"
            
            # Clean up temp file
            try:
                os.unlink(temp_file)
            except:
                pass
            
            # Collect final metrics
            final_metrics = self.get_system_metrics(PI2_HOST)
            self.system_metrics.append(final_metrics)
            
            # Log results
            speed_mbps = result.upload_speed / (1024 * 1024) if result.upload_speed > 0 else 0
            logger.info(f"Transfer completed: {transfer_time:.2f}s, {speed_mbps:.2f} MB/s")
            
            # Save detailed results
            detailed_results = {
                'test_name': test_name,
                'file_path': file_path,
                'file_size': file_size,
                'file_hash': file_hash,
                'transfer_time': transfer_time,
                'upload_speed_mbps': speed_mbps,
                'http_status': result.http_status,
                'success': result.success,
                'error_message': result.error_message,
                'baseline_metrics': baseline_metrics.to_dict(),
                'final_metrics': final_metrics.to_dict(),
                'timestamp': datetime.now().isoformat()
            }
            
            with open(RESULTS_DIR / f"{test_name}_detailed.json", 'w') as f:
                json.dump(detailed_results, f, indent=2)
        
        except Exception as e:
            result.error_message = f"Exception during transfer: {str(e)}"
            logger.error(f"Transfer failed: {e}")
            logger.error(traceback.format_exc())
        
        return result
    
    async def run_batch_test(self, files: List[TestFile], test_name: str, max_concurrent: int = 3) -> Dict:
        """Run batch transfer test with specified concurrency"""
        logger.info(f"Starting batch test: {test_name} (max concurrent: {max_concurrent})")
        
        # Filter available files
        available_files = []
        for test_file in files:
            size, hash_val = self.get_file_info(test_file.path)
            if size is not None:
                test_file.actual_size = size
                test_file.hash = hash_val
                available_files.append(test_file)
            else:
                logger.warning(f"Skipping unavailable file: {test_file.path}")
        
        if not available_files:
            logger.error(f"No files available for {test_name}")
            return {'success': False, 'message': 'No files available'}
        
        logger.info(f"Testing with {len(available_files)} files")
        
        # Collect baseline metrics
        baseline_metrics = self.get_system_metrics(PI2_HOST)
        
        # Run transfers with concurrency control
        semaphore = asyncio.Semaphore(max_concurrent)
        start_time = time.time()
        
        async def transfer_with_semaphore(test_file: TestFile, index: int):
            async with semaphore:
                file_test_name = f"{test_name}_{index:03d}_{os.path.basename(test_file.path)}"
                return await self.transfer_file_via_api(test_file.path, file_test_name)
        
        # Execute all transfers
        tasks = [transfer_with_semaphore(f, i) for i, f in enumerate(available_files)]
        results = await asyncio.gather(*tasks, return_exceptions=True)
        
        end_time = time.time()
        total_time = end_time - start_time
        
        # Collect final metrics
        final_metrics = self.get_system_metrics(PI2_HOST)
        
        # Analyze results
        successful = 0
        failed = 0
        total_bytes = 0
        
        for i, result in enumerate(results):
            if isinstance(result, TransferResult):
                self.results.append(result)
                if result.success:
                    successful += 1
                    total_bytes += result.file_size
                else:
                    failed += 1
            else:
                failed += 1
                logger.error(f"Transfer {i} resulted in exception: {result}")
        
        success_rate = (successful / len(results)) * 100 if results else 0
        avg_speed = (total_bytes / total_time / 1024 / 1024) if total_time > 0 else 0
        
        # Save batch summary
        batch_summary = {
            'test_name': test_name,
            'total_files': len(available_files),
            'max_concurrent': max_concurrent,
            'successful_transfers': successful,
            'failed_transfers': failed,
            'success_rate': success_rate,
            'total_time': total_time,
            'total_bytes': total_bytes,
            'average_speed_mbps': avg_speed,
            'average_time_per_file': total_time / len(available_files) if available_files else 0,
            'baseline_metrics': baseline_metrics.to_dict(),
            'final_metrics': final_metrics.to_dict(),
            'timestamp': datetime.now().isoformat()
        }
        
        with open(RESULTS_DIR / f"{test_name}_batch_summary.json", 'w') as f:
            json.dump(batch_summary, f, indent=2)
        
        logger.info(f"Batch test completed: {success_rate:.1f}% success rate")
        logger.info(f"Total time: {total_time:.2f}s, Average speed: {avg_speed:.2f} MB/s")
        
        return batch_summary
    
    def generate_final_report(self):
        """Generate comprehensive final report"""
        report_file = RESULTS_DIR / f"comprehensive_test_report_{TIMESTAMP}.json"
        
        # Analyze all results
        total_transfers = len(self.results)
        successful_transfers = sum(1 for r in self.results if r.success)
        failed_transfers = total_transfers - successful_transfers
        overall_success_rate = (successful_transfers / total_transfers * 100) if total_transfers > 0 else 0
        
        # Calculate performance metrics
        if successful_transfers > 0:
            successful_results = [r for r in self.results if r.success]
            avg_speed = sum(r.upload_speed for r in successful_results) / len(successful_results) / (1024 * 1024)
            avg_time = sum(r.transfer_time for r in successful_results) / len(successful_results)
            total_data = sum(r.file_size for r in successful_results)
        else:
            avg_speed = avg_time = total_data = 0
        
        # Load batch summaries
        batch_summaries = []
        for batch_file in RESULTS_DIR.glob("*_batch_summary.json"):
            with open(batch_file) as f:
                batch_summaries.append(json.load(f))
        
        report = {
            'test_metadata': {
                'timestamp': TIMESTAMP,
                'test_date': datetime.now().isoformat(),
                'pi1_host': PI1_HOST,
                'pi2_host': PI2_HOST,
                'api_base': API_BASE,
                'results_directory': str(RESULTS_DIR)
            },
            'summary': {
                'total_individual_transfers': total_transfers,
                'successful_transfers': successful_transfers,
                'failed_transfers': failed_transfers,
                'overall_success_rate': overall_success_rate,
                'batch_tests_completed': len(batch_summaries)
            },
            'performance_metrics': {
                'average_speed_mbps': avg_speed,
                'average_transfer_time': avg_time,
                'total_data_transferred_bytes': total_data,
                'total_data_transferred_gb': total_data / (1024**3)
            },
            'validation_criteria': {
                'success_rate_target': 95.0,
                'success_rate_achieved': overall_success_rate,
                'success_rate_met': overall_success_rate >= 95.0,
                'speed_target_mbps': 5.0,
                'speed_achieved_mbps': avg_speed,
                'speed_target_met': avg_speed >= 5.0
            },
            'individual_results': [r.to_dict() for r in self.results],
            'batch_summaries': batch_summaries,
            'system_metrics': [m.to_dict() for m in self.system_metrics]
        }
        
        with open(report_file, 'w') as f:
            json.dump(report, f, indent=2, default=str)
        
        logger.info(f"Final report generated: {report_file}")
        return report
    
    async def run_all_tests(self):
        """Execute the complete test suite"""
        logger.info("Starting Pi-to-Pi Transfer Performance Testing")
        logger.info(f"Timestamp: {TIMESTAMP}")
        logger.info(f"Results directory: {RESULTS_DIR}")
        
        # Check connectivity
        if not self.check_connectivity():
            logger.error("Connectivity check failed. Aborting tests.")
            return
        
        try:
            # Test 1: Small Files (Sequential)
            logger.info("=== TEST 1: Small Files (Sequential) ===")
            for i, test_file in enumerate(self.test_files['small']):
                result = await self.transfer_file_via_api(
                    test_file.path, 
                    f"small_sequential_{i:03d}_{os.path.basename(test_file.path)}"
                )
                self.results.append(result)
                await asyncio.sleep(2)  # Brief pause between transfers
            
            # Test 2: Medium Files (Sequential)
            logger.info("=== TEST 2: Medium Files (Sequential) ===")
            for i, test_file in enumerate(self.test_files['medium']):
                result = await self.transfer_file_via_api(
                    test_file.path,
                    f"medium_sequential_{i:03d}_{os.path.basename(test_file.path)}"
                )
                self.results.append(result)
                await asyncio.sleep(5)  # Longer pause for medium files
            
            # Test 3: Large Files (Sequential) 
            logger.info("=== TEST 3: Large Files (Sequential) ===")
            for i, test_file in enumerate(self.test_files['large']):
                result = await self.transfer_file_via_api(
                    test_file.path,
                    f"large_sequential_{i:03d}_{os.path.basename(test_file.path)}"
                )
                self.results.append(result)
                await asyncio.sleep(10)  # Long pause for large files
            
            # Test 4: Batch Upload (3 concurrent medium files)
            logger.info("=== TEST 4: Batch Upload Test (3 concurrent) ===")
            await self.run_batch_test(
                self.test_files['medium'], 
                "batch_3_concurrent_medium", 
                max_concurrent=3
            )
            
            # Test 5: Stress Test (10 files, 2 concurrent)
            logger.info("=== TEST 5: Stress Test (Original Problem Scenario) ===")
            stress_files = self.test_files['stress'][:10]  # Limit to 10 files
            await self.run_batch_test(
                stress_files,
                "stress_test_10_files_2_concurrent",
                max_concurrent=2
            )
            
            # Test 6: Memory Pressure Test (Large files, 1 concurrent)
            logger.info("=== TEST 6: Memory Pressure Test ===")
            await self.run_batch_test(
                self.test_files['large'],
                "memory_pressure_1_concurrent",
                max_concurrent=1
            )
            
        except Exception as e:
            logger.error(f"Test execution error: {e}")
            logger.error(traceback.format_exc())
        
        finally:
            # Generate final report
            logger.info("=== GENERATING FINAL REPORT ===")
            report = self.generate_final_report()
            
            # Print summary
            logger.info("=== TEST SUMMARY ===")
            logger.info(f"Total transfers: {len(self.results)}")
            logger.info(f"Success rate: {report['summary']['overall_success_rate']:.1f}%")
            logger.info(f"Average speed: {report['performance_metrics']['average_speed_mbps']:.2f} MB/s")
            logger.info(f"Validation criteria met: Speed={report['validation_criteria']['speed_target_met']}, Success={report['validation_criteria']['success_rate_met']}")
            
            logger.info("All tests completed! Check test_results/ for detailed results.")

async def main():
    """Main entry point"""
    async with PiTestSuite() as test_suite:
        await test_suite.run_all_tests()

if __name__ == "__main__":
    asyncio.run(main())