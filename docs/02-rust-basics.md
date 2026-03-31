# Rust Basics for Beacon

This guide covers the essential Rust concepts you need to understand before building Beacon. We'll focus on practical knowledge rather than comprehensive coverage.

## Why Rust?

Before diving in, let's understand why Rust is perfect for Beacon:

- **Memory Safety**: No null pointer crashes, no data races
- **Performance**: As fast as C/C++, no garbage collection pauses
- **Concurrency**: Fearless concurrent programming
- **Small Footprint**: Minimal runtime overhead

## Core Concepts

### 1. Variables and Mutability

In Rust, variables are **immutable by default**. You must explicitly make them mutable.

```rust
// Immutable variable (default)
let x = 5;
// x = 6; // ❌ This would cause a compile error

// Mutable variable
let mut y = 5;
y = 6; // ✅ This is fine

// Type annotations (usually inferred)
let z: i32 = 10;
```

**Why this matters for Beacon**: Immutability prevents accidental changes to configuration, making your code safer.

### 2. Ownership (The Most Important Concept)

Rust's ownership system is what makes it unique. Every value has a single owner, and when the owner goes out of scope, the value is dropped.

```rust
fn main() {
    let s1 = String::from("hello");
    let s2 = s1; // s1 is "moved" to s2
    
    // println!("{}", s1); // ❌ Error! s1 no longer owns the string
    println!("{}", s2); // ✅ s2 owns the string now
}
```

**Key Rules**:
1. Each value has exactly one owner
2. When the owner goes out of scope, the value is dropped
3. You can't use a value after it's been moved

**Why this matters for Beacon**: No memory leaks, no use-after-free bugs. Container management is safe by default.

### 3. Borrowing and References

Instead of moving ownership, you can **borrow** a value using references.

```rust
fn main() {
    let s1 = String::from("hello");
    
    // Immutable borrow
    let len = calculate_length(&s1);
    println!("Length of '{}' is {}", s1, len); // ✅ s1 still usable
}

fn calculate_length(s: &String) -> usize {
    s.len() // We can read s, but not modify it
}
```

**Mutable borrows**:
```rust
fn main() {
    let mut s = String::from("hello");
    change(&mut s);
    println!("{}", s); // Prints "hello, world"
}

fn change(s: &mut String) {
    s.push_str(", world");
}
```

**Borrowing Rules**:
- You can have **many immutable references** OR **one mutable reference**
- But not both at the same time
- References must always be valid

**Why this matters for Beacon**: Safe concurrent access to container state without data races.

### 4. Structs (Data Structures)

Structs let you create custom types.

```rust
// Define a struct
struct Container {
    id: String,
    name: String,
    status: String,
    cpu_usage: f64,
}

// Create an instance
let container = Container {
    id: String::from("abc123"),
    name: String::from("web-server"),
    status: String::from("running"),
    cpu_usage: 25.5,
};

// Access fields
println!("Container {} is {}", container.name, container.status);
```

**Why this matters for Beacon**: You'll use structs to represent containers, clusters, metrics, etc.

### 5. Enums and Pattern Matching

Enums represent types that can be one of several variants.

```rust
enum ContainerStatus {
    Running,
    Stopped,
    Paused,
    Restarting,
}

// Using enums with data
enum Event {
    ContainerStarted(String),
    ContainerStopped(String),
    MetricReceived { name: String, value: f64 },
}

// Pattern matching
fn handle_event(event: Event) {
    match event {
        Event::ContainerStarted(id) => {
            println!("Container {} started", id);
        }
        Event::ContainerStopped(id) => {
            println!("Container {} stopped", id);
        }
        Event::MetricReceived { name, value } => {
            println!("Metric {}: {}", name, value);
        }
    }
}
```

**Why this matters for Beacon**: Perfect for representing container states, API responses, and events.

### 6. Result and Option (Error Handling)

Rust doesn't have exceptions. Instead, it uses `Result` and `Option`.

**Option** - for values that might not exist:
```rust
fn find_container(id: &str) -> Option<Container> {
    // Returns Some(container) if found, None if not found
    if id == "abc123" {
        Some(Container { /* ... */ })
    } else {
        None
    }
}

// Using Option
match find_container("abc123") {
    Some(container) => println!("Found: {}", container.name),
    None => println!("Container not found"),
}
```

**Result** - for operations that can fail:
```rust
use std::fs::File;
use std::io::Error;

fn open_config() -> Result<File, Error> {
    File::open("config.toml")
}

// Using Result
match open_config() {
    Ok(file) => println!("Config opened successfully"),
    Err(error) => println!("Failed to open config: {}", error),
}

// Shorthand with ? operator
fn read_config() -> Result<String, Error> {
    let mut file = File::open("config.toml")?; // Returns early if error
    // ... continue if Ok
    Ok(String::from("config data"))
}
```

