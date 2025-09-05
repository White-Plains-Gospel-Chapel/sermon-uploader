#!/usr/bin/env python3
"""
Enhanced Discord Notifier with Live Message Updates
Supports creating and updating messages for live progress tracking
"""

import json
import time
import threading
try:
    import requests
except ImportError:
    # For testing without requests library
    requests = None
from datetime import datetime, timezone, timedelta
from typing import Dict, Optional, List, Any
from dataclasses import dataclass
from enum import Enum
import logging

logger = logging.getLogger(__name__)


def get_est_time():
    """Get current time in EST/EDT timezone"""
    # EST is UTC-5, EDT is UTC-4. Using fixed offset for consistency
    # You could also use pytz for automatic DST handling if available
    est_offset = timedelta(hours=-5)  # EST (change to -4 for EDT)
    utc_time = datetime.now(timezone.utc)
    est_time = utc_time + est_offset
    return est_time


class ProcessingStage(Enum):
    """Stages in the sermon processing pipeline"""
    DETECTED = "File Detected"
    UPLOADING = "Uploading to MinIO"
    UPLOADED = "Upload Complete"
    PROCESSING = "Converting to AAC"
    COMPLETED = "Processing Complete"
    ERROR = "Error Occurred"


@dataclass
class ProgressMessage:
    """Tracks a Discord message for live updates"""
    message_id: str
    channel_id: str
    webhook_url: str
    title: str
    created_at: datetime
    last_updated: datetime
    stages: Dict[str, ProcessingStage]
    file_info: Dict[str, Any]


