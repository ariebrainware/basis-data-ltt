# Project Documentation

This document summarize the setup, available routes, and core functionalities provided by the project.

## Table of Contents
- [Overview](#overview)
- [Setup](#setup)
- [Routes](#routes)
- [Functionality](#functionality)

## Overview

This project is a backend service written in Go. It is designed to manage basis data and provide a RESTful API interface. The documentation covers installation, configuration, and route details.

## Setup

### Prerequisites
- Go (version 1.15+ recommended)
- Git

### Installation

1. Clone the repository:
    ```
    git clone https://github.com/ariebrainware/basis-data-ltt.git
    ```
2. Navigate to the project directory:
    ```
    cd /Users/ariebrainware/go/src/github.com/ariebrainware/basis-data-ltt
    ```
3. Install dependencies:
    ```
    go mod download
    ```

### Configuration
- Create and configure any necessary environment variables. For example, a `.env` file can be used to set database connections and server ports.
- Sample environment variables:
  ```
    APPNAME=ltt-be
    APITOKEN=ed25519key
    APPENV=local
    APPPORT=19091
    GINMODE=debug
    DBHOST=localhost
    DBPORT=3306
    DBNAME=databasename
    DBUSER=databaseuser
    DBPASS=databasepassword
  ```

### Build and Run
- To build the project, use:
  ```
  go build -o basisdata
  ```
- To run the service, use:
  ```
  ./basisdata
  ```
- During development, you can simply run:
  ```
  go run main.go
  ```

## Routes

Below is an outline of the REST API endpoints provided:

### GET /
- **Description:** Health check or landing route.
- **Response:** Simple status message confirming the service is operational.

### GET /api/resource
- **Description:** Retrieve a list of resources.
- **Response:** JSON array of resources.

### POST /api/resource
- **Description:** Create a new resource.
- **Request Body:** JSON payload with resource details.
- **Response:** JSON object of the newly created resource.

### PUT /api/resource/{id}
- **Description:** Update an existing resource.
- **Path Parameter:** `id` of the resource to update.
- **Request Body:** JSON payload with updated resource details.
- **Response:** JSON object of the updated resource.

### DELETE /api/resource/{id}
- **Description:** Delete a specific resource.
- **Path Parameter:** `id` of the resource to delete.
- **Response:** JSON message confirming deletion.

## Functionality

### Data Management
The primary functionality revolves around creating, reading, updating, and deleting (CRUD) resources in the database. This includes:
- Validating input data.
- Managing database transactions.
- Returning appropriate HTTP status codes and error messages.

### Error Handling
- The project uses proper error handling middleware to capture and log errors.
- Returns structured JSON error responses with helpful error messages.

### Middleware
- Logging: Request and response logging.
- Authentication & Authorization: Secure certain routes based on user roles (if applicable).

### Testing
- Include unit and integration tests to cover API endpoint behaviors.
- Use Go's built-in testing framework:
  ```
  go test ./...
  ```

## Conclusion

This document provides a high-level overview of the necessary steps for setting up, running, and understanding the API and its routes. For more details, please refer to inline code comments and further documentation within the codebase.
