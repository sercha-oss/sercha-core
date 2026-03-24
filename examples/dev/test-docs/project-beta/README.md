# Project Beta

A Python-based microservice for data processing.

## Overview

Project Beta processes incoming data streams and provides analytics capabilities.

## Installation

```bash
pip install -r requirements.txt
python app.py
```

## API Endpoints

- `GET /status` - Health check
- `POST /process` - Process data
- `GET /analytics` - Get analytics data

## Configuration

Set environment variables:
- `BETA_PORT`: Server port (default: 5000)
- `BETA_DEBUG`: Enable debug mode
- `BETA_DB_URL`: Database connection string
