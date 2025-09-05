#!/usr/bin/env python3
"""
Comprehensive API Upload Testing Suite
=====================================

Tests the sermon uploader API endpoints using real 40GB WAV files from ridgepoint Pi.
This test suite validates:
- Direct API uploads via /api/upload
- Presigned URL uploads via /api/upload/presigned
- Batch presigned uploads via /api/upload/presigned-batch
- Performance and reliability under real load conditions

Usage:
    python3 api_upload_tests.py --config config.json
    python3 api_upload_tests.py --test single --size small
    python3 api_upload_tests.py --test batch --files 5
    python3 api_upload_tests.py --test stress --duration 300

Requirements:
    - Python 3.8+
    - requests
    - paramiko (for SSH file access)
    - concurrent.futures
"""

import os
import sys
import json
import time
import hashlib
import logging
import argparse
import threading
import statistics
from datetime import datetime, timedelta
from typing import Dict, List, Optional, Tuple, Any
from dataclasses import dataclass, asdict
from concurrent.futures import ThreadPoolExecutor, as_completed

import requests
import paramiko
from requests.adapters import HTTPAdapter
from requests.packages.urllib3.util.retry import Retry

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('api_upload_tests.log'),
        logging.StreamHandler(sys.stdout)
    ]
)
logger = logging.getLogger(__name__)

@dataclass
class TestFile:
    """Represents a test file from ridgepoint Pi"""
    name: str
    path: str
    size: int
    category: str  # small, medium, large, xlarge
    remote_path: str
    hash_sha256: Optional[str] = None

@dataclass
class UploadResult:
    """Result of a single upload operation"""
    test_id: str
    file_name: str
    file_size: int
    method: str  # 'direct', 'presigned', 'batch'
    success: bool
    duration: float
    error: Optional[str] = None
    throughput_mbps: Optional[float] = None
    response_data: Optional[Dict] = None
    api_response_time: Optional[float] = None
    upload_time: Optional[float] = None

@dataclass
class TestMetrics:
    """Comprehensive test metrics"""
    test_name: str
    start_time: datetime
    end_time: datetime
    total_files: int
    successful_uploads: int
    failed_uploads: int
    total_bytes: int
    total_duration: float
    avg_throughput_mbps: float
    success_rate: float
    results: List[UploadResult]
    
    # Performance metrics
    min_duration: float
    max_duration: float
    p50_duration: float
    p95_duration: float
    p99_duration: float
    
    # API performance
    avg_api_response_time: float
    min_api_response_time: float
    max_api_response_time: float

