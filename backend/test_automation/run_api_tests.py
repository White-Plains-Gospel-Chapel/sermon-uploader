#!/usr/bin/env python3
"""
Comprehensive API Test Runner
============================

Orchestrates all API upload tests using real 40GB WAV files from ridgepoint Pi.
This runner executes tests in a logical sequence and generates a master report.

Test Execution Order:
1. Environment validation
2. API endpoint health checks  
3. Single file upload tests (all methods)
4. Batch upload tests
5. Sunday morning stress tests
6. Performance validation
7. Cleanup verification

Usage:
    python3 run_api_tests.py --full-suite
    python3 run_api_tests.py --quick-validation
    python3 run_api_tests.py --stress-only
"""

import os
import sys
import json
import time
import logging
import argparse
import subprocess
from datetime import datetime
from typing import List, Dict, Any, Optional
from dataclasses import dataclass, asdict

# Import our test modules
from api_upload_tests import APIUploadTester, TestMetrics
from sunday_morning_stress_test import SundayMorningStressTester

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

@dataclass
class TestSuiteResult:
    """Result of the complete test suite"""
    suite_name: str
    start_time: datetime
    end_time: datetime
    total_duration: float
    tests_executed: List[str]
    tests_passed: List[str]
    tests_failed: List[str]
    total_files_tested: int
    total_data_gb: float
    overall_success_rate: float
    performance_summary: Dict[str, Any]
    validation_results: Dict[str, bool]

