# Building Your First HTTP Server with Axum

This guide will walk you through building a simple HTTP server using Axum, one of the fastest and most ergonomic web frameworks in Rust. This is your first hands-on Beacon component!

## What You'll Build

A simple HTTP server that:
- Responds to GET requests at `/` with "Hello, Beacon!"
- Responds to GET requests at `/health` with a health check status
- Runs on `http://localhost:3000`

## Why Axum?

- **Fast**: Built on top of Tokio and Hyper (production-proven)
- **Type-safe**: Leverages Rust's type system to prevent runtime errors
- **Ergonomic**: Clean, intuitive API
- **Modular**: Easy to add features incrementally

## Step 1: Create the Project

```bash
# Navigate to your projects directory
cd /Users/mac/Documents/projects/personal/unbound

# Create a new Rust project
cargo new beacon-server
cd beacon-server
```

This creates:
```
beacon-server/
├── Cargo.toml      # Project configuration
└── src/
    └── main.rs     # Your code goes here
```

## Step 2: Add Dependencies

Open `Cargo.toml` and add the required dependencies:

```toml
[package]
name = "beacon-server"
version = "0.1.0"
edition = "2021"

[dependencies]
# Tokio: Async runtime that powers everything
tokio = { version = "1.36", features = ["full"] }

# Axum: Web framework for building APIs
axum = "0.7"
```

**What these dependencies do**:
- `tokio`: Provides the async runtime (handles concurrent operations)
- `axum`: Web framework built on top of Tokio

## Step 3: Write the Code

Replace the contents of `src/main.rs` with this heavily commented code:

```rust
// Import the necessary modules from Axum
use axum::{
    routing::get,  // For defining GET routes
    Router,        // For building the application router
};

// The #[tokio::main] attribute transforms our async main function
// into a synchronous main function that runs on the Tokio runtime
#[tokio::main]
async fn main() {
    // Build our application with routes
    // Router::new() creates a new router
    let app = Router::new()
        // Add a route: GET / -> handler function
        .route("/", get(root_handler))
        // Add another route: GET /health -> health check
        .route("/health", get(health_handler));

    // Define the address to listen on
    // 0.0.0.0 means "listen on all network interfaces"
    // 3000 is the port number
    let listener = tokio::net::TcpListener::bind("0.0.0.0:3000")
        .await  // .await waits for the async operation to complete
        .unwrap();  // .unwrap() panics if there's an error (fine for learning)

    // Print a message so we know the server started
    println!("🚀 Beacon server listening on http://localhost:3000");
    
    // Start the server and serve our app
    // This runs forever until you stop it (Ctrl+C)
    axum::serve(listener, app)
        .await
        .unwrap();
}

// Handler function for the root route (/)
// This is an async function that returns a string
async fn root_handler() -> &'static str {
    "Hello, Beacon! 🚀"
}

// Handler function for the health check route (/health)
async fn health_handler() -> &'static str {
    "OK"
}
```

## Step 4: Run the Server

```bash
# Build and run the server
cargo run
```

You should see:
```
   Compiling beacon-server v0.1.0
    Finished dev [unoptimized + debuginfo] target(s) in 3.2s
     Running `target/debug/beacon-server`
🚀 Beacon server listening on http://localhost:3000
```

## Step 5: Test It

Open a new terminal and test your server:

```bash
# Test the root endpoint
curl http://localhost:3000/
# Output: Hello, Beacon! 🚀

# Test the health endpoint
curl http://localhost:3000/health
# Output: OK
```

Or open your browser and visit:
- http://localhost:3000/
- http://localhost:3000/health

## Understanding the Code

Let's break down what's happening:

### 1. The Async Runtime
```rust
#[tokio::main]
async fn main() {
```
- `#[tokio::main]` is a macro that sets up the Tokio async runtime
- Without this, you can't use `.await` in your code
- It handles all the complexity of async execution for you

### 2. Building Routes
```rust
let app = Router::new()
    .route("/", get(root_handler))
    .route("/health", get(health_handler));
```
- `Router::new()` creates a new router
- `.route(path, method(handler))` adds a route
- Routes are matched in the order they're defined