class RidgepointFileManager:
    """Manages SSH connection and file access to ridgepoint Pi"""
    
    def __init__(self, hostname: str, username: str, private_key_path: Optional[str] = None):
        self.hostname = hostname
        self.username = username
        self.private_key_path = private_key_path
        self.ssh_client = None
        self.sftp_client = None
    
    def connect(self) -> bool:
        """Establish SSH connection to ridgepoint Pi"""
        try:
            self.ssh_client = paramiko.SSHClient()
            self.ssh_client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
            
            if self.private_key_path and os.path.exists(self.private_key_path):
                key = paramiko.RSAKey.from_private_key_file(self.private_key_path)
                self.ssh_client.connect(self.hostname, username=self.username, pkey=key)
            else:
                self.ssh_client.connect(self.hostname, username=self.username)
            
            self.sftp_client = self.ssh_client.open_sftp()
            logger.info(f"Connected to {self.hostname}")
            return True
            
        except Exception as e:
            logger.error(f"Failed to connect to {self.hostname}: {e}")
            return False
    
    def disconnect(self):
        """Close SSH connection"""
        if self.sftp_client:
            self.sftp_client.close()
        if self.ssh_client:
            self.ssh_client.close()
    
    def discover_wav_files(self) -> List[TestFile]:
        """Discover and categorize WAV files on ridgepoint Pi"""
        if not self.ssh_client:
            raise Exception("Not connected to ridgepoint Pi")
        
        logger.info("Discovering WAV files on ridgepoint Pi...")
        
        # Execute find command to get all WAV files with sizes
        stdin, stdout, stderr = self.ssh_client.exec_command(
            "find /home/gaius/data -name '*.wav' -type f -exec ls -l {} \\; | "
            "awk '{print $9 \"|\" $5}' | grep -v '^\\s*$'"
        )
        
        files = []
        for line in stdout:
            line = line.strip()
            if '|' in line:
                path, size_str = line.split('|', 1)
                try:
                    size = int(size_str)
                    if size > 0:  # Skip empty files
                        file_name = os.path.basename(path)
                        category = self._categorize_file(size)
                        
                        test_file = TestFile(
                            name=file_name,
                            path=path,
                            size=size,
                            category=category,
                            remote_path=path
                        )
                        files.append(test_file)
                        
                except ValueError:
                    continue
        
        logger.info(f"Discovered {len(files)} WAV files")
        self._log_file_distribution(files)
        return files
    
    def _categorize_file(self, size: int) -> str:
        """Categorize file by size"""
        mb = size / (1024 * 1024)
        if mb < 100:
            return "small"
        elif mb < 500:
            return "medium"
        elif mb < 1000:
            return "large"
        else:
            return "xlarge"
    
    def _log_file_distribution(self, files: List[TestFile]):
        """Log file distribution by category"""
        categories = {"small": 0, "medium": 0, "large": 0, "xlarge": 0}
        total_size = 0
        
        for file in files:
            categories[file.category] += 1
            total_size += file.size
        
        logger.info("File distribution:")
        for category, count in categories.items():
            logger.info(f"  {category}: {count} files")
        logger.info(f"  Total size: {total_size / (1024**3):.2f} GB")
    
    def read_file_chunk(self, file_path: str, offset: int = 0, size: int = None) -> bytes:
        """Read file chunk from ridgepoint Pi"""
        if not self.sftp_client:
            raise Exception("SFTP client not available")
        
        with self.sftp_client.open(file_path, 'rb') as remote_file:
            if offset > 0:
                remote_file.seek(offset)
            
            if size:
                return remote_file.read(size)
            else:
                return remote_file.read()
    
    def calculate_file_hash(self, file_path: str) -> str:
        """Calculate SHA256 hash of remote file"""
        logger.info(f"Calculating hash for {file_path}")
        
        hasher = hashlib.sha256()
        chunk_size = 1024 * 1024  # 1MB chunks
        
        with self.sftp_client.open(file_path, 'rb') as remote_file:
            while True:
                chunk = remote_file.read(chunk_size)
                if not chunk:
                    break
                hasher.update(chunk)
        
        return hasher.hexdigest()

