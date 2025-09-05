#!/usr/bin/env python3
"""
Enhanced Sermon Processor with Live Discord Updates
Monitors MinIO bucket for new WAV files and converts them to AAC with live progress tracking
"""

import os
import sys
import json
import time
import threading
import subprocess
import hashlib
import requests
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional
from dataclasses import dataclass
import logging
from minio import Minio
from minio.error import S3Error
from dotenv import load_dotenv
import tempfile
import shutil

# Add parent directory to path for shared modules
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from shared.discord_live_notifier import DiscordLiveNotifier, SermonPipelineTracker, ProcessingStage

# Load environment variables
load_dotenv()

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('/var/log/sermon_processor.log'),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger(__name__)


@dataclass
class Config:
    """Configuration for the Pi processor"""
    # MinIO Configuration
    minio_endpoint: str = "localhost:9000"  # Local MinIO on Pi
    minio_access_key: str = "your-access-key"
    minio_secret_key: str = "your-secret-key"
    minio_secure: bool = False
    bucket_name: str = "sermons"
    
    # Discord Configuration
    discord_webhook_url: str = "https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN"
    
    # Processing Configuration
    aac_bitrate: str = "320k"  # Highest quality streaming
    aac_codec: str = "aac"
    wav_suffix: str = "_raw"
    aac_suffix: str = "_streamable"
    
    # Monitoring
    check_interval: int = 30  # Check for new files every 30 seconds
    temp_dir: str = "/tmp/sermon_processing"
    
    @classmethod
    def from_env(cls):
        """Load configuration from environment variables"""
        return cls(
            minio_endpoint=os.getenv('MINIO_ENDPOINT', cls.minio_endpoint),
            minio_access_key=os.getenv('MINIO_ACCESS_KEY', cls.minio_access_key),
            minio_secret_key=os.getenv('MINIO_SECRET_KEY', cls.minio_secret_key),
            minio_secure=os.getenv('MINIO_SECURE', 'false').lower() == 'true',
            bucket_name=os.getenv('MINIO_BUCKET', cls.bucket_name),
            discord_webhook_url=os.getenv('DISCORD_WEBHOOK_URL', cls.discord_webhook_url),
            aac_bitrate=os.getenv('AAC_BITRATE', cls.aac_bitrate),
            check_interval=int(os.getenv('CHECK_INTERVAL', str(cls.check_interval))),
        )


