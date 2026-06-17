#!/bin/bash

# Configuration variables
WATCH_DIR="/mnt/hd1/Download"     # aria2 download directory
VR_DIR="/mnt/hd1/Multimedia/VR"   # VR file target directory
DMM_DIR="/mnt/hd1/Multimedia/DMM" # Non-VR file target directory
TOOL_PATH="better-av-tool"        # Tool path
LOG_FILE="/mnt/hd1/Multimedia/VR/file-monitor.log"

# Create log function
log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" >> "$LOG_FILE"
}

# Check if directories exist
if [ ! -d "$WATCH_DIR" ] || [ ! -d "$VR_DIR" ] || [ ! -d "$DMM_DIR" ]; then
    log "Error: Source or target directory does not exist"
    exit 1
fi

# Process file function
process_file() {
    local file="$1"
    local filename=$(basename "$file")
    local dest_dir
    local tool_work_dir
    
    # Ensure the file download is complete (check if .aria2 suffix file exists)
    if [ -f "${file}.aria2" ]; then
        return
    fi
    
    # Determine if the file is a VR file based on its name
    if [[ "$filename" =~ [vV][rR] ]]; then
        dest_dir="$VR_DIR"
        tool_work_dir="$VR_DIR"
    else
        dest_dir="$DMM_DIR"
        tool_work_dir="$DMM_DIR"
    fi
    
    local dest_file="$dest_dir/$filename"
    
    log "Processing file: $filename"
    
    # Move file
    mv "$file" "$dest_file"
    if [ $? -ne 0 ]; then
        log "Error: Failed to move file - $filename"
        return
    fi
    
    # Switch to corresponding working directory and execute the processing tool
    cd "$tool_work_dir" && ./"$TOOL_PATH"
    if [ $? -eq 0 ]; then
        log "Successfully processed file: $filename"
    else
        log "Error: Failed to process file - $filename"
    fi
}

# Main processing logic
main() {
    log "Starting directory scan: $WATCH_DIR"
    
    # Modified find command, excluding *__temp and *.aria2 files
    find "$WATCH_DIR" -type f ! -name "*__temp" ! -name "*.aria2" -print0 | while IFS= read -r -d '' file; do
        if [[ "$file" != *"__temp"* ]] && [[ "$file" != *.aria2 ]]; then
            process_file "$file"
        fi
    done
}

# Run the main program
main