**Why this matters for Beacon**: Docker/Kubernetes operations can fail. Result forces you to handle errors explicitly.

### 7. Traits (Interfaces)

Traits define shared behavior.

```rust
// Define a trait
trait Manageable {
    fn start(&self) -> Result<(), String>;
    fn stop(&self) -> Result<(), String>;
    fn status(&self) -> String;
}

// Implement trait for a type
impl Manageable for Container {
    fn start(&self) -> Result<(), String> {
        println!("Starting container {}", self.id);
        Ok(())
    }
    
    fn stop(&self) -> Result<(), String> {
        println!("Stopping container {}", self.id);
        Ok(())
    }
    
    fn status(&self) -> String {
        self.status.clone()
    }
}
```

**Why this matters for Beacon**: You'll use traits to define common behavior for Docker containers, K8s pods, etc.

### 8. Async/Await (Asynchronous Programming)

Beacon needs to handle many concurrent operations (monitoring multiple containers, handling API requests, etc.).

```rust
use tokio; // Async runtime

// Async function
async fn fetch_container_stats(id: &str) -> Result<Stats, Error> {
    // Asynchronous operations
    let stats = fetch_from_docker(id).await?;
    Ok(stats)
}

#[tokio::main]
async fn main() {
    // Run async function
    match fetch_container_stats("abc123").await {
        Ok(stats) => println!("Stats: {:?}", stats),
        Err(e) => println!("Error: {}", e),
    }
}
```

**Key points**:
- `async fn` creates an asynchronous function
- `.await` waits for an async operation to complete
- `tokio` is the async runtime we'll use

**Why this matters for Beacon**: You'll monitor hundreds of containers simultaneously without blocking.

### 9. Modules and Crates

Organize code into modules and use external libraries (crates).

```rust
// In src/docker.rs
pub mod docker {
    pub fn list_containers() -> Vec<String> {
        vec!["container1".to_string(), "container2".to_string()]
    }
}

// In src/main.rs
mod docker; // Import the module

fn main() {
    let containers = docker::list_containers();
    println!("{:?}", containers);
}
```

**External crates** (in Cargo.toml):
```toml
[dependencies]
tokio = { version = "1.36", features = ["full"] }
axum = "0.7"
bollard = "0.16"
```

**Why this matters for Beacon**: You'll organize code into modules (docker, kubernetes, metrics, etc.) and use many crates.

## Practical Example: Container Struct

Let's combine these concepts into something Beacon-like:

```rust
use std::collections::HashMap;

// Enum for container status
#[derive(Debug, Clone)]
enum Status {
    Running,
    Stopped,
    Paused,
}

// Struct representing a container
#[derive(Debug)]
struct Container {
    id: String,
    name: String,
    status: Status,
    labels: HashMap<String, String>,
}

// Implementation block for Container
impl Container {
    // Constructor
    fn new(id: String, name: String) -> Self {
        Container {
            id,
            name,
            status: Status::Stopped,
            labels: HashMap::new(),
        }
    }
    
    // Methods
    fn start(&mut self) -> Result<(), String> {
        match self.status {
            Status::Running => Err("Already running".to_string()),
            _ => {
                self.status = Status::Running;
                Ok(())
            }
        }
    }
    
    fn stop(&mut self) -> Result<(), String> {
        match self.status {
            Status::Stopped => Err("Already stopped".to_string()),
            _ => {
                self.status = Status::Stopped;
                Ok(())
            }
        }
    }
    
    fn add_label(&mut self, key: String, value: String) {
        self.labels.insert(key, value);
    }
}

fn main() {
    // Create a container
    let mut container = Container::new(
        "abc123".to_string(),
        "web-server".to_string(),
    );
    
    // Add labels
    container.add_label("env".to_string(), "production".to_string());
    
    // Start the container
    match container.start() {
        Ok(_) => println!("Container started successfully"),
        Err(e) => println!("Failed to start: {}", e),
    }
    
    println!("{:?}", container);
}
```

## Learning Resources

### Essential Reading
1. **The Rust Book** (Chapters 1-10): https://doc.rust-lang.org/book/
   - Focus on chapters 3, 4, 6, 9, 10
2. **Rust by Example**: https://doc.rust-lang.org/rust-by-example/
   - Great for hands-on learning

### Practice
- **Rustlings**: Small exercises to learn Rust (https://github.com/rust-lang/rustlings)
- **Rust Playground**: Test code snippets (https://play.rust-lang.org/)

## Next Steps

Now that you understand the basics:

1. **Practice**: Spend a few days writing small Rust programs
2. **Build**: Start with the [Hello World Server](./03-hello-world-server.md)
3. **Reference**: Keep the Rust Book handy for deeper explanations

Don't worry if ownership/borrowing feels confusing at first—it clicks after practice! The Rust compiler is your friend; its error messages are incredibly helpful.

Ready to build your first Beacon component? Let's go! 🚀