class SermonProcessorLive:
    """Main processor class with live Discord updates"""
    
    def __init__(self):
        self.config = Config.from_env()
        
        # Initialize Discord notifier with live updates
        self.notifier = DiscordLiveNotifier(self.config.discord_webhook_url)
        self.tracker = SermonPipelineTracker(self.notifier)
        
        # Initialize MinIO client
        self.minio_client = None
        self.connect_to_minio()
        
        # Track processed files
        self.processed_files = set()
        self.load_processed_files()
        
        # Create temp directory
        os.makedirs(self.config.temp_dir, exist_ok=True)
        
        # Processing flag
        self.running = False
        
        # Current batch tracking
        self.current_batch_id = None
        
    def connect_to_minio(self):
        """Connect to MinIO server"""
        try:
            self.minio_client = Minio(
                self.config.minio_endpoint,
                access_key=self.config.minio_access_key,
                secret_key=self.config.minio_secret_key,
                secure=self.config.minio_secure
            )
            
            if not self.minio_client.bucket_exists(self.config.bucket_name):
                logger.error(f"Bucket {self.config.bucket_name} does not exist")
                return False
                
            logger.info(f"Connected to MinIO bucket: {self.config.bucket_name}")
            return True
            
        except Exception as e:
            logger.error(f"Failed to connect to MinIO: {e}")
            return False
            
    def load_processed_files(self):
        """Load list of already processed files"""
        try:
            # Check for existing AAC files in bucket
            objects = self.minio_client.list_objects(
                self.config.bucket_name,
                prefix="aac/",
                recursive=True
            )
            
            for obj in objects:
                if obj.object_name.endswith('.aac'):
                    # Extract original filename from AAC filename
                    filename = os.path.basename(obj.object_name)
                    # Remove _streamable.aac and add back _raw.wav
                    original = filename.replace(f"{self.config.aac_suffix}.aac", f"{self.config.wav_suffix}.wav")
                    self.processed_files.add(original)
                    
            logger.info(f"Loaded {len(self.processed_files)} processed files")
            
        except Exception as e:
            logger.error(f"Failed to load processed files: {e}")
            
    def get_pending_files(self) -> List[str]:
        """Get list of WAV files that haven't been processed yet"""
        pending = []
        
        try:
            # List all WAV files
            objects = self.minio_client.list_objects(
                self.config.bucket_name,
                prefix="wav/",
                recursive=True
            )
            
            for obj in objects:
                if obj.object_name.endswith('.wav'):
                    filename = os.path.basename(obj.object_name)
                    if filename not in self.processed_files:
                        pending.append(obj.object_name)
                        
        except Exception as e:
            logger.error(f"Failed to get pending files: {e}")
            
        return pending
        
    def convert_wav_to_aac(self, input_path: str, output_path: str) -> bool:
        """Convert WAV file to AAC using FFmpeg"""
        try:
            # Build FFmpeg command for highest quality AAC
            cmd = [
                'ffmpeg',
                '-i', input_path,
                '-c:a', self.config.aac_codec,
                '-b:a', self.config.aac_bitrate,
                '-ar', '48000',  # Sample rate
                '-ac', '2',      # Stereo
                '-movflags', '+faststart',  # Optimize for streaming
                '-y',            # Overwrite output
                output_path
            ]
            
            # Run conversion
            result = subprocess.run(
                cmd,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
                timeout=600  # 10 minute timeout
            )
            
            if result.returncode != 0:
                logger.error(f"FFmpeg error: {result.stderr}")
                return False
                
            return True
            
        except subprocess.TimeoutExpired:
            logger.error("FFmpeg conversion timed out")
            return False
        except Exception as e:
            logger.error(f"Failed to convert file: {e}")
            return False
            
    def update_metadata(self, wav_filename: str, aac_filename: str):
        """Update metadata JSON with AAC filename"""
        try:
            # Download existing metadata
            metadata_key = f"metadata/{wav_filename}.json"
            
            # Create temp file for metadata
            temp_metadata = os.path.join(self.config.temp_dir, "metadata.json")
            
            try:
                self.minio_client.fget_object(
                    self.config.bucket_name,
                    metadata_key,
                    temp_metadata
                )
                
                # Load and update metadata
                with open(temp_metadata, 'r') as f:
                    metadata = json.load(f)
                    
            except S3Error:
                # Metadata doesn't exist, create new
                metadata = {
                    'original_filename': wav_filename,
                    'ai_analysis': {}
                }
                
            # Update with AAC info
            metadata['aac_filename'] = aac_filename
            metadata['aac_conversion_date'] = datetime.now().isoformat()
            metadata['ai_analysis']['aac_filename'] = aac_filename
            metadata['ai_analysis']['processing_status'] = 'aac_converted'
            
            # Save updated metadata
            with open(temp_metadata, 'w') as f:
                json.dump(metadata, f, indent=2)
                
            # Upload back to MinIO
            self.minio_client.fput_object(
                self.config.bucket_name,
                metadata_key,
                temp_metadata,
                content_type='application/json'
            )
            
            # Cleanup
            os.remove(temp_metadata)
            
        except Exception as e:
            logger.error(f"Failed to update metadata: {e}")
            
    def process_file(self, wav_object_name: str) -> bool:
        """Process a single WAV file with live updates"""
        filename = os.path.basename(wav_object_name)
        logger.info(f"Processing: {filename}")
        
        # Update Discord - mark as processing
        if self.current_batch_id:
            self.tracker.mark_file_processing(self.current_batch_id, filename)
        
        # Generate AAC filename
        aac_filename = filename.replace(f"{self.config.wav_suffix}.wav", f"{self.config.aac_suffix}.aac")
        
        # Create temp paths
        temp_wav = os.path.join(self.config.temp_dir, filename)
        temp_aac = os.path.join(self.config.temp_dir, aac_filename)
        
        try:
            # Download WAV file
            logger.info(f"Downloading: {filename}")
            self.minio_client.fget_object(
                self.config.bucket_name,
                wav_object_name,
                temp_wav
            )
            
            # Get original file size
            original_size = os.path.getsize(temp_wav)
            
            # Convert to AAC
            logger.info(f"Converting: {filename} -> {aac_filename}")
            start_time = time.time()
            
            if not self.convert_wav_to_aac(temp_wav, temp_aac):
                raise Exception("Conversion failed")
                
            conversion_time = time.time() - start_time
            
            # Get converted file size
            converted_size = os.path.getsize(temp_aac)
            size_reduction = ((original_size - converted_size) / original_size) * 100
            
            # Upload AAC file
            logger.info(f"Uploading: {aac_filename}")
            self.minio_client.fput_object(
                self.config.bucket_name,
                f"aac/{aac_filename}",
                temp_aac,
                content_type='audio/aac'
            )
            
            # Update metadata
            self.update_metadata(filename, aac_filename)
            
            # Mark as processed
            self.processed_files.add(filename)
            
            # Update Discord - mark as complete
            if self.current_batch_id:
                self.tracker.mark_file_complete(
                    self.current_batch_id, 
                    filename,
                    duration=conversion_time,
                    size_reduction=size_reduction
                )
            
            logger.info(f"Successfully processed: {filename} (Size reduced by {size_reduction:.1f}%)")
            return True
            
        except Exception as e:
            logger.error(f"Failed to process {filename}: {e}")
            
            # Update Discord - mark as error
            if self.current_batch_id:
                self.tracker.mark_file_error(self.current_batch_id, filename, str(e))
            
            return False
            
        finally:
            # Cleanup temp files
            for temp_file in [temp_wav, temp_aac]:
                if os.path.exists(temp_file):
                    try:
                        os.remove(temp_file)
                    except Exception as e:
                        logger.warning(f"Failed to remove temp file {temp_file}: {e}")
                        
    def process_pending_files(self):
        """Process all pending files with live batch tracking"""
        pending = self.get_pending_files()
        
        if not pending:
            logger.debug("No pending files to process")
            return
            
        logger.info(f"Found {len(pending)} files to process")
        
        # Start a new batch in Discord
        filenames = [os.path.basename(f) for f in pending]
        self.current_batch_id = self.tracker.start_batch_upload(filenames)
        
        # Process each file
        for wav_object in pending:
            if not self.running:
                break
                
            self.process_file(wav_object)
            
            # Small delay between files to avoid overloading
            time.sleep(2)
        
        # Complete the batch
        if self.current_batch_id:
            self.tracker.complete_batch(self.current_batch_id)
            self.current_batch_id = None
            
    def monitor_loop(self):
        """Main monitoring loop"""
        logger.info("Starting monitoring loop")
        
        # Send startup notification
        pending = self.get_pending_files()
        self.notifier.send_simple_notification(
            title="🚀 Sermon Processor Started",
            description="Pi processor is monitoring for new files",
            color=0x3498db,
            fields=[
                {"name": "Pending Files", "value": str(len(pending)), "inline": True},
                {"name": "Check Interval", "value": f"{self.config.check_interval} seconds", "inline": True},
            ]
        )
        
        while self.running:
            try:
                self.process_pending_files()
                
                # Clean up old Discord messages periodically
                self.notifier.cleanup_old_messages(hours=48)
                
                # Wait before next check
                for _ in range(self.config.check_interval):
                    if not self.running:
                        break
                    time.sleep(1)
                    
            except Exception as e:
                logger.error(f"Error in monitoring loop: {e}")
                time.sleep(10)  # Wait a bit before retrying
                
        logger.info("Monitoring loop stopped")
        
    def start(self):
        """Start the processor"""
        if self.running:
            logger.warning("Processor is already running")
            return
            
        if not self.minio_client:
            logger.error("Not connected to MinIO")
            return
            
        self.running = True
        
        # Start monitoring in separate thread
        monitor_thread = threading.Thread(target=self.monitor_loop)
        monitor_thread.daemon = True
        monitor_thread.start()
        
        logger.info("Sermon processor started")
        
    def stop(self):
        """Stop the processor"""
        logger.info("Stopping processor...")
        self.running = False
        
    def cleanup(self):
        """Cleanup resources"""
        # Clean temp directory
        if os.path.exists(self.config.temp_dir):
            try:
                shutil.rmtree(self.config.temp_dir)
            except Exception as e:
                logger.warning(f"Failed to clean temp directory: {e}")
                
        # Close session
        if hasattr(self, 'notifier') and self.notifier.session:
            self.notifier.session.close()


def main():
    """Main entry point"""
    processor = SermonProcessorLive()
    
    try:
        processor.start()
        
        # Keep running until interrupted
        while True:
            time.sleep(1)
            
    except KeyboardInterrupt:
        logger.info("Received interrupt signal")
    except Exception as e:
        logger.error(f"Unexpected error: {e}")
    finally:
        processor.stop()
        processor.cleanup()
        logger.info("Processor stopped")


if __name__ == "__main__":
    # Check if FFmpeg is installed
    try:
        subprocess.run(['ffmpeg', '-version'], capture_output=True, check=True)
    except (subprocess.CalledProcessError, FileNotFoundError):
        print("Error: FFmpeg is not installed or not in PATH")
        print("Please install FFmpeg: sudo apt-get install ffmpeg")
        sys.exit(1)
        
    main()