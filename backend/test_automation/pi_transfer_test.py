#!/usr/bin/env python3

import os
import time
import subprocess
import json
from datetime import datetime

# Configuration
API_BASE = 'http://192.168.1.127:8000/api'
PI1_HOST = '192.168.1.195'
PI1_USER = 'gaius'

def escape_path(path):
    """Properly escape file paths for SSH/SCP"""
    return path.replace(' ', '\\ ').replace('(', '\\(').replace(')', '\\)')

def test_file_transfer(file_name, test_name):
    """Test transferring a single file"""
    print(f'\n=== {test_name.upper()} ===')
    print(f'Testing: {file_name}')
    
    try:
        # Step 1: Find the file on Pi1
        find_cmd = f'find /home/gaius/data/sermon-test-wavs -name "{file_name}" -type f'
        find_result = subprocess.run([
            'ssh', f'{PI1_USER}@{PI1_HOST}', find_cmd
        ], capture_output=True, text=True, timeout=30)
        
        if find_result.returncode != 0 or not find_result.stdout.strip():
            print(f'❌ File not found: {file_name}')
            return None
        
        file_path = find_result.stdout.strip().split('\n')[0]  # Take first match
        print(f'✅ Found at: {file_path}')
        
        # Step 2: Get file size
        size_cmd = f'stat -c%s "{file_path}"'
        size_result = subprocess.run([
            'ssh', f'{PI1_USER}@{PI1_HOST}', size_cmd
        ], capture_output=True, text=True, timeout=15)
        
        if size_result.returncode != 0:
            print('❌ Could not get file size')
            return None
        
        file_size = int(size_result.stdout.strip())
        size_mb = file_size / (1024 * 1024)
        print(f'✅ Size: {file_size:,} bytes ({size_mb:.2f} MB)')
        
        # Step 3: Create a simplified copy approach
        # Use SSH to cat the file and redirect locally
        temp_file = f'/tmp/test_{int(time.time())}_{file_name.replace(" ", "_")}'
        print(f'📁 Downloading to: {temp_file}')
        
        download_start = time.time()
        
        # Use SSH with cat to stream the file
        with open(temp_file, 'wb') as f:
            ssh_process = subprocess.Popen([
                'ssh', f'{PI1_USER}@{PI1_HOST}', f'cat "{file_path}"'
            ], stdout=f, stderr=subprocess.PIPE)
            
            ssh_process.wait(timeout=600)  # 10 minute timeout
        
        download_time = time.time() - download_start
        
        if ssh_process.returncode != 0:
            print(f'❌ Download failed: {ssh_process.stderr.read().decode()}')
            return None
        
        # Verify download
        if not os.path.exists(temp_file):
            print('❌ Local file was not created')
            return None
        
        local_size = os.path.getsize(temp_file)
        if local_size != file_size:
            print(f'❌ Size mismatch: expected {file_size}, got {local_size}')
            return None
        
        print(f'✅ Downloaded successfully in {download_time:.2f}s')
        download_speed = size_mb / download_time if download_time > 0 else 0
        print(f'📊 Download speed: {download_speed:.2f} MB/s')
        
        # Step 4: Test the API upload
        print('🚀 Starting API upload...')
        
        # Get baseline system metrics
        metrics_cmd = 'echo "=== BASELINE ===" && free -h && echo "CPU:" && uptime'
        subprocess.run(['ssh', f'{PI1_USER}@192.168.1.127', metrics_cmd], timeout=10)
        
        upload_start = time.time()
        
        upload_result = subprocess.run([
            'curl', 
            '-v',  # Verbose for debugging
            '-w', '\nHTTP_CODE:%{http_code}\nTIME_TOTAL:%{time_total}\nSPEED_UPLOAD:%{speed_upload}\nSIZE_UPLOAD:%{size_upload}\n',
            '-F', f'file=@{temp_file}',
            f'{API_BASE}/upload'
        ], capture_output=True, text=True, timeout=1200)
        
        upload_time = time.time() - upload_start
        
        # Parse curl output
        stdout_lines = upload_result.stdout.split('\n')
        stderr_lines = upload_result.stderr.split('\n')
        
        # Extract metrics from stdout
        metrics = {}
        for line in stdout_lines:
            if ':' in line and line.count(':') == 1:
                key, value = line.split(':', 1)
                if key in ['HTTP_CODE', 'TIME_TOTAL', 'SPEED_UPLOAD', 'SIZE_UPLOAD']:
                    try:
                        if key == 'HTTP_CODE':
                            metrics[key] = int(value)
                        else:
                            metrics[key] = float(value)
                    except ValueError:
                        pass
        
        # Calculate speeds
        actual_speed_mbps = size_mb / upload_time if upload_time > 0 else 0
        curl_speed_mbps = metrics.get('SPEED_UPLOAD', 0) / (1024 * 1024)
        
        print(f'\n📈 UPLOAD RESULTS:')
        print(f'   Return Code: {upload_result.returncode}')
        print(f'   HTTP Status: {metrics.get("HTTP_CODE", "unknown")}')
        print(f'   Upload Time: {upload_time:.2f}s (curl: {metrics.get("TIME_TOTAL", 0):.2f}s)')
        print(f'   File Size: {size_mb:.2f} MB')
        print(f'   Upload Speed: {actual_speed_mbps:.2f} MB/s (curl: {curl_speed_mbps:.2f} MB/s)')
        print(f'   Bytes Uploaded: {metrics.get("SIZE_UPLOAD", 0):,.0f}')
        
        success = (upload_result.returncode == 0 and 
                  metrics.get('HTTP_CODE', 0) in [200, 201])
        
        if success:
            print('   ✅ SUCCESS!')
            
            # Performance evaluation
            if actual_speed_mbps >= 10:
                print('   🚀 EXCELLENT: Speed >= 10 MB/s')
            elif actual_speed_mbps >= 5:
                print('   ✅ GOOD: Speed >= 5 MB/s (target met)')
            else:
                print('   ⚠️ BELOW TARGET: Speed < 5 MB/s')
                
            # Show successful response snippet
            response_text = '\n'.join([line for line in stdout_lines 
                                     if not line.startswith(('HTTP_CODE:', 'TIME_', 'SPEED_', 'SIZE_'))])
            if response_text.strip():
                print(f'   📄 Response preview: {response_text.strip()[:150]}...')
        else:
            print(f'   ❌ FAILED')
            if upload_result.returncode != 0:
                print(f'   Curl error: {upload_result.returncode}')
            
            # Show error details
            error_lines = [line for line in stderr_lines if 'error' in line.lower() or 'failed' in line.lower()]
            if error_lines:
                for line in error_lines[:3]:
                    print(f'   Error: {line}')
        
        # Get final system metrics
        print('\n📊 Final system metrics:')
        final_metrics_cmd = 'echo "=== FINAL ===" && free -h && echo "CPU:" && uptime'
        subprocess.run(['ssh', f'{PI1_USER}@192.168.1.127', final_metrics_cmd], timeout=10)
        
        # Cleanup
        try:
            os.unlink(temp_file)
            print('✅ Temporary file cleaned up')
        except Exception as e:
            print(f'⚠️ Cleanup warning: {e}')
        
        # Return results
        return {
            'test_name': test_name,
            'file_name': file_name,
            'file_size_bytes': file_size,
            'file_size_mb': size_mb,
            'download_time': download_time,
            'download_speed_mbps': download_speed,
            'upload_time': upload_time,
            'upload_speed_mbps': actual_speed_mbps,
            'curl_speed_mbps': curl_speed_mbps,
            'http_code': metrics.get('HTTP_CODE', 0),
            'success': success,
            'timestamp': datetime.now().isoformat()
        }
        
    except subprocess.TimeoutExpired:
        print('❌ Test timed out')
        return {'test_name': test_name, 'error': 'timeout'}
    except Exception as e:
        print(f'❌ Test error: {e}')
        return {'test_name': test_name, 'error': str(e)}