class SermonUploaderAPI:
    """Interface to the sermon uploader API"""
    
    def __init__(self, base_url: str):
        self.base_url = base_url.rstrip('/')
        self.session = self._create_session()
    
    def _create_session(self) -> requests.Session:
        """Create HTTP session with optimized settings"""
        session = requests.Session()
        
        # Configure retry strategy
        retry_strategy = Retry(
            total=3,
            status_forcelist=[429, 500, 502, 503, 504],
            method_whitelist=["HEAD", "GET", "POST", "PUT", "DELETE", "OPTIONS", "TRACE"],
            backoff_factor=1
        )
        
        adapter = HTTPAdapter(max_retries=retry_strategy, pool_connections=20, pool_maxsize=20)
        session.mount("http://", adapter)
        session.mount("https://", adapter)
        
        # Set timeouts
        session.timeout = (30, 300)  # Connect timeout, read timeout
        
        return session
    
    def health_check(self) -> bool:
        """Check if API is healthy"""
        try:
            response = self.session.get(f"{self.base_url}/api/health")
            return response.status_code == 200
        except Exception as e:
            logger.error(f"Health check failed: {e}")
            return False
    
    def upload_direct(self, file_data: bytes, filename: str) -> Tuple[bool, Dict, float]:
        """Upload file directly via /api/upload"""
        url = f"{self.base_url}/api/upload"
        
        files = {'files': (filename, file_data, 'audio/wav')}
        
        start_time = time.time()
        try:
            response = self.session.post(url, files=files)
            duration = time.time() - start_time
            
            if response.status_code == 200:
                return True, response.json(), duration
            else:
                return False, {"error": f"HTTP {response.status_code}: {response.text}"}, duration
        
        except Exception as e:
            duration = time.time() - start_time
            return False, {"error": str(e)}, duration
    
    def get_presigned_url(self, filename: str) -> Tuple[bool, Dict, float]:
        """Get presigned URL for file upload"""
        url = f"{self.base_url}/api/upload/presigned"
        
        payload = {"filename": filename}
        
        start_time = time.time()
        try:
            response = self.session.post(url, json=payload)
            duration = time.time() - start_time
            
            if response.status_code == 200:
                return True, response.json(), duration
            else:
                return False, {"error": f"HTTP {response.status_code}: {response.text}"}, duration
        
        except Exception as e:
            duration = time.time() - start_time
            return False, {"error": str(e)}, duration
    
    def upload_presigned(self, presigned_url: str, file_data: bytes) -> Tuple[bool, Dict, float]:
        """Upload file using presigned URL"""
        start_time = time.time()
        try:
            response = self.session.put(
                presigned_url, 
                data=file_data,
                headers={'Content-Type': 'audio/wav'}
            )
            duration = time.time() - start_time
            
            if response.status_code in [200, 204]:
                return True, {"message": "Upload successful"}, duration
            else:
                return False, {"error": f"HTTP {response.status_code}: {response.text}"}, duration
        
        except Exception as e:
            duration = time.time() - start_time
            return False, {"error": str(e)}, duration
    
    def get_presigned_urls_batch(self, filenames: List[str]) -> Tuple[bool, Dict, float]:
        """Get batch presigned URLs for multiple files"""
        url = f"{self.base_url}/api/upload/presigned-batch"
        
        payload = {"filenames": filenames}
        
        start_time = time.time()
        try:
            response = self.session.post(url, json=payload)
            duration = time.time() - start_time
            
            if response.status_code == 200:
                return True, response.json(), duration
            else:
                return False, {"error": f"HTTP {response.status_code}: {response.text}"}, duration
        
        except Exception as e:
            duration = time.time() - start_time
            return False, {"error": str(e)}, duration
    
    def check_duplicate(self, file_hash: str) -> Tuple[bool, Dict, float]:
        """Check if file is duplicate"""
        url = f"{self.base_url}/api/check-duplicate"
        
        payload = {"hash": file_hash}
        
        start_time = time.time()
        try:
            response = self.session.post(url, json=payload)
            duration = time.time() - start_time
            
            return response.status_code == 200, response.json(), duration
        
        except Exception as e:
            duration = time.time() - start_time
            return False, {"error": str(e)}, duration

