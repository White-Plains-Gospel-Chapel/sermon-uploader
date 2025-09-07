#!/bin/bash

# MinIO CORS Configuration Script
# This script properly configures CORS for MinIO to allow browser uploads

MINIO_ALIAS="myminio"
MINIO_URL="http://192.168.1.127:9000"
MINIO_ACCESS_KEY="gaius"
MINIO_SECRET_KEY="John 3:16"
BUCKET_NAME="sermons"

echo "🔧 Configuring MinIO CORS settings..."

# Set up MinIO client alias
echo "Setting up MinIO client alias..."
mc alias set $MINIO_ALIAS $MINIO_URL "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" --api S3v4

# Create CORS configuration JSON
cat > /tmp/cors-config.json << 'EOF'
[
  {
    "AllowedOrigins": ["*"],
    "AllowedMethods": ["GET", "PUT", "POST", "DELETE", "HEAD"],
    "AllowedHeaders": ["*"],
    "ExposeHeaders": ["ETag", "x-amz-request-id", "x-amz-id-2", "x-amz-server-side-encryption"],
    "MaxAgeSeconds": 3600
  }
]
EOF

echo "📝 CORS configuration:"
cat /tmp/cors-config.json

# Apply CORS configuration to the bucket
echo "Applying CORS configuration to bucket: $BUCKET_NAME"
mc cors set /tmp/cors-config.json $MINIO_ALIAS/$BUCKET_NAME

# Verify CORS configuration
echo "✅ Verifying CORS configuration..."
mc cors get $MINIO_ALIAS/$BUCKET_NAME

# Set bucket policy to allow public read/write for presigned URLs
echo "Setting bucket policy for presigned URL access..."
cat > /tmp/bucket-policy.json << EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": ["*"]},
      "Action": [
        "s3:GetBucketLocation",
        "s3:ListBucket"
      ],
      "Resource": ["arn:aws:s3:::$BUCKET_NAME"]
    },
    {
      "Effect": "Allow",
      "Principal": {"AWS": ["*"]},
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:DeleteObject"
      ],
      "Resource": ["arn:aws:s3:::$BUCKET_NAME/*"],
      "Condition": {
        "StringLike": {
          "aws:Referer": ["*"]
        }
      }
    }
  ]
}
EOF

# Apply bucket policy
mc anonymous set-json /tmp/bucket-policy.json $MINIO_ALIAS/$BUCKET_NAME

# Test CORS with curl
echo "🧪 Testing CORS configuration..."
echo "Testing OPTIONS request..."
curl -I -X OPTIONS \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: PUT" \
  -H "Access-Control-Request-Headers: Content-Type" \
  $MINIO_URL/$BUCKET_NAME/test.wav 2>/dev/null | head -5

echo ""
echo "✅ CORS configuration complete!"
echo ""
echo "Note: If you're still experiencing CORS issues:"
echo "1. Ensure MinIO is running with the correct configuration"
echo "2. Check that your frontend is using the correct URLs"
echo "3. Verify network connectivity between frontend and MinIO"
echo "4. Check browser console for specific CORS error messages"

# Clean up temp files
rm -f /tmp/cors-config.json /tmp/bucket-policy.json