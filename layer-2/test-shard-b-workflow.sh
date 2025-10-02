#!/bin/bash

BASE_URL="http://localhost:7001"
L1_URL="http://localhost:5000"

echo "🧪 Testing Shard B Complete Workflow"
echo "====================================="
echo "Shard: shard-b | Group: group-b"
echo ""

# 1. Create Session
echo "📝 Step 1: Creating session..."
SESSION_RESPONSE=$(curl -s -X POST "$BASE_URL/session/start" \
  -H "Content-Type: application/json" \
  -d '{"operator_id":"OPR-003"}')

SESSION_ID=$(echo $SESSION_RESPONSE | jq -r '.session_id')
if [ "$SESSION_ID" = "null" ] || [ -z "$SESSION_ID" ]; then
    echo "❌ Failed to create session"
    echo "$SESSION_RESPONSE" | jq '.'
    exit 1
fi
echo "✅ Session created: $SESSION_ID"
echo ""

# 2. Scan Package
echo "📦 Step 2: Scanning package..."
SCAN_RESPONSE=$(curl -s -X GET "$BASE_URL/session/$SESSION_ID/scan" \
  -H "Content-Type: application/json" \
  -d '{"package_id":"PKG-002"}')

if echo "$SCAN_RESPONSE" | jq -e '.message' > /dev/null 2>&1; then
    echo "✅ Package scanned"
    echo "$SCAN_RESPONSE" | jq -c '{message, package_id, supplier, status}'
else
    echo "❌ Scan failed"
    echo "$SCAN_RESPONSE" | jq '.'
    exit 1
fi
echo ""

# 3. Validate Package
echo "🔍 Step 3: Validating package..."
VALIDATE_RESPONSE=$(curl -s -X POST "$BASE_URL/session/$SESSION_ID/validate" \
  -H "Content-Type: application/json" \
  -d '{"signature":"sig_global_tech_002","package_id":"PKG-002"}')

if echo "$VALIDATE_RESPONSE" | jq -e '.message' > /dev/null 2>&1; then
    echo "✅ Package validated"
    echo "$VALIDATE_RESPONSE" | jq -c '{message, package_id, is_trusted, status}'
else
    echo "❌ Validation failed"
    echo "$VALIDATE_RESPONSE" | jq '.'
    exit 1
fi
echo ""

# 4. Quality Check
echo "✔️  Step 4: Quality check..."
QC_RESPONSE=$(curl -s -X POST "$BASE_URL/session/$SESSION_ID/qc" \
  -H "Content-Type: application/json" \
  -d '{"passed":true,"issues":[]}')

if echo "$QC_RESPONSE" | jq -e '.message' > /dev/null 2>&1; then
    echo "✅ Quality check passed"
    echo "$QC_RESPONSE" | jq -c '{message, qc_id, passed, status}'
else
    echo "❌ QC failed"
    echo "$QC_RESPONSE" | jq '.'
    exit 1
fi
echo ""

# 5. Label Package
echo "🏷️  Step 5: Creating label..."
LABEL_RESPONSE=$(curl -s -X POST "$BASE_URL/session/$SESSION_ID/label" \
  -H "Content-Type: application/json" \
  -d '{"courier_id":"CUR-002"}')

if echo "$LABEL_RESPONSE" | jq -e '.message' > /dev/null 2>&1; then
    echo "✅ Label created"
    echo "$LABEL_RESPONSE" | jq -c '{message, label_id, tracking_no, courier}'
else
    echo "❌ Label creation failed"
    echo "$LABEL_RESPONSE" | jq '.'
    exit 1
fi
echo ""

# 6. Commit to L1
echo "🔗 Step 6: Committing to L1..."
COMMIT_RESPONSE=$(curl -s -X POST "$BASE_URL/session/$SESSION_ID/commit" \
  -H "Content-Type: application/json")

TX_HASH=$(echo "$COMMIT_RESPONSE" | jq -r '.tx_hash')
BLOCK_HEIGHT=$(echo "$COMMIT_RESPONSE" | jq -r '.block_height')

if [ "$TX_HASH" != "null" ] && [ ! -z "$TX_HASH" ]; then
    echo "✅ Session committed to L1!"
    echo "$COMMIT_RESPONSE" | jq -c '{message, session_id, tx_hash, block_height, shard_id, status}'
    echo ""
    
    # 7. Verify on L1
    echo "📊 Step 7: Verifying on L1..."
    sleep 1  # Give L1 a moment to process
    
    # Check session in L1 by shard
    echo "   Checking L1 sessions for shard-b..."
    L1_SESSIONS=$(curl -s "$L1_URL/l1/sessions/shard/shard-b")
    SESSION_COUNT=$(echo "$L1_SESSIONS" | jq '.data | length')
    echo "   ✅ Found $SESSION_COUNT sessions for shard-b in L1"
    
    # Check session in L1 by group
    echo "   Checking L1 sessions for group-b..."
    L1_GROUP_SESSIONS=$(curl -s "$L1_URL/l1/sessions/group/group-b")
    GROUP_SESSION_COUNT=$(echo "$L1_GROUP_SESSIONS" | jq '.data | length')
    echo "   ✅ Found $GROUP_SESSION_COUNT sessions for group-b in L1"
    
    echo ""
    echo "🎉 WORKFLOW COMPLETE!"
    echo "╔════════════════════════════════════════╗"
    echo "║         Shard B Test Results          ║"
    echo "╠════════════════════════════════════════╣"
    echo "║ Session ID:    $SESSION_ID"
    echo "║ Tx Hash:       $TX_HASH"
    echo "║ Block Height:  $BLOCK_HEIGHT"
    echo "║ Shard:         shard-b"
    echo "║ Client Group:  group-b"
    echo "╚════════════════════════════════════════╝"
else
    echo "❌ Commit failed"
    echo "$COMMIT_RESPONSE" | jq '.'
    exit 1
fi