class DiscordLiveNotifier:
    """Discord notifier with live message update capability"""
    
    def __init__(self, webhook_url: str):
        self.webhook_url = webhook_url
        self.session = requests.Session() if requests else None
        self.active_messages: Dict[str, ProgressMessage] = {}
        self.message_lock = threading.Lock()
        
        # Parse webhook URL to get webhook ID and token
        parts = webhook_url.split('/')
        if len(parts) >= 2:
            self.webhook_id = parts[-2]
            self.webhook_token = parts[-1]
        else:
            raise ValueError("Invalid webhook URL format")
    
    def create_progress_message(self, title: str, files: List[str]) -> Optional[ProgressMessage]:
        """Create a new progress message for tracking file processing"""
        try:
            # Initialize stages for all files
            stages = {file: ProcessingStage.DETECTED for file in files}
            
            # Create initial embed
            embed = self._create_progress_embed(title, stages, {})
            
            # Send message with ?wait=true to get message details back
            payload = {"embeds": [embed]}
            response = self.session.post(
                f"{self.webhook_url}?wait=true",
                json=payload,
                timeout=10
            )
            
            if response.status_code == 200:
                data = response.json()
                message = ProgressMessage(
                    message_id=data['id'],
                    channel_id=data['channel_id'],
                    webhook_url=self.webhook_url,
                    title=title,
                    created_at=get_est_time(),
                    last_updated=get_est_time(),
                    stages=stages,
                    file_info={}
                )
                
                # Store message for future updates
                with self.message_lock:
                    self.active_messages[message.message_id] = message
                
                logger.info(f"Created progress message: {message.message_id}")
                return message
            else:
                logger.error(f"Failed to create message: {response.status_code}")
                return None
                
        except Exception as e:
            logger.error(f"Failed to create progress message: {e}")
            return None
    
    def update_progress(self, message_id: str, file: str, stage: ProcessingStage, 
                        info: Optional[Dict[str, Any]] = None) -> bool:
        """Update the progress of a specific file in a message"""
        try:
            with self.message_lock:
                if message_id not in self.active_messages:
                    logger.warning(f"Message {message_id} not found")
                    return False
                
                message = self.active_messages[message_id]
                
                # Update stage
                if file in message.stages:
                    message.stages[file] = stage
                    
                # Update file info if provided
                if info:
                    if file not in message.file_info:
                        message.file_info[file] = {}
                    message.file_info[file].update(info)
                
                message.last_updated = get_est_time()
            
            # Create updated embed
            embed = self._create_progress_embed(
                message.title,
                message.stages,
                message.file_info
            )
            
            # Edit the message via webhook
            edit_url = f"https://discord.com/api/webhooks/{self.webhook_id}/{self.webhook_token}/messages/{message_id}"
            
            response = self.session.patch(
                edit_url,
                json={"embeds": [embed]},
                timeout=10
            )
            
            if response.status_code == 200:
                logger.debug(f"Updated message {message_id} for {file}: {stage.value}")
                return True
            else:
                logger.error(f"Failed to update message: {response.status_code}")
                return False
                
        except Exception as e:
            logger.error(f"Failed to update progress: {e}")
            return False
    
    def _create_progress_embed(self, title: str, stages: Dict[str, ProcessingStage], 
                               file_info: Dict[str, Any]) -> Dict:
        """Create an embed showing current progress"""
        # Determine overall color based on stages
        if any(stage == ProcessingStage.ERROR for stage in stages.values()):
            color = 0xff0000  # Red for error
        elif all(stage == ProcessingStage.COMPLETED for stage in stages.values()):
            color = 0x00ff00  # Green for all complete
        else:
            color = 0xffaa00  # Orange for in progress
        
        # Create progress bars for each file
        fields = []
        for file, stage in stages.items():
            progress = self._get_progress_bar(stage)
            value = f"{progress}\n**Status:** {stage.value}"
            
            # Add additional info if available
            if file in file_info:
                info = file_info[file]
                if 'progress_percent' in info:
                    value += f" ({info['progress_percent']}%)"
                if 'size' in info:
                    value += f"\n**Size:** {self._format_size(info['size'])}"
                if 'duration' in info:
                    value += f"\n**Duration:** {info['duration']:.1f}s"
            
            fields.append({
                "name": f"📄 {file[:50]}",  # Truncate long filenames
                "value": value,
                "inline": False
            })
        
        # Add summary field
        total = len(stages)
        completed = sum(1 for s in stages.values() if s == ProcessingStage.COMPLETED)
        errors = sum(1 for s in stages.values() if s == ProcessingStage.ERROR)
        
        summary = f"**Total:** {total} files\n"
        summary += f"**Completed:** {completed}\n"
        if errors > 0:
            summary += f"**Errors:** {errors}"
        
        fields.append({
            "name": "📊 Summary",
            "value": summary,
            "inline": True
        })
        
        return {
            "title": title,
            "color": color,
            "fields": fields,
            "timestamp": get_est_time().isoformat(),
            "footer": {
                "text": f"Sermon Processing Pipeline • EST {get_est_time().strftime('%I:%M %p')}"
            }
        }
    
    def _get_progress_bar(self, stage: ProcessingStage) -> str:
        """Generate a visual progress bar for a stage"""
        stages_order = [
            ProcessingStage.DETECTED,
            ProcessingStage.UPLOADING,
            ProcessingStage.UPLOADED,
            ProcessingStage.PROCESSING,
            ProcessingStage.COMPLETED
        ]
        
        if stage == ProcessingStage.ERROR:
            return "❌ ░░░░░░░░░░ Error"
        
        try:
            current_index = stages_order.index(stage)
            filled = "█" * (current_index + 1) * 2
            empty = "░" * (len(stages_order) - current_index - 1) * 2
            return f"{filled}{empty}"
        except ValueError:
            return "░░░░░░░░░░"
    
    def _format_size(self, size_bytes: int) -> str:
        """Format file size in human-readable format"""
        for unit in ['B', 'KB', 'MB', 'GB']:
            if size_bytes < 1024.0:
                return f"{size_bytes:.1f} {unit}"
            size_bytes /= 1024.0
        return f"{size_bytes:.1f} TB"
    
    def send_simple_notification(self, title: str, description: str, 
                                 color: int = 0x00ff00, fields: List[Dict] = None):
        """Send a simple one-time notification (backward compatibility)"""
        try:
            embed = {
                "title": title,
                "description": description,
                "color": color,
                "timestamp": get_est_time().isoformat(),
                "footer": {
                    "text": f"Sermon Processor • EST {get_est_time().strftime('%I:%M %p')}"
                }
            }
            
            if fields:
                embed["fields"] = fields
            
            payload = {"embeds": [embed]}
            
            response = self.session.post(
                self.webhook_url,
                json=payload,
                timeout=10
            )
            
            if response.status_code != 204:
                logger.warning(f"Discord notification failed: {response.status_code}")
                
        except Exception as e:
            logger.error(f"Failed to send Discord notification: {e}")
    
    def cleanup_old_messages(self, hours: int = 24):
        """Clean up old tracked messages after specified hours"""
        with self.message_lock:
            cutoff = get_est_time()
            old_messages = []
            
            for msg_id, msg in self.active_messages.items():
                age = (cutoff - msg.created_at).total_seconds() / 3600
                if age > hours:
                    old_messages.append(msg_id)
            
            for msg_id in old_messages:
                del self.active_messages[msg_id]
                logger.debug(f"Cleaned up old message: {msg_id}")