def main():
    """Run the comprehensive test suite"""
    print('🚀 COMPREHENSIVE PI-TO-PI TRANSFER PERFORMANCE TEST')
    print('🔧 Testing optimized MinIO upload performance')
    print('=' * 60)
    
    all_results = []
    
    # Test files in order of complexity
    test_cases = [
        ('medium_test_30sec.wav', 'Small File Test (5MB)'),
        ('large_test_3min.wav', 'Medium File Test (31MB)'),
        ('sermon_60min.wav', 'Large File Test (605MB)'),
    ]
    
    for i, (file_name, test_desc) in enumerate(test_cases):
        result = test_file_transfer(file_name, test_desc)
        if result:
            all_results.append(result)
        
        # Wait between tests to avoid overwhelming the system
        if i < len(test_cases) - 1:
            wait_time = 15 if i == 0 else 30
            print(f'\n⏱️ Waiting {wait_time} seconds before next test...')
            time.sleep(wait_time)
    
    # Generate comprehensive report
    print('\n' + '=' * 60)
    print('📊 COMPREHENSIVE TEST REPORT')
    print('=' * 60)
    
    if not all_results:
        print('❌ No successful tests completed')
        return
    
    successful_tests = [r for r in all_results if r.get('success', False)]
    failed_tests = [r for r in all_results if not r.get('success', False)]
    
    print(f'📈 SUMMARY STATISTICS:')
    print(f'   Total Tests: {len(all_results)}')
    print(f'   Successful: {len(successful_tests)}')
    print(f'   Failed: {len(failed_tests)}')
    print(f'   Success Rate: {len(successful_tests)/len(all_results)*100:.1f}%')
    
    if successful_tests:
        # Performance analysis
        speeds = [r['upload_speed_mbps'] for r in successful_tests]
        sizes = [r['file_size_mb'] for r in successful_tests]
        times = [r['upload_time'] for r in successful_tests]
        
        avg_speed = sum(speeds) / len(speeds)
        max_speed = max(speeds)
        min_speed = min(speeds)
        total_data = sum(sizes)
        total_time = sum(times)
        overall_speed = total_data / total_time if total_time > 0 else 0
        
        print(f'\n🚀 PERFORMANCE METRICS:')
        print(f'   Average Speed: {avg_speed:.2f} MB/s')
        print(f'   Maximum Speed: {max_speed:.2f} MB/s')  
        print(f'   Minimum Speed: {min_speed:.2f} MB/s')
        print(f'   Overall Speed: {overall_speed:.2f} MB/s')
        print(f'   Total Data Transferred: {total_data:.2f} MB')
        print(f'   Total Transfer Time: {total_time:.2f} seconds')
        
        print(f'\n✅ VALIDATION CRITERIA:')
        target_met = avg_speed >= 5.0
        print(f'   Target Speed (≥5 MB/s): {avg_speed:.2f} MB/s - {"✅ MET" if target_met else "❌ NOT MET"}')
        success_rate_met = len(successful_tests)/len(all_results) >= 0.95
        print(f'   Target Success Rate (≥95%): {len(successful_tests)/len(all_results)*100:.1f}% - {"✅ MET" if success_rate_met else "❌ NOT MET"}')
        
        if target_met and success_rate_met:
            print(f'\n🎉 ALL OPTIMIZATION TARGETS ACHIEVED!')
            print(f'   The MinIO optimizations have successfully resolved the upload performance issues.')
        else:
            print(f'\n⚠️ Some targets not met. Further optimization may be needed.')
        
        # Individual test details
        print(f'\n📋 INDIVIDUAL TEST RESULTS:')
        for result in successful_tests:
            print(f'   {result["test_name"]}:')
            print(f'     File: {result["file_name"]} ({result["file_size_mb"]:.1f} MB)')
            print(f'     Speed: {result["upload_speed_mbps"]:.2f} MB/s')
            print(f'     Time: {result["upload_time"]:.2f}s')
            print(f'     Status: {"✅ Success" if result["success"] else "❌ Failed"}')
    
    # Save detailed results
    timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
    results_file = f'comprehensive_test_results_{timestamp}.json'
    with open(results_file, 'w') as f:
        json.dump(all_results, f, indent=2)
    
    print(f'\n📄 Detailed results saved to: {results_file}')
    print('✅ COMPREHENSIVE TESTING COMPLETED')

if __name__ == '__main__':
    main()
