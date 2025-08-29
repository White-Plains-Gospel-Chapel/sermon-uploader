#!/usr/bin/env python3
"""
Test script to validate Pi processor components
"""

def test_imports():
    """Test all required imports"""
    print("Testing Pi processor imports...")
    
    try:
        from minio import Minio
        print("✓ minio imported")
    except ImportError as e:
        print(f"✗ minio failed: {e}")
        return False
        
    try:
        import requests
        print("✓ requests imported")
    except ImportError as e:
        print(f"✗ requests failed: {e}")
        return False
        
    try:
        import subprocess
        print("✓ subprocess imported")
    except ImportError as e:
        print(f"✗ subprocess failed: {e}")
        return False
        
    try:
        from dotenv import load_dotenv
        print("✓ dotenv imported")
    except ImportError as e:
        print(f"✗ dotenv failed: {e}")
        return False
        
    print("All imports successful!")
    return True

def test_ffmpeg():
    """Test FFmpeg installation"""
    print("\nTesting FFmpeg...")
    
    try:
        import subprocess
        result = subprocess.run(['ffmpeg', '-version'], capture_output=True, text=True)
        if result.returncode == 0:
            print("✓ FFmpeg is installed and working")
            return True
        else:
            print("✗ FFmpeg returned error code")
            return False
    except FileNotFoundError:
        print("✗ FFmpeg not found in PATH")
        return False
    except Exception as e:
        print(f"✗ FFmpeg test failed: {e}")
        return False

def test_minio_connection():
    """Test MinIO connection"""
    print("\nTesting MinIO connection...")
    
    try:
        from minio import Minio
        
        # Test connection
        client = Minio(
            "localhost:9000",  # Pi uses localhost
            access_key="gaius",
            secret_key="John 3:16",
            secure=False
        )
        
        # Try to list buckets
        buckets = list(client.list_buckets())
        print(f"✓ MinIO connection successful. Found {len(buckets)} buckets.")
        
        # Check if sermons bucket exists
        if client.bucket_exists("sermons"):
            print("✓ 'sermons' bucket exists")
        else:
            print("! 'sermons' bucket does not exist - will be created")
            
        return True
        
    except Exception as e:
        print(f"✗ MinIO connection failed: {e}")
        print("  Make sure MinIO is running: docker run -d --name minio -p 9000:9000 -p 9001:9001 -e MINIO_ROOT_USER=gaius -e MINIO_ROOT_PASSWORD='John 3:16' -v minio_data:/data minio/minio:latest server /data --console-address ':9001'")
        return False

def test_conversion():
    """Test audio conversion capability"""
    print("\nTesting audio conversion...")
    
    try:
        import subprocess
        import tempfile
        import os
        
        # Create a simple test audio file (1 second of silence)
        temp_dir = tempfile.mkdtemp()
        wav_file = os.path.join(temp_dir, "test.wav")
        aac_file = os.path.join(temp_dir, "test.aac")
        
        # Generate a 1-second silent WAV file
        cmd_gen = [
            'ffmpeg', '-f', 'lavfi', '-i', 'anullsrc=channel_layout=stereo:sample_rate=48000',
            '-t', '1', '-y', wav_file
        ]
        
        result = subprocess.run(cmd_gen, capture_output=True)
        if result.returncode != 0:
            print("✗ Failed to generate test WAV file")
            return False
            
        # Convert WAV to AAC
        cmd_conv = [
            'ffmpeg', '-i', wav_file, '-c:a', 'aac', '-b:a', '320k',
            '-ar', '48000', '-ac', '2', '-y', aac_file
        ]
        
        result = subprocess.run(cmd_conv, capture_output=True)
        if result.returncode == 0 and os.path.exists(aac_file):
            print("✓ Audio conversion working")
            # Cleanup
            os.remove(wav_file)
            os.remove(aac_file)
            os.rmdir(temp_dir)
            return True
        else:
            print("✗ Audio conversion failed")
            return False
            
    except Exception as e:
        print(f"✗ Conversion test failed: {e}")
        return False

if __name__ == "__main__":
    print("=" * 50)
    print("PI PROCESSOR - SYSTEM TEST")
    print("=" * 50)
    
    all_passed = True
    
    all_passed &= test_imports()
    all_passed &= test_ffmpeg()
    all_passed &= test_minio_connection()
    all_passed &= test_conversion()
    
    print("\n" + "=" * 50)
    if all_passed:
        print("✓ ALL TESTS PASSED - Pi processor ready!")
    else:
        print("✗ SOME TESTS FAILED - Check errors above")
    print("=" * 50)