class SermonPipelineTracker:
    """High-level tracker for the sermon processing pipeline"""
    
    def __init__(self, notifier: DiscordLiveNotifier):
        self.notifier = notifier
        self.active_batches: Dict[str, ProgressMessage] = {}
    
    def start_batch_upload(self, files: List[str]) -> Optional[str]:
        """Start tracking a batch upload"""
        title = f"📤 Uploading {len(files)} Sermon{'s' if len(files) > 1 else ''}"
        message = self.notifier.create_progress_message(title, files)
        
        if message:
            self.active_batches[message.message_id] = message
            return message.message_id
        return None
    
    def update_file_upload_progress(self, batch_id: str, filename: str, 
                                   progress_percent: int, size: Optional[int] = None):
        """Update upload progress for a file"""
        info = {"progress_percent": progress_percent}
        if size:
            info["size"] = size
        
        stage = ProcessingStage.UPLOADING if progress_percent < 100 else ProcessingStage.UPLOADED
        self.notifier.update_progress(batch_id, filename, stage, info)
    
    def mark_file_processing(self, batch_id: str, filename: str):
        """Mark a file as being processed (converted)"""
        self.notifier.update_progress(batch_id, filename, ProcessingStage.PROCESSING)
    
    def mark_file_complete(self, batch_id: str, filename: str, 
                          duration: Optional[float] = None, size_reduction: Optional[float] = None):
        """Mark a file as complete"""
        info = {}
        if duration:
            info["duration"] = duration
        if size_reduction:
            info["size_reduction"] = size_reduction
        
        self.notifier.update_progress(batch_id, filename, ProcessingStage.COMPLETED, info)
    
    def mark_file_error(self, batch_id: str, filename: str, error: str):
        """Mark a file as having an error"""
        info = {"error": error[:200]}  # Truncate long errors
        self.notifier.update_progress(batch_id, filename, ProcessingStage.ERROR, info)
    
    def complete_batch(self, batch_id: str):
        """Mark a batch as complete and clean up"""
        if batch_id in self.active_batches:
            # Final update with completion summary
            message = self.active_batches[batch_id]
            
            # Change title to indicate completion
            completed = sum(1 for s in message.stages.values() 
                          if s == ProcessingStage.COMPLETED)
            errors = sum(1 for s in message.stages.values() 
                       if s == ProcessingStage.ERROR)
            
            if errors > 0:
                new_title = f"⚠️ Batch Complete with {errors} Error{'s' if errors > 1 else ''}"
            else:
                new_title = f"✅ Successfully Processed {completed} Sermon{'s' if completed > 1 else ''}"
            
            message.title = new_title
            
            # Send final update
            embed = self.notifier._create_progress_embed(
                new_title,
                message.stages,
                message.file_info
            )
            
            edit_url = f"https://discord.com/api/webhooks/{self.notifier.webhook_id}/{self.notifier.webhook_token}/messages/{batch_id}"
            
            try:
                self.notifier.session.patch(
                    edit_url,
                    json={"embeds": [embed]},
                    timeout=10
                )
            except Exception as e:
                logger.error(f"Failed to send final update: {e}")
            
            # Remove from active batches
            del self.active_batches[batch_id]


if __name__ == "__main__":
    # Example usage
    import os
    from dotenv import load_dotenv
    
    load_dotenv()
    
    # Initialize notifier
    webhook_url = os.getenv('DISCORD_WEBHOOK_URL', '')
    if not webhook_url:
        print("Please set DISCORD_WEBHOOK_URL in .env file")
        exit(1)
    
    notifier = DiscordLiveNotifier(webhook_url)
    tracker = SermonPipelineTracker(notifier)
    
    # Simulate a batch upload with live updates
    files = ["sermon_2024_01_01.wav", "sermon_2024_01_08.wav"]
    
    print("Starting batch upload simulation...")
    batch_id = tracker.start_batch_upload(files)
    
    if batch_id:
        print(f"Created batch: {batch_id}")
        
        # Simulate upload progress
        for i in range(0, 101, 20):
            time.sleep(1)
            for file in files:
                tracker.update_file_upload_progress(batch_id, file, i, 50000000)
                print(f"Updated {file}: {i}%")
        
        # Simulate processing
        for file in files:
            tracker.mark_file_processing(batch_id, file)
            print(f"Processing {file}...")
            time.sleep(2)
            
            tracker.mark_file_complete(batch_id, file, duration=5.2, size_reduction=75.3)
            print(f"Completed {file}")
        
        # Complete the batch
        tracker.complete_batch(batch_id)
        print("Batch complete!")