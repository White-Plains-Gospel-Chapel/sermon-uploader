// Simple upload fix - handles bulk uploads without freezing the frontend

export async function uploadWithDelay(file: File, uploadUrl: string, delay: number = 0) {
  // Add delay to prevent overwhelming the browser
  if (delay > 0) {
    await new Promise(resolve => setTimeout(resolve, delay));
  }

  console.log(`🚀 Starting upload: ${file.name} (${(file.size / 1024 / 1024).toFixed(1)} MB)`);

  return new Promise<void>((resolve, reject) => {
    const xhr = new XMLHttpRequest();

    // Progress tracking
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        const progress = (e.loaded / e.total) * 100;
        console.log(`📊 ${file.name}: ${progress.toFixed(1)}%`);
      }
    };

    // Success handler
    xhr.onload = () => {
      if (xhr.status === 200 || xhr.status === 204) {
        console.log(`✅ Upload completed: ${file.name}`);
        resolve();
      } else {
        const error = new Error(`Upload failed: ${xhr.status} ${xhr.statusText}`);
        console.error(`❌ Upload failed: ${file.name}`, error);
        reject(error);
      }
    };

    // Error handlers
    xhr.onerror = () => {
      const error = new Error('Network error');
      console.error(`❌ Network error: ${file.name}`, error);
      reject(error);
    };

    xhr.ontimeout = () => {
      const error = new Error('Upload timeout');
      console.error(`❌ Timeout: ${file.name}`, error);
      reject(error);
    };

    // Configure request
    xhr.timeout = 10 * 60 * 1000; // 10 minutes timeout
    xhr.open('PUT', uploadUrl);
    xhr.setRequestHeader('Content-Type', file.type || 'application/octet-stream');
    xhr.setRequestHeader('Content-Length', file.size.toString());
    
    // Send file
    xhr.send(file);
  });
}

// Bulk upload with proper sequencing
export async function bulkUploadWithQueue(files: FileList | File[], getUploadUrl: (file: File) => Promise<string>) {
  const fileArray = Array.from(files);
  const maxConcurrent = 3; // Limit concurrent uploads
  const delayBetween = 200; // 200ms delay between starting uploads

  console.log(`📦 Starting bulk upload: ${fileArray.length} files`);

  const results: Array<{ file: File; success: boolean; error?: Error }> = [];

  // Process files in batches
  for (let i = 0; i < fileArray.length; i += maxConcurrent) {
    const batch = fileArray.slice(i, i + maxConcurrent);
    console.log(`📦 Processing batch ${Math.floor(i / maxConcurrent) + 1}: ${batch.length} files`);

    // Start batch uploads with delays
    const batchPromises = batch.map(async (file, batchIndex) => {
      try {
        // Get upload URL
        const uploadUrl = await getUploadUrl(file);
        
        // Add delay to prevent browser overload
        const delay = batchIndex * delayBetween;
        
        // Upload file
        await uploadWithDelay(file, uploadUrl, delay);
        
        return { file, success: true };
      } catch (error) {
        console.error(`❌ Failed to upload ${file.name}:`, error);
        return { file, success: false, error: error as Error };
      }
    });

    // Wait for batch to complete
    const batchResults = await Promise.all(batchPromises);
    results.push(...batchResults);

    // Add delay between batches
    if (i + maxConcurrent < fileArray.length) {
      console.log(`⏳ Waiting before next batch...`);
      await new Promise(resolve => setTimeout(resolve, 1000));
    }
  }

  // Summary
  const successful = results.filter(r => r.success).length;
  const failed = results.length - successful;
  
  console.log(`📊 Bulk upload complete: ${successful} successful, ${failed} failed`);
  
  return results;
}