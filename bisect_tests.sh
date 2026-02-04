#!/usr/bin/env bash
set -euo pipefail
tests=$(go test ./endpoint -list . | awk '/^Test/ {print $1}')
mapfile -t arr <<<"$tests"
n=${#arr[@]}
if [ $n -eq 0 ]; then echo "No tests found"; exit 1; fi
run_range(){
  local start=$1; local end=$2
  local pat=""
  for ((i=start;i<=end;i++)); do
    if [ -n "$pat" ]; then pat="$pat|${arr[i]}"; else pat="${arr[i]}"; fi
  done
  echo "Running range $start-$end: ${arr[start]}..${arr[end]}"
  go test ./endpoint -run "^(${pat})$" -v -count=1 || return 1
  return 0
}
# First check if full set fails
if run_range 0 $((n-1)); then
  echo "All endpoint tests passed in full run"
  exit 0
fi
# binary search
low=0; high=$((n-1))
while [ $low -lt $high ]; do
  mid=$(((low+high)/2))
  if run_range $low $mid; then
    low=$((mid+1))
  else
    high=$mid
  fi
done
echo "Failing test is: ${arr[low]}"
exit 0
