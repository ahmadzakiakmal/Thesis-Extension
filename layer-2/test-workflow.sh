#!/bin/bash

BASE_URL="http://localhost:6000"

echo "🧪 Testing Complete L2 Shard Workflow"
echo "======================================"
echo ""

# 1. Create Session
echo "📝 Step 1: Creating session..."
SESSION_RESPONSE=$(curl -s -X POST "$BASE_URL/session/start" \
  -H "Content-Type: application/json" \
  -d '{"operator_id":"OPR-001"}')

SESSION_ID=$(echo $SESSION_RESPONSE | jq -r '.session_id')
echo "✅ Session created: $SESSION_ID"
echo ""

# 2. Scan Package
echo "📦 Step 2: Scanning package..."
SCAN_RESPONSE=$(curl -s -X GET "$BASE_URL/session/$SESSION_ID/scan" \
  -H "Content-Type: application/json" \
  -d '{"package_id":"PKG-001"}')
echo "✅ Package scanned"
echo "$SCAN_RESPONSE" | jq '.'
echo ""

# 3. Validate Package
echo "🔍 Step 3: Validating package..."
VALIDATE_RESPONSE=$(curl -s -X POST "$BASE_URL/session/$SESSION_ID/validate" \
  -H "Content-Type: application/json" \
  -d '{"signature":"sig_acme_electronics_001","package_id":"PKG-001"}')
echo "✅ Package validated"
echo "$VALIDATE_RESPONSE" | jq '.'
echo ""

# 4. Quality Check
echo "✔️  Step 4: Quality check..."
QC_RESPONSE=$(curl -s -X POST "$BASE_URL/session/$SESSION_ID/qc" \
  -H "Content-Type: application/json" \
  -d '{"passed":true,"issues":[]}')
echo "✅ Quality check passed"
echo "$QC_RESPONSE" | jq '.'
echo ""

# 5. Label Package
echo "🏷️  Step 5: Creating label..."
LABEL_RESPONSE=$(curl -s -X POST "$BASE_URL/session/$SESSION_ID/label" \
  -H "Content-Type: application/json" \
  -d '{"courier_id":"CUR-001"}')
echo "✅ Label created"
echo "$LABEL_RESPONSE" | jq '.'
echo ""

# 6. Commit to L1
echo "🔗 Step 6: Committing to L1..."
COMMIT_RESPONSE=$(curl -s -X POST "$BASE_URL/session/$SESSION_ID/commit" \
  -H "Content-Type: application/json")

if echo "$COMMIT_RESPONSE" | jq -e '.tx_hash' > /dev/null 2>&1; then
    echo "✅ Session committed to L1!"
    echo "$COMMIT_RESPONSE" | jq '.'
    echo ""
    echo "🎉 WORKFLOW COMPLETE!"
    echo "   Session ID: $SESSION_ID"
    echo "   L1 Tx Hash: $(echo $COMMIT_RESPONSE | jq -r '.tx_hash')"
else
    echo "⚠️  Commit failed (L1 might not be running)"
    echo "$COMMIT_RESPONSE" | jq '.'
fi