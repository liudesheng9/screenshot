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