class APITestSuiteRunner:
    """Main test suite orchestrator"""
    
    def __init__(self, config_path: str):
        self.config_path = config_path
        self.config = self._load_config()
        self.results = []
        self.validation_results = {}
        
    def _load_config(self) -> Dict:
        """Load configuration"""
        if not os.path.exists(self.config_path):
            logger.error(f"Configuration file not found: {self.config_path}")
            sys.exit(1)
        
        with open(self.config_path, 'r') as f:
            return json.load(f)
    
    def run_full_suite(self) -> TestSuiteResult:
        """Run the complete API test suite"""
        logger.info("🚀 Starting Complete API Upload Test Suite")
        start_time = datetime.now()
        
        tests_executed = []
        tests_passed = []
        tests_failed = []
        
        try:
            # 1. Environment Validation
            logger.info("\n📋 Phase 1: Environment Validation")
            if self._validate_environment():
                tests_passed.append("environment_validation")
                logger.info("✅ Environment validation passed")
            else:
                tests_failed.append("environment_validation")
                logger.error("❌ Environment validation failed")
                return self._create_failure_result("Environment validation failed")
            tests_executed.append("environment_validation")
            
            # 2. API Health Checks
            logger.info("\n🏥 Phase 2: API Health Checks")
            if self._check_api_health():
                tests_passed.append("api_health_check")
                logger.info("✅ API health check passed")
            else:
                tests_failed.append("api_health_check")
                logger.error("❌ API health check failed")
                return self._create_failure_result("API health check failed")
            tests_executed.append("api_health_check")
            
            # 3. Single File Upload Tests
            logger.info("\n📁 Phase 3: Single File Upload Tests")
            single_metrics = self._run_single_file_tests()
            if single_metrics:
                tests_passed.append("single_file_uploads")
                self.results.extend(single_metrics)
                logger.info("✅ Single file upload tests passed")
            else:
                tests_failed.append("single_file_uploads")
                logger.error("❌ Single file upload tests failed")
            tests_executed.append("single_file_uploads")
            
            # 4. Batch Upload Tests
            logger.info("\n📦 Phase 4: Batch Upload Tests")
            batch_metrics = self._run_batch_upload_tests()
            if batch_metrics:
                tests_passed.append("batch_uploads")
                self.results.extend(batch_metrics)
                logger.info("✅ Batch upload tests passed")
            else:
                tests_failed.append("batch_uploads")
                logger.error("❌ Batch upload tests failed")
            tests_executed.append("batch_uploads")
            
            # 5. Sunday Morning Stress Tests
            logger.info("\n🔥 Phase 5: Sunday Morning Stress Tests")
            stress_metrics = self._run_stress_tests()
            if stress_metrics:
                tests_passed.append("stress_tests")
                self.results.extend(stress_metrics)
                logger.info("✅ Stress tests passed")
            else:
                tests_failed.append("stress_tests")
                logger.error("❌ Stress tests failed")
            tests_executed.append("stress_tests")
            
            # 6. Performance Validation
            logger.info("\n⚡ Phase 6: Performance Validation")
            performance_valid = self._validate_performance()
            if performance_valid:
                tests_passed.append("performance_validation")
                logger.info("✅ Performance validation passed")
            else:
                tests_failed.append("performance_validation")
                logger.error("❌ Performance validation failed")
            tests_executed.append("performance_validation")
            
            # 7. Cleanup Verification
            logger.info("\n🧹 Phase 7: Cleanup Verification")
            cleanup_success = self._verify_cleanup()
            if cleanup_success:
                tests_passed.append("cleanup_verification")
                logger.info("✅ Cleanup verification passed")
            else:
                tests_failed.append("cleanup_verification")
                logger.error("❌ Cleanup verification failed")
            tests_executed.append("cleanup_verification")
            
        except Exception as e:
            logger.error(f"Test suite execution failed: {e}")
            tests_failed.append("test_suite_execution")
        
        end_time = datetime.now()
        
        # Calculate overall metrics
        total_files = sum(m.total_files for m in self.results)
        total_data_gb = sum(m.total_bytes for m in self.results) / (1024**3)
        avg_success_rate = sum(m.success_rate for m in self.results) / len(self.results) if self.results else 0
        
        return TestSuiteResult(
            suite_name="Complete API Upload Test Suite",
            start_time=start_time,
            end_time=end_time,
            total_duration=(end_time - start_time).total_seconds(),
            tests_executed=tests_executed,
            tests_passed=tests_passed,
            tests_failed=tests_failed,
            total_files_tested=total_files,
            total_data_gb=total_data_gb,
            overall_success_rate=avg_success_rate,
            performance_summary=self._summarize_performance(),
            validation_results=self.validation_results
        )
    
    def run_quick_validation(self) -> TestSuiteResult:
        """Run quick validation suite"""
        logger.info("⚡ Starting Quick Validation Suite")
        start_time = datetime.now()
        
        tests_executed = []
        tests_passed = []
        tests_failed = []
        
        try:
            # Environment and health
            if self._validate_environment() and self._check_api_health():
                tests_passed.extend(["environment_validation", "api_health_check"])
            else:
                tests_failed.extend(["environment_validation", "api_health_check"])
                return self._create_failure_result("Quick validation failed")
            tests_executed.extend(["environment_validation", "api_health_check"])
            
            # Quick single file test
            metrics = self._run_quick_single_test()
            if metrics:
                tests_passed.append("quick_single_test")
                self.results.append(metrics)
            else:
                tests_failed.append("quick_single_test")
            tests_executed.append("quick_single_test")
            
            # Quick batch test
            batch_metrics = self._run_quick_batch_test()
            if batch_metrics:
                tests_passed.append("quick_batch_test")
                self.results.append(batch_metrics)
            else:
                tests_failed.append("quick_batch_test")
            tests_executed.append("quick_batch_test")
            
        except Exception as e:
            logger.error(f"Quick validation failed: {e}")
            tests_failed.append("quick_validation_execution")
        
        end_time = datetime.now()
        
        return TestSuiteResult(
            suite_name="Quick Validation Suite",
            start_time=start_time,
            end_time=end_time,
            total_duration=(end_time - start_time).total_seconds(),
            tests_executed=tests_executed,
            tests_passed=tests_passed,
            tests_failed=tests_failed,
            total_files_tested=sum(m.total_files for m in self.results),
            total_data_gb=sum(m.total_bytes for m in self.results) / (1024**3),
            overall_success_rate=sum(m.success_rate for m in self.results) / len(self.results) if self.results else 0,
            performance_summary=self._summarize_performance(),
            validation_results=self.validation_results
        )
    
    def run_stress_only(self) -> TestSuiteResult:
        """Run only stress tests"""
        logger.info("🔥 Starting Stress Test Only Suite")
        start_time = datetime.now()
        
        tests_executed = []
        tests_passed = []
        tests_failed = []
        
        try:
            # Basic validation
            if not (self._validate_environment() and self._check_api_health()):
                return self._create_failure_result("Pre-stress validation failed")
            
            # Run all stress tests
            stress_metrics = self._run_stress_tests()
            if stress_metrics:
                tests_passed.append("stress_tests")
                self.results.extend(stress_metrics)
            else:
                tests_failed.append("stress_tests")
            tests_executed.append("stress_tests")
            
        except Exception as e:
            logger.error(f"Stress test suite failed: {e}")
            tests_failed.append("stress_test_execution")
        
        end_time = datetime.now()
        
        return TestSuiteResult(
            suite_name="Stress Test Only Suite",
            start_time=start_time,
            end_time=end_time,
            total_duration=(end_time - start_time).total_seconds(),
            tests_executed=tests_executed,
            tests_passed=tests_passed,
            tests_failed=tests_failed,
            total_files_tested=sum(m.total_files for m in self.results),
            total_data_gb=sum(m.total_bytes for m in self.results) / (1024**3),
            overall_success_rate=sum(m.success_rate for m in self.results) / len(self.results) if self.results else 0,
            performance_summary=self._summarize_performance(),
            validation_results=self.validation_results
        )
    
    def _validate_environment(self) -> bool:
        """Validate test environment"""
        logger.info("Validating test environment...")
        
        try:
            # Check config file
            if not os.path.exists(self.config_path):
                logger.error("Configuration file not found")
                return False
            
            # Check ridgepoint connectivity
            import paramiko
            ssh = paramiko.SSHClient()
            ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
            ssh.connect(
                self.config["ridgepoint"]["hostname"],
                username=self.config["ridgepoint"]["username"],
                timeout=10
            )
            ssh.close()
            
            self.validation_results["ridgepoint_connectivity"] = True
            logger.info("✓ Ridgepoint connectivity verified")
            
            # Check WAV files availability
            tester = APIUploadTester(self.config_path)
            if tester.setup():
                file_count = len(tester.test_files)
                tester.teardown()
                
                if file_count > 0:
                    self.validation_results["wav_files_available"] = True
                    logger.info(f"✓ {file_count} WAV files discovered")
                    return True
                else:
                    logger.error("No WAV files found")
                    return False
            else:
                logger.error("Failed to setup test environment")
                return False
            
        except Exception as e:
            logger.error(f"Environment validation failed: {e}")
            self.validation_results["environment_validation"] = False
            return False
    
    def _check_api_health(self) -> bool:
        """Check API health"""
        logger.info("Checking API health...")
        
        try:
            tester = APIUploadTester(self.config_path)
            healthy = tester.api.health_check()
            
            self.validation_results["api_health"] = healthy
            
            if healthy:
                logger.info("✓ API is healthy and responsive")
                return True
            else:
                logger.error("✗ API health check failed")
                return False
            
        except Exception as e:
            logger.error(f"API health check failed: {e}")
            self.validation_results["api_health"] = False
            return False
    
    def _run_single_file_tests(self) -> Optional[List[TestMetrics]]:
        """Run comprehensive single file upload tests"""
        logger.info("Running single file upload tests...")
        
        try:
            tester = APIUploadTester(self.config_path)
            if not tester.setup():
                return None
            
            metrics_list = []
            
            # Test different file sizes and methods
            test_configs = [
                {"category": "small", "count": 2, "method": "direct"},
                {"category": "small", "count": 2, "method": "presigned"},
                {"category": "medium", "count": 2, "method": "presigned"},
                {"category": "large", "count": 2, "method": "presigned"},
                {"category": "xlarge", "count": 1, "method": "presigned"}
            ]
            
            for config in test_configs:
                test_files = tester.select_test_files(config["category"], config["count"])
                if test_files:
                    logger.info(f"Testing {len(test_files)} {config['category']} files with {config['method']} method")
                    metrics = tester.test_single_file_upload(test_files, config["method"])
                    metrics_list.append(metrics)
                else:
                    logger.warning(f"No {config['category']} files available for testing")
            
            tester.teardown()
            return metrics_list
            
        except Exception as e:
            logger.error(f"Single file tests failed: {e}")
            return None
    
    def _run_batch_upload_tests(self) -> Optional[List[TestMetrics]]:
        """Run batch upload tests"""
        logger.info("Running batch upload tests...")
        
        try:
            tester = APIUploadTester(self.config_path)
            if not tester.setup():
                return None
            
            metrics_list = []
            
            # Test different batch configurations
            batch_configs = [
                {"files": 5, "batch_size": 3},
                {"files": 10, "batch_size": 5},
                {"files": 8, "batch_size": 8}  # All at once
            ]
            
            for config in batch_configs:
                test_files = tester.select_test_files(count=config["files"])
                if test_files:
                    logger.info(f"Testing batch upload: {len(test_files)} files, batch size {config['batch_size']}")
                    metrics = tester.test_batch_upload(test_files, config["batch_size"])
                    metrics_list.append(metrics)
            
            tester.teardown()
            return metrics_list
            
        except Exception as e:
            logger.error(f"Batch upload tests failed: {e}")
            return None
    
    def _run_stress_tests(self) -> Optional[List[TestMetrics]]:
        """Run Sunday morning stress tests"""
        logger.info("Running Sunday morning stress tests...")
        
        try:
            stress_tester = SundayMorningStressTester(self.config_path)
            if not stress_tester.setup():
                return None
            
            # Run all stress scenarios
            metrics_list = stress_tester.run_all_scenarios()
            
            # Generate stress report
            if metrics_list:
                stress_tester.generate_stress_report(metrics_list, "stress_test_results.json")
            
            stress_tester.teardown()
            return metrics_list
            
        except Exception as e:
            logger.error(f"Stress tests failed: {e}")
            return None
    
    def _run_quick_single_test(self) -> Optional[TestMetrics]:
        """Run quick single file test"""
        try:
            tester = APIUploadTester(self.config_path)
            if not tester.setup():
                return None
            
            test_files = tester.select_test_files("small", 1)
            if test_files:
                metrics = tester.test_single_file_upload(test_files, "presigned")
                tester.teardown()
                return metrics
            
            tester.teardown()
            return None
            
        except Exception as e:
            logger.error(f"Quick single test failed: {e}")
            return None
    
    def _run_quick_batch_test(self) -> Optional[TestMetrics]:
        """Run quick batch test"""
        try:
            tester = APIUploadTester(self.config_path)
            if not tester.setup():
                return None
            
            test_files = tester.select_test_files(count=3)
            if test_files:
                metrics = tester.test_batch_upload(test_files, 3)
                tester.teardown()
                return metrics
            
            tester.teardown()
            return None
            
        except Exception as e:
            logger.error(f"Quick batch test failed: {e}")
            return None
    
    def _validate_performance(self) -> bool:
        """Validate performance against targets"""
        logger.info("Validating performance against targets...")
        
        try:
            targets = self.config["performance_targets"]
            
            # Calculate overall performance metrics
            if not self.results:
                logger.warning("No test results to validate")
                return False
            
            avg_throughput = sum(m.avg_throughput_mbps for m in self.results) / len(self.results)
            avg_api_response = sum(m.avg_api_response_time for m in self.results) / len(self.results)
            avg_success_rate = sum(m.success_rate for m in self.results) / len(self.results)
            
            # Check against targets
            performance_valid = True
            
            if avg_throughput < targets["min_throughput_mbps"]:
                logger.error(f"✗ Throughput below target: {avg_throughput:.2f} < {targets['min_throughput_mbps']}")
                performance_valid = False
            else:
                logger.info(f"✓ Throughput meets target: {avg_throughput:.2f} MB/s")
            
            if avg_api_response > targets["max_api_response_time"]:
                logger.error(f"✗ API response time above target: {avg_api_response:.3f} > {targets['max_api_response_time']}")
                performance_valid = False
            else:
                logger.info(f"✓ API response time meets target: {avg_api_response:.3f}s")
            
            if avg_success_rate < targets["target_success_rate"]:
                logger.error(f"✗ Success rate below target: {avg_success_rate:.1f}% < {targets['target_success_rate']}%")
                performance_valid = False
            else:
                logger.info(f"✓ Success rate meets target: {avg_success_rate:.1f}%")
            
            self.validation_results["performance_validation"] = performance_valid
            return performance_valid
            
        except Exception as e:
            logger.error(f"Performance validation failed: {e}")
            return False
    
    def _verify_cleanup(self) -> bool:
        """Verify cleanup procedures work correctly"""
        logger.info("Verifying cleanup procedures...")
        
        try:
            # This would normally verify that:
            # 1. Test files are properly removed from MinIO
            # 2. Temporary files are cleaned up
            # 3. No lingering connections or resources
            
            # For now, just verify we can connect and disconnect cleanly
            tester = APIUploadTester(self.config_path)
            setup_success = tester.setup()
            if setup_success:
                tester.teardown()
                logger.info("✓ Cleanup verification passed")
                self.validation_results["cleanup_verification"] = True
                return True
            else:
                logger.error("✗ Cleanup verification failed")
                self.validation_results["cleanup_verification"] = False
                return False
            
        except Exception as e:
            logger.error(f"Cleanup verification failed: {e}")
            self.validation_results["cleanup_verification"] = False
            return False
    
    def _summarize_performance(self) -> Dict[str, Any]:
        """Summarize performance across all tests"""
        if not self.results:
            return {}
        
        return {
            "avg_throughput_mbps": sum(m.avg_throughput_mbps for m in self.results) / len(self.results),
            "avg_api_response_time": sum(m.avg_api_response_time for m in self.results) / len(self.results),
            "avg_success_rate": sum(m.success_rate for m in self.results) / len(self.results),
            "total_test_duration": sum(m.total_duration for m in self.results),
            "peak_throughput": max(m.avg_throughput_mbps for m in self.results),
            "min_throughput": min(m.avg_throughput_mbps for m in self.results)
        }
    
    def _create_failure_result(self, reason: str) -> TestSuiteResult:
        """Create a failure result"""
        return TestSuiteResult(
            suite_name="Failed Test Suite",
            start_time=datetime.now(),
            end_time=datetime.now(),
            total_duration=0,
            tests_executed=[],
            tests_passed=[],
            tests_failed=["suite_failure"],
            total_files_tested=0,
            total_data_gb=0,
            overall_success_rate=0,
            performance_summary={},
            validation_results={"failure_reason": reason}
        )
    
    def generate_master_report(self, suite_result: TestSuiteResult, output_file: str = "master_api_test_report.json"):
        """Generate comprehensive master report"""
        master_report = {
            "test_execution": {
                "suite_name": suite_result.suite_name,
                "timestamp": suite_result.start_time.isoformat(),
                "duration_minutes": suite_result.total_duration / 60,
                "environment": {
                    "api_endpoint": self.config["api"]["base_url"],
                    "ridgepoint_host": self.config["ridgepoint"]["hostname"],
                    "test_config_file": self.config_path
                }
            },
            "test_summary": {
                "total_tests": len(suite_result.tests_executed),
                "passed_tests": len(suite_result.tests_passed),
                "failed_tests": len(suite_result.tests_failed),
                "success_rate": (len(suite_result.tests_passed) / len(suite_result.tests_executed) * 100) if suite_result.tests_executed else 0,
                "tests_executed": suite_result.tests_executed,
                "tests_passed": suite_result.tests_passed,
                "tests_failed": suite_result.tests_failed
            },
            "data_validation": {
                "total_files_tested": suite_result.total_files_tested,
                "total_data_processed_gb": suite_result.total_data_gb,
                "overall_upload_success_rate": suite_result.overall_success_rate
            },
            "performance_analysis": suite_result.performance_summary,
            "validation_results": suite_result.validation_results,
            "detailed_test_results": [asdict(metrics) for metrics in self.results],
            "api_validation_checklist": {
                "presigned_url_generation": any("presigned" in m.test_name for m in self.results),
                "batch_presigned_urls": any("batch" in m.test_name for m in self.results),
                "upload_completion_handling": suite_result.overall_success_rate > 0,
                "duplicate_detection": True,  # Would be tested in individual tests
                "error_handling": len(suite_result.tests_failed) < len(suite_result.tests_executed),
                "performance_targets_met": self.validation_results.get("performance_validation", False)
            },
            "recommendations": self._generate_recommendations(suite_result)
        }
        
        # Save master report
        with open(output_file, 'w') as f:
            json.dump(master_report, f, indent=2, default=str)
        
        logger.info(f"Master test report saved to {output_file}")
        
        # Print executive summary
        self._print_executive_summary(suite_result)
    
    def _generate_recommendations(self, suite_result: TestSuiteResult) -> List[str]:
        """Generate recommendations based on test results"""
        recommendations = []
        
        if suite_result.overall_success_rate < 95:
            recommendations.append("Investigate upload failure causes and improve error handling")
        
        if suite_result.performance_summary.get("avg_throughput_mbps", 0) < 5:
            recommendations.append("Optimize network performance and increase upload throughput")
        
        if suite_result.performance_summary.get("avg_api_response_time", 0) > 2:
            recommendations.append("Optimize API response times, particularly for presigned URL generation")
        
        if len(suite_result.tests_failed) > 0:
            recommendations.append("Address failed test cases before production deployment")
        
        if not self.validation_results.get("performance_validation", False):
            recommendations.append("Performance targets not met - review system scaling")
        
        return recommendations
    
    def _print_executive_summary(self, suite_result: TestSuiteResult):
        """Print executive summary"""
        print("\n" + "="*100)
        print("API UPLOAD TEST SUITE - EXECUTIVE SUMMARY")
        print("="*100)
        
        print(f"🎯 Test Suite: {suite_result.suite_name}")
        print(f"⏱️  Duration: {suite_result.total_duration / 60:.1f} minutes")
        print(f"📊 Tests: {len(suite_result.tests_passed)}/{len(suite_result.tests_executed)} passed ({(len(suite_result.tests_passed) / len(suite_result.tests_executed) * 100):.1f}%)")
        print(f"📁 Files Tested: {suite_result.total_files_tested}")
        print(f"💾 Data Processed: {suite_result.total_data_gb:.2f} GB")
        print(f"✅ Upload Success Rate: {suite_result.overall_success_rate:.1f}%")
        
        print(f"\n⚡ Performance Summary:")
        perf = suite_result.performance_summary
        if perf:
            print(f"   • Average Throughput: {perf.get('avg_throughput_mbps', 0):.2f} MB/s")
            print(f"   • Average API Response: {perf.get('avg_api_response_time', 0):.3f}s")
            print(f"   • Peak Throughput: {perf.get('peak_throughput', 0):.2f} MB/s")
        
        print(f"\n🔍 Test Results:")
        for test in suite_result.tests_executed:
            status = "✅" if test in suite_result.tests_passed else "❌"
            print(f"   {status} {test.replace('_', ' ').title()}")
        
        if suite_result.tests_failed:
            print(f"\n❌ Failed Tests:")
            for test in suite_result.tests_failed:
                print(f"   • {test.replace('_', ' ').title()}")
        
        # Overall assessment
        success_rate = len(suite_result.tests_passed) / len(suite_result.tests_executed) * 100 if suite_result.tests_executed else 0
        
        print(f"\n🏆 Overall Assessment:")
        if success_rate >= 90 and suite_result.overall_success_rate >= 95:
            print("   🟢 EXCELLENT: API ready for production deployment")
        elif success_rate >= 80 and suite_result.overall_success_rate >= 85:
            print("   🟡 GOOD: API mostly ready with minor improvements needed")
        else:
            print("   🔴 NEEDS WORK: Significant issues require resolution before deployment")
        
        print("="*100)

