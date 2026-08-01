#!/bin/bash
kill $(lsof -t -i :8080) 2>/dev/null || true
cd src/server
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable"
export SESSION_SECRET="test-secret"
go run cmd/server/main.go > /home/jules/server_output.log 2>&1 &
SERVER_PID=$!
sleep 5
cd ../web
PLAYWRIGHT_SKIP_WEB_SERVER=true PLAYWRIGHT_BASE_URL="http://localhost:5173" pnpm exec vite dev --port 5173 > /home/jules/vite_output.log 2>&1 &
VITE_PID=$!
sleep 5
PLAYWRIGHT_SKIP_WEB_SERVER=true PLAYWRIGHT_BASE_URL="http://localhost:5173" pnpm test:e2e console-overview.spec.ts
kill $VITE_PID
kill $SERVER_PID
