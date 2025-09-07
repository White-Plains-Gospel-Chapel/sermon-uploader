#!/usr/bin/env node

// Test CORS configuration for MinIO bulk uploads
const http = require('http');

async function testCORS() {
  console.log('🧪 Testing CORS configuration for bulk uploads...\n');

  // Step 1: Get presigned URLs from backend
  console.log('1️⃣ Requesting presigned URLs from backend...');
  
  const files = [
    { filename: `test-cors-1-${Date.now()}.wav`, fileSize: 1024 * 1024 * 50 }, // 50MB
    { filename: `test-cors-2-${Date.now()}.wav`, fileSize: 1024 * 1024 * 150 }, // 150MB
    { filename: `test-cors-3-${Date.now()}.wav`, fileSize: 1024 * 1024 * 250 }, // 250MB
  ];

  try {
    const response = await fetch('http://localhost:8000/api/upload/presigned-batch', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ files })
    });

    if (!response.ok) {
      throw new Error(`Backend responded with ${response.status}`);
    }

    const result = await response.json();
    console.log('✅ Got presigned URLs from backend');
    console.log(`   - Success count: ${result.success_count}`);
    console.log(`   - Duplicate count: ${result.duplicate_count}`);
    console.log(`   - Error count: ${result.error_count}\n`);

    // Step 2: Test CORS preflight for each URL
    console.log('2️⃣ Testing CORS preflight requests...');
    
    for (const file of files) {
      const fileResult = result.results[file.filename];
      if (fileResult && fileResult.uploadUrl) {
        const url = new URL(fileResult.uploadUrl);
        
        console.log(`   Testing ${file.filename} (${fileResult.uploadMethod})...`);
        
        // Test OPTIONS request
        const corsResponse = await fetch(url.origin + url.pathname, {
          method: 'OPTIONS',
          headers: {
            'Origin': 'http://localhost:3000',
            'Access-Control-Request-Method': 'PUT',
            'Access-Control-Request-Headers': 'Content-Type'
          }
        });

        if (corsResponse.status === 204 || corsResponse.status === 200) {
          console.log(`   ✅ CORS preflight successful for ${file.filename}`);
          
          // Check CORS headers
          const allowOrigin = corsResponse.headers.get('Access-Control-Allow-Origin');
          const allowMethods = corsResponse.headers.get('Access-Control-Allow-Methods');
          
          if (allowOrigin) {
            console.log(`      Allow-Origin: ${allowOrigin}`);
          }
          if (allowMethods) {
            console.log(`      Allow-Methods: ${allowMethods}`);
          }
        } else {
          console.log(`   ❌ CORS preflight failed for ${file.filename} (status: ${corsResponse.status})`);
        }
      }
    }

    // Step 3: Test actual upload with small test data
    console.log('\n3️⃣ Testing actual upload with CORS...');
    
    const testFile = files[0];
    const testResult = result.results[testFile.filename];
    
    if (testResult && testResult.uploadUrl) {
      // Create small test data (1KB)
      const testData = Buffer.alloc(1024);
      
      const uploadResponse = await fetch(testResult.uploadUrl, {
        method: 'PUT',
        headers: {
          'Content-Type': 'audio/wav',
          'Origin': 'http://localhost:3000'
        },
        body: testData
      });

      if (uploadResponse.ok) {
        console.log('✅ Test upload successful with CORS');
        console.log(`   Status: ${uploadResponse.status}`);
        
        const corsHeader = uploadResponse.headers.get('Access-Control-Allow-Origin');
        if (corsHeader) {
          console.log(`   CORS header present: ${corsHeader}`);
        }
      } else {
        console.log(`❌ Test upload failed (status: ${uploadResponse.status})`);
      }
    }

    console.log('\n✅ CORS test completed successfully!');
    console.log('Your bulk upload should work now.');

  } catch (error) {
    console.error('\n❌ CORS test failed:', error.message);
    console.error('\nTroubleshooting tips:');
    console.error('1. Ensure backend is running on port 8000');
    console.error('2. Ensure MinIO is accessible at 192.168.1.127:9000');
    console.error('3. Check that CORS is configured on MinIO server');
    console.error('4. Verify network connectivity between services');
  }
}

// Run the test
testCORS();