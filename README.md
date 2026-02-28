# Screenshot Server

A headless screenshot server application for Windows that captures, manages, and provides access to screenshots via TCP.

## Overview

This application is a background service that:

1. Periodically captures screenshots from all displays
2. Stores screenshots with metadata in a database
3. Provides TCP interface for remote access and control
4. Manages screenshot library with cleaning and organization features

## Features

- **Multi-Display Support**: Captures screenshots from all active displays
- **Database Management**: Stores screenshots with metadata including:
  - Timestamp information (year, month, day, hour, minute, second)
  - Display number
  - File hash for integrity verification
- **TCP Interface**: Remote control and access via TCP port
- **Library Management**: Automatic organization and cleanup of screenshots
- **Headless Operation**: Runs as a background service without a GUI
- **Data Consistency**: Handles duplicate entries by overwriting existing data

## System Requirements

- **OS**: Windows 10
- **Dependencies**: SQLite for database storage

## Architecture

The application runs multiple concurrent threads:

1. **Screenshot Thread**: Captures screenshots from all displays
2. **Library Management Thread**: Organizes and manages the screenshot database
3. **Database Maintenance Thread**: Performs periodic cleanup of the database
4. **TCP Communication Thread**: Handles remote control via TCP connections

## TCP API Commands

The server supports various commands through its TCP interface for control, querying and managing the screenshot service.

### Server Control Commands

- **0**: Stop the server - Sets the global signal to stop all services
- **1**: Start the server - Sets the global signal to start all services
- **2**: Pause the server - Sets the global signal to pause all services
- **hello server**: Connection check - Returns "1" to confirm the server is running

### SQL Commands (Database Queries)

- **sql count**: Get total count of screenshots in the database

  - `sql count`: Returns the total number of screenshots in the database
  - `sql count date YYYYMMDD`: Returns the count of screenshots taken on a specific date (format: YYYYMMDD)
  - `sql count date all`: Returns the count of screenshots for each date in the database, sorted chronologically
  - `sql count hour HH`: Returns the count of screenshots taken during a specific hour (00-23)
  - `sql count hour all`: Returns the count of screenshots for each hour (00-23), aggregated across all dates
  - `sql count date YYYYMMDD hour all`: Returns the count of screenshots per hour for a specific date
  - `sql count hour HH date all`: Returns the count of screenshots per date for a specific hour
  - `sql count date YYYYMMDD hour HH`: Returns the count of screenshots for a specific date and hour

- **sql dump**: Save query results to a file in the dump path

  - `sql dump count`: Dumps the total count to a file
  - `sql dump count date YYYYMMDD`: Dumps the count for a specific date to a file
  - `sql dump count date all`: Dumps counts for all dates to a file
  - `sql dump count hour HH`: Dumps the count for a specific hour to a file
  - `sql dump count hour all`: Dumps counts for all hours to a file
  - `sql dump count date YYYYMMDD hour all`: Dumps counts per hour for a specific date to a file
  - `sql dump count hour HH date all`: Dumps counts per date for a specific hour to a file
  - `sql dump filename`: Dumps all filenames to a file
  - `sql dump filename date YYYYMMDD`: Dumps filenames for a specific date to a file
  - `sql dump filename hour HH`: Dumps filenames for a specific hour to a file
  - `sql dump filename date YYYYMMDD hour HH`: Dumps filenames for a specific date and hour to a file

- **sql min_date**: Returns the earliest date that has screenshots in the database
- **sql max_date**: Returns the latest date that has screenshots in the database

### Image Export Commands

- **img count YYYYMMDDHHMM-HHMM**: Returns the number of archived images in Img_path for the given same-day time range (inclusive)
- **img copy YYYYMMDDHHMM-HHMM [dest]**: Clears `dest` then copies matching images (default `./img_dump` when omitted)

### Management Commands

- **man dump clean**: Cleans up dump files from the dump directory
- **man mem check**: Starts the memory image checking robot to scan for image integrity
- **man tidy database**: Runs database maintenance to clean up and optimize the database
- **man import-dir [dir] [--remap A:B,...]**: Imports PNG metadata from an external directory into the local database
  - Uses PNG EXIF metadata first, then falls back to filename parsing
  - Skips duplicates by ID and updates existing rows when filename matches with a different ID
  - Example: `man import-dir D:/backup/screenshots`
  - Example with remap: `man import-dir D:/backup/screenshots --remap 1:2,2:3`
- **man status**: Shows the current status of the screenshot service and storage
  - Displays if screenshot service is running or stopped
  - Shows the number of active screenshot threads if running
  - Indicates if storage is enabled or disabled
- **man store**: Enables storage of screenshots (turns on saving to disk)
- **man nostore**: Disables storage of screenshots (turns off saving to disk)
- **man config load [path]**: Loads a configuration file from the specified path
  - Updates configuration settings dynamically without restarting
  - Currently only updates the screenshot_second parameter

## Database Schema

Screenshots are stored in a SQLite database with the following schema:

- id: Unique identifier (SHA-256 hash of filename)
- hash: Image hash
- hash_kind: Type of hash algorithm used
- year, month, day, hour, minute, second: Timestamp information
- display_num: Monitor/display number
- file_name: Original filename

## Network Interface

The application listens on a TCP port (configurable) on 127.0.0.1, allowing for:

- Remote control commands
- Screenshot retrieval
- Status monitoring

## Usage

1. Start the application (it will run in the background)
2. Screenshots are automatically taken at defined intervals
3. Access and control via TCP interface
4. Screenshots are stored in the configured cache path
