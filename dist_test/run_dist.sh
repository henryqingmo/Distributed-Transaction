#!/bin/bash
# Distributed test runner — run this on any one VM after all 5 servers are up.
# Usage: ./run_dist.sh <path-to-config> <path-to-mp3-dir>
# Example: ./run_dist.sh ../config.txt ../mp3

CONFIG=${1:-../config.txt}
MP3=${2:-../mp3}
CLIENT="$MP3/client"
PASS=0
FAIL=0

run_test() {
    local name=$1
    local input=$2
    local expected=$3
    local actual
    actual=$("$CLIENT" 1 "$CONFIG" < "$input" 2>&1)
    if diff -q <(echo "$actual") "$expected" > /dev/null 2>&1; then
        echo "[PASS] $name"
        PASS=$((PASS+1))
    else
        echo "[FAIL] $name"
        echo "  Expected:"
        cat "$expected" | sed 's/^/    /'
        echo "  Got:"
        echo "$actual" | sed 's/^/    /'
        FAIL=$((FAIL+1))
    fi
}

DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== Checking servers are reachable ==="
while IFS= read -r line; do
    branch=$(echo "$line" | awk '{print $1}')
    host=$(echo "$line" | awk '{print $2}')
    port=$(echo "$line" | awk '{print $3}')
    if nc -z -w2 "$host" "$port" 2>/dev/null; then
        echo "  [UP]   $branch @ $host:$port"
    else
        echo "  [DOWN] $branch @ $host:$port  <-- server not reachable!"
    fi
done < "$CONFIG"
echo ""

echo "=== Sequential Tests ==="

run_test "t1: basic deposit/balance/commit across branches" \
    "$DIR/t1_basic.txt" "$DIR/t1_basic_expected.txt"

run_test "t2: balance on nonexistent account -> NOT FOUND, ABORTED" \
    "$DIR/t2_not_found.txt" "$DIR/t2_not_found_expected.txt"

run_test "t3: negative balance at commit -> ABORTED" \
    "$DIR/t3_negative_balance.txt" "$DIR/t3_negative_balance_expected.txt"

echo ""
echo "=== Concurrent Non-Conflicting Transactions (should both succeed) ==="

"$CLIENT" 2 "$CONFIG" < "$DIR/t4_concurrent_noconflict_c1.txt" > /tmp/t4_c1_out.txt 2>&1 &
PID1=$!
"$CLIENT" 3 "$CONFIG" < "$DIR/t4_concurrent_noconflict_c2.txt" > /tmp/t4_c2_out.txt 2>&1 &
PID2=$!
wait $PID1 $PID2

if diff -q /tmp/t4_c1_out.txt "$DIR/t4_concurrent_noconflict_c1_expected.txt" > /dev/null 2>&1 && \
   diff -q /tmp/t4_c2_out.txt "$DIR/t4_concurrent_noconflict_c2_expected.txt" > /dev/null 2>&1; then
    echo "[PASS] t4: both non-conflicting transactions committed"
    PASS=$((PASS+1))
else
    echo "[FAIL] t4: concurrent non-conflicting transactions"
    echo "  Client1 got:"; cat /tmp/t4_c1_out.txt | sed 's/^/    /'
    echo "  Client2 got:"; cat /tmp/t4_c2_out.txt | sed 's/^/    /'
    FAIL=$((FAIL+1))
fi

echo ""
echo "=== Deadlock Test (one commits, one aborts) ==="

# First set up the accounts used in deadlock test
"$CLIENT" 4 "$CONFIG" < "$DIR/t5_deadlock_setup.txt" > /dev/null 2>&1

"$CLIENT" 5 "$CONFIG" < "$DIR/t5_deadlock_c1.txt" > /tmp/t5_c1_out.txt 2>&1 &
PID1=$!
"$CLIENT" 6 "$CONFIG" < "$DIR/t5_deadlock_c2.txt" > /tmp/t5_c2_out.txt 2>&1 &
PID2=$!
wait $PID1 $PID2

C1=$(cat /tmp/t5_c1_out.txt | tail -1)
C2=$(cat /tmp/t5_c2_out.txt | tail -1)

if { [ "$C1" = "COMMIT OK" ] && [ "$C2" = "ABORTED" ]; } || \
   { [ "$C1" = "ABORTED" ] && [ "$C2" = "COMMIT OK" ]; }; then
    echo "[PASS] t5: deadlock resolved — one committed, one aborted"
    PASS=$((PASS+1))
else
    echo "[FAIL] t5: unexpected deadlock outcome"
    echo "  Client1 last line: $C1"
    echo "  Client2 last line: $C2"
    FAIL=$((FAIL+1))
fi

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
