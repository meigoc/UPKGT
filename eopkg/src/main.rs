mod parser;
mod installer;

use std::env;
use std::path::Path;

fn main() {
    let args: Vec<String> = env::args().collect();
    if args.len() < 3 {
        eprintln!("Usage: {} <command> <package_path>", args[0]);
        return;
    }

    let command = &args[1];
    let package_path = Path::new(&args[2]);

    match command.as_str() {
        "install" => installer::install_package(package_path),
        _ => eprintln!("Unknown command: {}", command),
    }
}