class APIUploadTester:
    """Main testing orchestrator"""
    
    def __init__(self, config_path: str):
        self.config = self._load_config(config_path)
        self.file_manager = RidgepointFileManager(
            hostname=self.config["ridgepoint"]["hostname"],
            username=self.config["ridgepoint"]["username"],
            private_key_path=self.config["ridgepoint"].get("private_key_path")
        )
        self.api = SermonUploaderAPI(self.config["api"]["base_url"])
        self.test_files = []
        self.results = []
    
    def _load_config(self, config_path: str) -> Dict:
        """Load test configuration"""
        with open(config_path, 'r') as f:
            return json.load(f)
    
    def setup(self) -> bool:
        """Setup test environment"""
        logger.info("Setting up test environment...")
        
        # Check API health
        if not self.api.health_check():
            logger.error("API health check failed")
            return False
        
        # Connect to ridgepoint Pi
        if not self.file_manager.connect():
            logger.error("Failed to connect to ridgepoint Pi")
            return False
        
        # Discover test files
        self.test_files = self.file_manager.discover_wav_files()
        if not self.test_files:
            logger.error("No test files found")
            return False
        
        logger.info("Test environment setup complete")
        return True
    
    def teardown(self):
        """Cleanup test environment"""
        self.file_manager.disconnect()
        logger.info("Test environment cleanup complete")
    
    def select_test_files(self, category: str = None, count: int = None) -> List[TestFile]:
        """Select files for testing based on criteria"""
        files = self.test_files
        
        if category:
            files = [f for f in files if f.category == category]
        
        if count:
            files = files[:count]
        
        return files
    
    def test_single_file_upload(self, test_files: List[TestFile], method: str = "direct") -> TestMetrics:
        """Test single file uploads"""
        logger.info(f"Testing single file uploads using {method} method")
        
        results = []
        start_time = datetime.now()
        
        for i, test_file in enumerate(test_files, 1):
            logger.info(f"Testing file {i}/{len(test_files)}: {test_file.name} ({test_file.size / (1024**2):.1f} MB)")
            
            try:
                # Read file from ridgepoint
                file_data = self.file_manager.read_file_chunk(test_file.remote_path)
                
                # Perform upload based on method
                if method == "direct":
                    result = self._test_direct_upload(test_file, file_data)
                elif method == "presigned":
                    result = self._test_presigned_upload(test_file, file_data)
                else:
                    raise ValueError(f"Unknown method: {method}")
                
                results.append(result)
                
                # Log immediate result
                if result.success:
                    logger.info(f"✓ Upload successful: {result.throughput_mbps:.2f} MB/s")
                else:
                    logger.error(f"✗ Upload failed: {result.error}")
            
            except Exception as e:
                logger.error(f"Test failed for {test_file.name}: {e}")
                results.append(UploadResult(
                    test_id=f"single_{method}_{i}",
                    file_name=test_file.name,
                    file_size=test_file.size,
                    method=method,
                    success=False,
                    duration=0,
                    error=str(e)
                ))
        
        end_time = datetime.now()
        return self._calculate_metrics(f"single_{method}", start_time, end_time, results)
    
    def test_batch_upload(self, test_files: List[TestFile], batch_size: int = 5) -> TestMetrics:
        """Test batch uploads using presigned URLs"""
        logger.info(f"Testing batch uploads with batch size {batch_size}")
        
        results = []
        start_time = datetime.now()
        
        # Process files in batches
        for i in range(0, len(test_files), batch_size):
            batch = test_files[i:i+batch_size]
            batch_num = i // batch_size + 1
            
            logger.info(f"Processing batch {batch_num}: {len(batch)} files")
            
            try:
                # Get batch presigned URLs
                filenames = [f.name for f in batch]
                success, response, api_time = self.api.get_presigned_urls_batch(filenames)
                
                if not success:
                    logger.error(f"Failed to get batch presigned URLs: {response}")
                    continue
                
                # Upload files concurrently
                batch_results = self._upload_batch_concurrent(batch, response["urls"], api_time)
                results.extend(batch_results)
            
            except Exception as e:
                logger.error(f"Batch {batch_num} failed: {e}")
        
        end_time = datetime.now()
        return self._calculate_metrics("batch_presigned", start_time, end_time, results)
    
    def test_stress_scenario(self, duration_seconds: int = 300) -> TestMetrics:
        """Test stress scenario simulating Sunday morning uploads"""
        logger.info(f"Running stress test for {duration_seconds} seconds")
        
        results = []
        start_time = datetime.now()
        end_time = start_time + timedelta(seconds=duration_seconds)
        
        # Select mixed file sizes for stress testing
        stress_files = []
        for category in ["small", "medium", "large", "xlarge"]:
            cat_files = [f for f in self.test_files if f.category == category]
            if cat_files:
                stress_files.extend(cat_files[:3])  # 3 files from each category
        
        upload_count = 0
        
        with ThreadPoolExecutor(max_workers=self.config["testing"]["max_concurrent_uploads"]) as executor:
            while datetime.now() < end_time:
                # Submit uploads
                futures = []
                batch = stress_files[:5]  # Use first 5 files cyclically
                
                for test_file in batch:
                    future = executor.submit(self._stress_upload_worker, test_file, upload_count)
                    futures.append(future)
                    upload_count += 1
                    
                    # Small delay between submissions
                    time.sleep(0.1)
                
                # Collect results
                for future in as_completed(futures, timeout=60):
                    try:
                        result = future.result()
                        results.append(result)
                        
                        if result.success:
                            logger.info(f"Stress upload {result.test_id}: {result.throughput_mbps:.2f} MB/s")
                    except Exception as e:
                        logger.error(f"Stress upload failed: {e}")
                
                # Brief pause between batches
                time.sleep(1)
        
        actual_end_time = datetime.now()
        return self._calculate_metrics("stress", start_time, actual_end_time, results)
    
    def _test_direct_upload(self, test_file: TestFile, file_data: bytes) -> UploadResult:
        """Test direct upload method"""
        test_id = f"direct_{int(time.time())}"
        
        success, response, duration = self.api.upload_direct(file_data, test_file.name)
        
        throughput = (test_file.size / (1024 * 1024)) / duration if duration > 0 else 0
        
        return UploadResult(
            test_id=test_id,
            file_name=test_file.name,
            file_size=test_file.size,
            method="direct",
            success=success,
            duration=duration,
            error=response.get("error") if not success else None,
            throughput_mbps=throughput,
            response_data=response,
            api_response_time=duration,
            upload_time=duration
        )
    
    def _test_presigned_upload(self, test_file: TestFile, file_data: bytes) -> UploadResult:
        """Test presigned URL upload method"""
        test_id = f"presigned_{int(time.time())}"
        
        # Get presigned URL
        success, response, api_time = self.api.get_presigned_url(test_file.name)
        if not success:
            return UploadResult(
                test_id=test_id,
                file_name=test_file.name,
                file_size=test_file.size,
                method="presigned",
                success=False,
                duration=api_time,
                error=response.get("error"),
                api_response_time=api_time
            )
        
        # Upload using presigned URL
        presigned_url = response.get("upload_url")
        success, upload_response, upload_time = self.api.upload_presigned(presigned_url, file_data)
        
        total_duration = api_time + upload_time
        throughput = (test_file.size / (1024 * 1024)) / upload_time if upload_time > 0 else 0
        
        return UploadResult(
            test_id=test_id,
            file_name=test_file.name,
            file_size=test_file.size,
            method="presigned",
            success=success,
            duration=total_duration,
            error=upload_response.get("error") if not success else None,
            throughput_mbps=throughput,
            response_data=upload_response,
            api_response_time=api_time,
            upload_time=upload_time
        )
    
    def _upload_batch_concurrent(self, batch: List[TestFile], presigned_urls: Dict, api_time: float) -> List[UploadResult]:
        """Upload batch of files concurrently"""
        results = []
        
        with ThreadPoolExecutor(max_workers=len(batch)) as executor:
            futures = {}
            
            for test_file in batch:
                if test_file.name in presigned_urls:
                    presigned_url = presigned_urls[test_file.name]
                    file_data = self.file_manager.read_file_chunk(test_file.remote_path)
                    
                    future = executor.submit(self.api.upload_presigned, presigned_url, file_data)
                    futures[future] = test_file
            
            for future in as_completed(futures):
                test_file = futures[future]
                try:
                    success, response, upload_time = future.result()
                    throughput = (test_file.size / (1024 * 1024)) / upload_time if upload_time > 0 else 0
                    
                    result = UploadResult(
                        test_id=f"batch_{int(time.time())}_{test_file.name}",
                        file_name=test_file.name,
                        file_size=test_file.size,
                        method="batch_presigned",
                        success=success,
                        duration=upload_time,
                        error=response.get("error") if not success else None,
                        throughput_mbps=throughput,
                        response_data=response,
                        api_response_time=api_time / len(batch),  # Distribute API time
                        upload_time=upload_time
                    )
                    results.append(result)
                    
                except Exception as e:
                    results.append(UploadResult(
                        test_id=f"batch_error_{test_file.name}",
                        file_name=test_file.name,
                        file_size=test_file.size,
                        method="batch_presigned",
                        success=False,
                        duration=0,
                        error=str(e)
                    ))
        
        return results
    
    def _stress_upload_worker(self, test_file: TestFile, upload_id: int) -> UploadResult:
        """Worker function for stress testing"""
        try:
            file_data = self.file_manager.read_file_chunk(test_file.remote_path)
            
            # Randomly choose upload method for stress testing
            import random
            method = random.choice(["direct", "presigned"])
            
            if method == "direct":
                return self._test_direct_upload(test_file, file_data)
            else:
                return self._test_presigned_upload(test_file, file_data)
        
        except Exception as e:
            return UploadResult(
                test_id=f"stress_{upload_id}",
                file_name=test_file.name,
                file_size=test_file.size,
                method="stress",
                success=False,
                duration=0,
                error=str(e)
            )
    
    def _calculate_metrics(self, test_name: str, start_time: datetime, end_time: datetime, results: List[UploadResult]) -> TestMetrics:
        """Calculate comprehensive test metrics"""
        successful_results = [r for r in results if r.success]
        failed_results = [r for r in results if not r.success]
        
        durations = [r.duration for r in results if r.duration > 0]
        api_times = [r.api_response_time for r in results if r.api_response_time and r.api_response_time > 0]
        
        total_bytes = sum(r.file_size for r in successful_results)
        total_duration = (end_time - start_time).total_seconds()
        
        return TestMetrics(
            test_name=test_name,
            start_time=start_time,
            end_time=end_time,
            total_files=len(results),
            successful_uploads=len(successful_results),
            failed_uploads=len(failed_results),
            total_bytes=total_bytes,
            total_duration=total_duration,
            avg_throughput_mbps=sum(r.throughput_mbps for r in successful_results if r.throughput_mbps) / len(successful_results) if successful_results else 0,
            success_rate=(len(successful_results) / len(results) * 100) if results else 0,
            results=results,
            
            # Duration percentiles
            min_duration=min(durations) if durations else 0,
            max_duration=max(durations) if durations else 0,
            p50_duration=statistics.median(durations) if durations else 0,
            p95_duration=statistics.quantiles(durations, n=20)[18] if len(durations) >= 20 else (max(durations) if durations else 0),
            p99_duration=statistics.quantiles(durations, n=100)[98] if len(durations) >= 100 else (max(durations) if durations else 0),
            
            # API response times
            avg_api_response_time=sum(api_times) / len(api_times) if api_times else 0,
            min_api_response_time=min(api_times) if api_times else 0,
            max_api_response_time=max(api_times) if api_times else 0
        )
    
    def generate_report(self, metrics_list: List[TestMetrics], output_file: str = "api_upload_test_report.json"):
        """Generate comprehensive test report"""
        report = {
            "test_summary": {
                "timestamp": datetime.now().isoformat(),
                "total_tests": len(metrics_list),
                "api_endpoint": self.config["api"]["base_url"],
                "ridgepoint_host": self.config["ridgepoint"]["hostname"],
            },
            "test_results": []
        }
        
        for metrics in metrics_list:
            test_result = {
                "test_name": metrics.test_name,
                "duration_seconds": metrics.total_duration,
                "files_tested": metrics.total_files,
                "success_rate_percent": metrics.success_rate,
                "total_data_gb": metrics.total_bytes / (1024**3),
                "avg_throughput_mbps": metrics.avg_throughput_mbps,
                "performance_metrics": {
                    "min_upload_time": metrics.min_duration,
                    "max_upload_time": metrics.max_duration,
                    "median_upload_time": metrics.p50_duration,
                    "p95_upload_time": metrics.p95_duration,
                    "p99_upload_time": metrics.p99_duration,
                    "avg_api_response_time": metrics.avg_api_response_time
                },
                "failed_uploads": [
                    {
                        "file": r.file_name,
                        "error": r.error,
                        "method": r.method
                    } for r in metrics.results if not r.success
                ],
                "detailed_results": [asdict(r) for r in metrics.results]
            }
            report["test_results"].append(test_result)
        
        # Save report
        with open(output_file, 'w') as f:
            json.dump(report, f, indent=2, default=str)
        
        logger.info(f"Test report saved to {output_file}")
        
        # Print summary
        self._print_summary(metrics_list)
    
    def _print_summary(self, metrics_list: List[TestMetrics]):
        """Print test summary to console"""
        print("\n" + "="*80)
        print("API UPLOAD TEST SUMMARY")
        print("="*80)
        
        for metrics in metrics_list:
            print(f"\nTest: {metrics.test_name.upper()}")
            print(f"Duration: {metrics.total_duration:.1f}s")
            print(f"Files: {metrics.successful_uploads}/{metrics.total_files} successful ({metrics.success_rate:.1f}%)")
            print(f"Data: {metrics.total_bytes / (1024**3):.2f} GB")
            print(f"Avg Throughput: {metrics.avg_throughput_mbps:.2f} MB/s")
            print(f"API Response Time: {metrics.avg_api_response_time:.3f}s")
            
            if metrics.failed_uploads > 0:
                print(f"❌ {metrics.failed_uploads} failed uploads")
            else:
                print("✅ All uploads successful")
        
        print("\n" + "="*80)

