# Makefile Commands

## New Server Management Commands

### `make kill-server`
Kills the running server process.

```bash
make kill-server
```

**Output:**
```
Killing server...
Server killed!
```

**Use case**: Stop the server when you need to make changes or free up the port.

---

### `make restart-server`
Kills the running server, rebuilds the application, and starts it in the foreground.

```bash
make restart-server
```

**Output:**
```
Killing server...
Server killed!
Building server...
Building CLI...
Build complete!
Starting server...
[Server runs in foreground with live logs]
```

**Features:**
- Automatically kills any running server
- Rebuilds both server and CLI binaries
- Starts server in foreground (blocking)
- Shows live logs in terminal
- Press Ctrl+C to stop

**Use case**: Quick restart after making code changes with live log monitoring.

---

## All Available Commands

Run `make help` to see all available commands:

```bash
$ make help
build                          Build the application
clean                          Clean build artifacts
deps                           Download dependencies
docker-build                   Build Docker image
docker-down                    Stop Docker Compose services
docker-logs                    View Docker logs
docker-up                      Start Docker Compose services
fmt                            Format code
help                           Display this help screen
install-tools                  Install development tools
kill-server                    Kill the running server
lint                           Run linters
restart-server                 Kill and restart the server
run-cli                        Run the CLI
run-server                     Run the server
test-coverage                  Run tests with coverage report
test                           Run tests
```

## Common Workflows

### Development Workflow

```bash
# 1. Make code changes
vim internal/app/download_service.go

# 2. Restart server to apply changes
make restart-server

# 3. Monitor logs
tail -f ~/Downloads/x-download/logs/$(date +%Y%m%d).log

# 4. Test the changes
make test
```

### Quick Restart

```bash
# One command to rebuild and restart
make restart-server
```

### Stop Server

```bash
# Stop the server
make kill-server
```

### Check Server Status

```bash
# Check if server is running
curl http://localhost:8080/health

# Or check process
ps aux | grep x-extract-server
```

## Implementation Details

### kill-server
```makefile
kill-server: ## Kill the running server
	@echo "Killing server..."
	@pkill -f $(SERVER_BINARY) || echo "No server process found"
	@echo "Server killed!"
```

- Uses `pkill -f` to find and kill the server process
- Gracefully handles case when no server is running
- Silent operation with `@` prefix

### restart-server
```makefile
restart-server: kill-server build ## Kill and restart the server
	@echo "Starting server..."
	./bin/$(SERVER_BINARY)
```

- Depends on `kill-server` and `build` targets
- Runs server in foreground (blocking)
- Shows live logs in terminal
- Press Ctrl+C to stop the server

## Benefits

1. **Fast Development**: Quick restart without manual steps
2. **Convenience**: Single command to rebuild and restart
3. **Live Logs**: See server output in real-time
4. **Easy Debugging**: Immediate feedback on errors
5. **Error Handling**: Gracefully handles missing processes
6. **Simple Control**: Ctrl+C to stop, make restart-server to restart

## Notes

- Server logs are written to `~/Downloads/x-download/logs/YYYYMMDD.log`
- Server runs on `localhost:8080` by default
- `make restart-server` runs in foreground (blocking) - shows live logs
- `make run-server` also runs in foreground
- Press Ctrl+C to stop the server when running in foreground

