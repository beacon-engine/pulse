# Development Setup

This guide will help you set up your development environment for building Pulse. We'll install Rust, essential tools, and configure your IDE for the best development experience.

## Prerequisites

- **Operating System**: macOS, Linux, or Windows (WSL recommended)
- **Terminal**: Basic familiarity with command-line interface
- **Git**: For version control (install from https://git-scm.com/)
- **Docker**: For testing container management features (install from https://docker.com/)

## Step 1: Install Rust

### Using rustup (Recommended)

Rustup is the official Rust installer and version manager.

```bash
# Install rustup
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Follow the prompts (default installation is fine)
# After installation, restart your terminal or run:
source $HOME/.cargo/env
```

### Verify Installation

```bash
# Check Rust version (should be 1.70+ or later)
rustc --version

# Check Cargo version (Rust's package manager)
cargo --version

# Check rustup version
rustup --version
```

Expected output:
```
rustc 1.77.0 (or newer)
cargo 1.77.0 (or newer)
rustup 1.26.0 (or newer)
```

## Step 2: Install Essential Tools

### rust-analyzer (IDE Language Server)

This provides code completion, error checking, and other IDE features.

```bash
# Install rust-analyzer
rustup component add rust-analyzer
```

### cargo-watch (Auto-rebuild on file changes)

This tool automatically rebuilds your project when files change, giving you instant feedback.

```bash
cargo install cargo-watch
```

### cargo-edit (Manage dependencies easily)

Adds commands like `cargo add` to easily add dependencies to your project.

```bash
cargo install cargo-edit
```

### clippy (Linter)

Clippy catches common mistakes and suggests improvements to your Rust code.

```bash
rustup component add clippy
```

### rustfmt (Code Formatter)

Automatically formats your code according to Rust style guidelines.

```bash
rustup component add rustfmt
```

## Step 3: IDE Setup

### Option 1: Visual Studio Code (Recommended for beginners)

1. **Install VS Code**: Download from https://code.visualstudio.com/

2. **Install Rust Extension**:
   - Open VS Code
   - Go to Extensions (Cmd+Shift+X on Mac, Ctrl+Shift+X on Windows/Linux)
   - Search for "rust-analyzer"
   - Install the official rust-analyzer extension

3. **Recommended VS Code Settings**:
   
   Create or update `.vscode/settings.json` in your project:
   ```json
   {
     "rust-analyzer.checkOnSave.command": "clippy",
     "editor.formatOnSave": true,
     "[rust]": {
       "editor.defaultFormatter": "rust-lang.rust-analyzer"
     }
   }
   ```

4. **Useful VS Code Extensions**:
   - **Error Lens**: Shows errors inline
   - **Better TOML**: Syntax highlighting for Cargo.toml
   - **GitLens**: Enhanced Git integration

### Option 2: Other IDEs

- **IntelliJ IDEA / CLion**: Install the Rust plugin
- **Vim/Neovim**: Use rust-analyzer with your LSP client
- **Emacs**: Use rust-analyzer with lsp-mode

## Step 4: Verify Everything Works

Let's create a simple test project to make sure everything is set up correctly.

```bash
# Create a new Rust project
cargo new hello-rust
cd hello-rust

# Build the project
cargo build

# Run the project
cargo run
```

You should see:
```
   Compiling hello-rust v0.1.0
    Finished dev [unoptimized + debuginfo] target(s) in 0.50s
     Running `target/debug/hello-rust`
Hello, world!
```

## Step 5: Understanding Cargo

Cargo is Rust's build system and package manager. Here are the essential commands:

```bash
# Create a new project
cargo new project-name

# Build your project (debug mode)
cargo build

# Build with optimizations (release mode)
cargo build --release

# Run your project
cargo run

# Check your code for errors (faster than build)
cargo check

# Run tests
cargo test

# Format your code
cargo fmt

# Run the linter
cargo clippy

# Add a dependency
cargo add tokio

# Update dependencies
cargo update

# Auto-rebuild on file changes
cargo watch -x run
```

## Step 6: Project Structure

A typical Rust project looks like this:

```
pulse/
├── Cargo.toml          # Project metadata and dependencies
├── Cargo.lock          # Locked dependency versions (auto-generated)
├── src/
│   ├── main.rs         # Entry point for binary applications
│   └── lib.rs          # Entry point for libraries (optional)
├── tests/              # Integration tests
├── benches/            # Benchmarks
└── examples/           # Example code
```

## Step 7: Install Docker (for testing)

Pulse manages Docker containers, so you'll need Docker installed.

### macOS
```bash
# Install Docker Desktop from https://docker.com/
# Or use Homebrew:
brew install --cask docker
```

### Linux
```bash
# Ubuntu/Debian
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Add your user to docker group (avoid sudo)
sudo usermod -aG docker $USER
# Log out and back in for this to take effect
```

### Verify Docker
```bash
docker --version
docker ps
```

## Common Issues & Solutions

### Issue: `rustc` not found after installation
**Solution**: Restart your terminal or run `source $HOME/.cargo/env`

### Issue: Permission denied when running Docker
**Solution**: Add your user to the docker group (Linux) or ensure Docker Desktop is running (macOS/Windows)

### Issue: Slow compilation times
**Solution**: 
- Use `cargo check` instead of `cargo build` during development
- Enable incremental compilation (already default)
- Consider using `sccache` for caching

### Issue: rust-analyzer is slow or not working
**Solution**:
- Restart your IDE
- Run `cargo clean` and rebuild
- Check that rust-analyzer is using the correct Rust toolchain

## Next Steps

Now that your environment is set up, you're ready to:

1. Learn Rust basics ([Rust Basics](./02-rust-basics.md))
2. Build your first HTTP server ([Hello World Server](./03-hello-world-server.md))

## Additional Resources

- **Official Rust Book**: https://doc.rust-lang.org/book/
- **Rust by Example**: https://doc.rust-lang.org/rust-by-example/
- **Cargo Book**: https://doc.rust-lang.org/cargo/
- **Rust Playground**: https://play.rust-lang.org/ (test code in browser)
- **Rust Community**: https://www.rust-lang.org/community

Happy coding! 🦀