def main():
    parser = argparse.ArgumentParser(description="API Upload Testing Suite")
    parser.add_argument("--config", default="api_test_config.json", help="Configuration file path")
    parser.add_argument("--test", choices=["single", "batch", "stress", "all"], default="all", help="Test type to run")
    parser.add_argument("--method", choices=["direct", "presigned"], default="presigned", help="Upload method for single tests")
    parser.add_argument("--size", choices=["small", "medium", "large", "xlarge"], help="File size category")
    parser.add_argument("--files", type=int, default=5, help="Number of files to test")
    parser.add_argument("--batch-size", type=int, default=5, help="Batch size for batch tests")
    parser.add_argument("--duration", type=int, default=300, help="Stress test duration in seconds")
    parser.add_argument("--output", default="api_upload_test_report.json", help="Output report file")
    
    args = parser.parse_args()
    
    # Create default config if not exists
    if not os.path.exists(args.config):
        create_default_config(args.config)
        print(f"Created default config at {args.config}. Please update it with your settings.")
        return
    
    # Initialize tester
    tester = APIUploadTester(args.config)
    
    try:
        if not tester.setup():
            logger.error("Failed to setup test environment")
            return 1
        
        metrics_list = []
        
        if args.test in ["single", "all"]:
            test_files = tester.select_test_files(args.size, args.files)
            if test_files:
                metrics = tester.test_single_file_upload(test_files, args.method)
                metrics_list.append(metrics)
        
        if args.test in ["batch", "all"]:
            test_files = tester.select_test_files(args.size, args.files)
            if test_files:
                metrics = tester.test_batch_upload(test_files, args.batch_size)
                metrics_list.append(metrics)
        
        if args.test in ["stress", "all"]:
            metrics = tester.test_stress_scenario(args.duration)
            metrics_list.append(metrics)
        
        # Generate report
        if metrics_list:
            tester.generate_report(metrics_list, args.output)
        else:
            logger.warning("No tests were executed")
        
        return 0
    
    except KeyboardInterrupt:
        logger.info("Test interrupted by user")
        return 1
    except Exception as e:
        logger.error(f"Test failed: {e}")
        return 1
    finally:
        tester.teardown()

def create_default_config(config_path: str):
    """Create default configuration file"""
    config = {
        "api": {
            "base_url": "http://localhost:8000",
            "timeout": 300
        },
        "ridgepoint": {
            "hostname": "ridgepoint.local",
            "username": "gaius",
            "private_key_path": null
        },
        "testing": {
            "max_concurrent_uploads": 5,
            "chunk_size": 1048576,
            "retry_attempts": 3
        }
    }
    
    with open(config_path, 'w') as f:
        json.dump(config, f, indent=2)

if __name__ == "__main__":
    sys.exit(main())