#!/bin/bash

echo "=== Testing socket to state flow ==="
echo "Before:"
curl -s http://localhost:8080/api/state | jq '.status'

echo "Sending orch.started event..."
echo '{"type":"orch.started","time":"2026-01-20T10:00:00Z","payload":{"unit_count":1,"parallelism":4}}' | nc -U ~/.choo/web.sock &
sleep 2

echo "After:"
curl -s http://localhost:8080/api/state | jq '.status'
