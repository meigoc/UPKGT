use std::fs::File;
use std::io::{self, BufReader};
use std::path::Path;
use tar::Archive;
use xz2::read::XzDecoder;

pub fn install_package(package_path: &Path) {
    let tar_path = package_path.join("install.tar.xz");
    let tar_file = File::open(&tar_path).expect("Failed to open install.tar.xz");

    let decompressed = XzDecoder::new(BufReader::new(tar_file));
    let mut archive = Archive::new(decompressed);

    archive.unpack("/").expect("Failed to unpack the archive");

    println!("Package installed successfully!");
}