### 3. Binding to a Port
```rust
let listener = tokio::net::TcpListener::bind("0.0.0.0:3000")
    .await
    .unwrap();
```
- `TcpListener::bind()` creates a listener on the specified address
- `.await` waits for the binding to complete (it's async)
- `.unwrap()` panics if binding fails (e.g., port already in use)

### 4. Handler Functions
```rust
async fn root_handler() -> &'static str {
    "Hello, Beacon! 🚀"
}
```
- Handlers are async functions
- They return data that Axum automatically converts to HTTP responses
- `&'static str` is a string that lives for the entire program duration

## Next Steps: Adding JSON Responses

Let's make it more realistic by returning JSON (like a real API would).

### Step 1: Add Serde

Update `Cargo.toml`:
```toml
[dependencies]
tokio = { version = "1.36", features = ["full"] }
axum = "0.7"
serde = { version = "1.0", features = ["derive"] }  # Add this
```

### Step 2: Update the Code

```rust
use axum::{
    routing::get,
    Router,
    Json,  // Add this for JSON responses
};
use serde::Serialize;  // Add this

#[tokio::main]
async fn main() {
    let app = Router::new()
        .route("/", get(root_handler))
        .route("/health", get(health_handler))
        .route("/status", get(status_handler));  // New route

    let listener = tokio::net::TcpListener::bind("0.0.0.0:3000")
        .await
        .unwrap();

    println!("🚀 Beacon server listening on http://localhost:3000");
    
    axum::serve(listener, app)
        .await
        .unwrap();
}

async fn root_handler() -> &'static str {
    "Hello, Beacon! 🚀"
}

async fn health_handler() -> &'static str {
    "OK"
}

// New handler that returns JSON
// Define a struct for the response
#[derive(Serialize)]
struct StatusResponse {
    status: String,
    version: String,
    uptime_seconds: u64,
}

async fn status_handler() -> Json<StatusResponse> {
    // Create the response struct
    let response = StatusResponse {
        status: "running".to_string(),
        version: "0.1.0".to_string(),
        uptime_seconds: 42,  // Hardcoded for now
    };
    
    // Json() automatically serializes the struct to JSON
    Json(response)
}
```

### Step 3: Test JSON Response

```bash
# Restart the server (Ctrl+C to stop, then cargo run)
cargo run

# In another terminal:
curl http://localhost:3000/status
```

Output:
```json
{"status":"running","version":"0.1.0","uptime_seconds":42}
```

## Common Issues & Solutions

### Issue: "Address already in use"
**Cause**: Another process is using port 3000
**Solution**: 
```bash
# Find what's using port 3000
lsof -i :3000

# Kill it or change your port in the code
let listener = tokio::net::TcpListener::bind("0.0.0.0:3001")
```

### Issue: Compilation errors about lifetimes
**Cause**: Rust's borrow checker is protecting you
**Solution**: Read the error message carefully—Rust's errors are very helpful and often suggest fixes

### Issue: Server doesn't respond
**Cause**: Firewall or wrong address
**Solution**: 
- Try `127.0.0.1:3000` instead of `0.0.0.0:3000`
- Check firewall settings

## Development Workflow

Use `cargo-watch` for automatic rebuilding:

```bash
# Install cargo-watch (if you haven't already)
cargo install cargo-watch

# Auto-rebuild and run on file changes
cargo watch -x run
```

Now every time you save a file, the server automatically rebuilds and restarts!

## What You Learned

✅ How to create a Rust project with Cargo  
✅ How to add dependencies to `Cargo.toml`  
✅ How to use Tokio's async runtime  
✅ How to build HTTP routes with Axum  
✅ How to return both text and JSON responses  
✅ How to test your API with curl  

## Next Steps

You've built your first Beacon component! Next, you'll learn to:

1. **Connect to Docker** ([Docker Integration](./04-docker-integration.md))
2. **Add Prometheus metrics** ([Prometheus Metrics](./05-prometheus-metrics.md))
3. **Implement WebSockets** ([WebSocket Real-time](./06-websocket-realtime.md))

Each component builds on what you've learned here. Take your time, experiment, and don't be afraid to break things—that's how you learn!

## Challenge Exercise

Try adding these features on your own:

1. Add a `/version` endpoint that returns just the version number
2. Add a `/containers` endpoint that returns a JSON array of fake container data
3. Add error handling using `Result` instead of `.unwrap()`

Great work! You're on your way to building Beacon! 🚀
