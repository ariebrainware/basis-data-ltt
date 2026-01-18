# Project Documentation

This document summarize the setup, available routes, and core functionalities provided by the project.

## Table of Contents
- [Overview](#overview)
- [Setup](#setup)
- [API Documentation](#api-documentation)
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
    APPNAME=basis-data-ltt
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

## API Documentation

This project uses Swagger/OpenAPI for API documentation. Once the server is running, you can access the interactive API documentation at:

```
http://localhost:19091/swagger/index.html
```

**Note:** The Swagger UI is publicly accessible and does not require authentication to view the documentation. However, to test the API endpoints directly from Swagger UI, you will need to authenticate by clicking the "Authorize" button and providing the required tokens.

### Swagger UI Features

The Swagger UI provides:
- **Interactive API Testing**: Try out API endpoints directly from the browser
- **Complete Endpoint Documentation**: All endpoints with request/response schemas
- **Authentication Support**: Built-in support for Bearer token and session token authentication
- **Request/Response Examples**: Sample payloads for all endpoints

### Updating API Documentation

When you make changes to the API endpoints, regenerate the Swagger documentation using:

```bash
# Install swag CLI (one-time setup)
go install github.com/swaggo/swag/cmd/swag@latest

# Generate/update documentation
swag init --parseDependency --parseInternal
```

The documentation is generated from Go annotations in the code. After updating, the changes will be reflected in the Swagger UI.

## Routes

Below is an outline of the REST API endpoints provided:

**Note:** For complete and interactive API documentation, please visit the [Swagger UI](http://localhost:19091/swagger/index.html) when the server is running.

### Authentication Endpoints

- **POST /login** - User login with email and password
- **POST /signup** - Register a new user account
- **DELETE /logout** - Invalidate user session (requires authentication)
- **GET /token/validate** - Validate session token

### Patient Endpoints (Admin only, except POST /patient)

- **GET /patient** - List all patients with pagination and filtering
- **POST /patient** - Create a new patient (public endpoint)
- **GET /patient/{id}** - Get patient details by ID
- **PATCH /patient/{id}** - Update patient information
- **DELETE /patient/{id}** - Delete a patient

### Disease Endpoints (Admin only)

- **GET /disease** - List all diseases with pagination
- **POST /disease** - Create a new disease
- **GET /disease/{id}** - Get disease details by ID
- **PATCH /disease/{id}** - Update disease information
- **DELETE /disease/{id}** - Delete a disease

### Treatment Endpoints (Admin and Therapist)

- **GET /treatment** - List all treatments with filtering options
- **POST /treatment** - Create a new treatment record
- **PATCH /treatment/{id}** - Update treatment information
- **DELETE /treatment/{id}** - Delete a treatment record

### Therapist Endpoints (Admin only)

- **GET /therapist** - List all therapists with pagination and filtering
- **POST /therapist** - Register a new therapist
- **GET /therapist/{id}** - Get therapist details by ID
- **PATCH /therapist/{id}** - Update therapist information
- **PUT /therapist/{id}** - Approve a therapist account
- **DELETE /therapist/{id}** - Delete a therapist

### Legacy Route Documentation

### GET /
- **Description:** Health check or landing route.
- **Response:** Simple status message confirming the service is operational.

### GET /swagger/*
- **Description:** Swagger API documentation UI
- **Response:** Interactive API documentation interface

---

**Note:** The legacy routes below are deprecated. Please refer to the Swagger documentation for the current API structure.

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
