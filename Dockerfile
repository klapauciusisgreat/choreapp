# Start from the official Golang image
FROM golang:1.21

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files (if using Go modules) and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Install sqlite3
RUN apt-get update && apt-get install -y sqlite3

# Copy the rest of the application code
COPY . /app/

# Build the Go application
RUN go build -o chore-tracker ./app

# Expose the port the app runs on
EXPOSE 8080

# Command to run the executable
CMD ["./chore-tracker"]