# Troubleshooting Guide

## Common Issues

### Server Won't Start

#### Issue: Port already in use
```
Error: listen tcp :8080: bind: address already in use
```

**Solution:**
1. Check if another instance is running:
   ```bash
   lsof -i :8080
   ```
2. Kill the process or change the port in config:
   ```yaml
   server:
     port: 8081
   ```

#### Issue: Database locked
```
Error: database is locked
```

**Solution:**
1. Ensure no other instance is running
2. Remove the lock file:
   ```bash
   rm ~/Downloads/x-download/config/queue.db-shm
   rm ~/Downloads/x-download/config/queue.db-wal
   ```

### Download Failures

#### Issue: yt-dlp not found
```
Error: exec: "yt-dlp": executable file not found in $PATH
```

**Solution:**
1. Install yt-dlp:
   ```bash
   pip install yt-dlp
   # or
   brew install yt-dlp
   ```
2. Verify installation:
   ```bash
   which yt-dlp
   ```
3. Update config if needed:
   ```yaml
   twitter:
     ytdlp_binary: /path/to/yt-dlp
   ```

#### Issue: tdl not found
```
Error: exec: "tdl": executable file not found in $PATH
```

**Solution:**
1. Install tdl from https://github.com/iyear/tdl
2. Verify installation:
   ```bash
   which tdl
   ```
3. Update config if needed:
   ```yaml
   telegram:
     tdl_binary: /path/to/tdl
   ```

#### Issue: Twitter download fails with 403
```
Error: HTTP Error 403: Forbidden
```

**Solution:**
1. Update your Twitter cookies:
   ```bash
   # Export cookies from browser using extension
   # Save to ~/Downloads/x-download/cookies/x.com/default.cookie
   ```
2. Ensure cookie file path is correct in config:
   ```yaml
   twitter:
     cookie_file: $HOME/Downloads/x-download/cookies/x.com/default.cookie
   ```

#### Issue: Telegram authentication required
```
Error: not authorized
```

**Solution:**
1. Login to Telegram using tdl:
   ```bash
   tdl login -n rogan
   ```
2. Follow the authentication prompts
3. Verify profile in config matches:
   ```yaml
   telegram:
     profile: rogan
   ```

### Queue Issues

#### Issue: Downloads stuck in "processing"
```
Status shows "processing" but nothing happens
```

**Solution:**
1. Check server logs:
   ```bash
   tail -f ~/Downloads/x-download/x-extract.log
   ```
2. Restart the server:
   ```bash
   # If running as service
   systemctl restart x-extract
   
   # If running manually
   pkill x-extract-server
   ./bin/x-extract-server
   ```
3. Manually reset stuck downloads:
   ```bash
   x-extract-cli cancel <download-id>
   x-extract-cli retry <download-id>
   ```

#### Issue: Queue not processing
```
Downloads stay in "queued" status
```

**Solution:**
1. Check if queue manager is running:
   ```bash
   curl http://localhost:8080/health
   ```
2. Check configuration:
   ```yaml
   download:
     auto_start_workers: true
   ```
3. Restart the server

### Configuration Issues

#### Issue: Config file not found
```
Error: failed to read config file
```

**Solution:**
1. Create config file:
   ```bash
   mkdir -p ~/.x-extract
   cp configs/config.yaml ~/.x-extract/config.yaml
   ```
2. Or specify config path:
   ```bash
   CONFIG_PATH=/path/to/config.yaml ./bin/x-extract-server
   ```

#### Issue: Invalid configuration
```
Error: invalid configuration: download base directory not configured
```

**Solution:**
1. Check required fields in config:
   ```yaml
   download:
     base_dir: $HOME/Downloads/x-download
   queue:
     database_path: $HOME/Downloads/x-download/config/queue.db
   telegram:
     profile: rogan
   ```

### Permission Issues

#### Issue: Cannot create directories
```
Error: failed to create directory: permission denied
```

**Solution:**
1. Check directory permissions:
   ```bash
   ls -la ~/Downloads/
   ```
2. Create directories manually:
   ```bash
   mkdir -p ~/Downloads/x-download
   chmod 755 ~/Downloads/x-download
   ```

#### Issue: Cannot write to database
```
Error: attempt to write a readonly database
```

**Solution:**
1. Check database file permissions:
   ```bash
   ls -la ~/Downloads/x-download/queue.db
   ```
2. Fix permissions:
   ```bash
   chmod 644 ~/Downloads/x-download/queue.db
   ```

## Debugging

### Enable Debug Logging

Update config:
```yaml
logging:
  level: debug
  format: console
  output_path: stdout
```

Or set environment variable:
```bash
XEXTRACT_LOGGING_LEVEL=debug ./bin/x-extract-server
```

### View Logs

```bash
# Server logs (if using systemd)
journalctl -u x-extract -f

# Application logs
tail -f ~/Downloads/x-download/x-extract.log

# Docker logs
docker logs -f x-extract
```

### Check System Resources

```bash
# Check disk space
df -h ~/Downloads/x-download

# Check memory usage
ps aux | grep x-extract

# Check open files
lsof -p $(pgrep x-extract-server)
```

## Getting Help

If you're still experiencing issues:

1. Check the [GitHub Issues](https://github.com/yourusername/x-extract-go/issues)
2. Enable debug logging and collect logs
3. Create a new issue with:
   - Error message
   - Configuration (sanitized)
   - Steps to reproduce
   - System information (OS, Go version, etc.)