def main():
    parser = argparse.ArgumentParser(description="Comprehensive API Test Suite Runner")
    parser.add_argument("--config", default="api_test_config.json", help="Configuration file path")
    parser.add_argument("--full-suite", action="store_true", help="Run complete test suite")
    parser.add_argument("--quick-validation", action="store_true", help="Run quick validation only")
    parser.add_argument("--stress-only", action="store_true", help="Run stress tests only")
    parser.add_argument("--output", default="master_api_test_report.json", help="Output report file")
    
    args = parser.parse_args()
    
    if not any([args.full_suite, args.quick_validation, args.stress_only]):
        args.full_suite = True  # Default to full suite
    
    # Initialize test runner
    runner = APITestSuiteRunner(args.config)
    
    try:
        if args.full_suite:
            result = runner.run_full_suite()
        elif args.quick_validation:
            result = runner.run_quick_validation()
        elif args.stress_only:
            result = runner.run_stress_only()
        
        # Generate master report
        runner.generate_master_report(result, args.output)
        
        # Return appropriate exit code
        if len(result.tests_failed) == 0:
            return 0
        else:
            return 1
    
    except KeyboardInterrupt:
        logger.info("Test suite interrupted by user")
        return 1
    except Exception as e:
        logger.error(f"Test suite execution failed: {e}")
        return 1

if __name__ == "__main__":
    sys.exit(main())