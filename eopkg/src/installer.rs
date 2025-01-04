use crate::parser::{parse_files_xml, parse_metadata_xml, File};
use sha1::{Digest, Sha1};
use std::fs::{self, File as FsFile};
use std::io::{BufReader, Read};
use std::path::Path;
use tar::Archive;
use xz2::read::XzDecoder;

pub fn install_package(package_path: &Path) {
    let metadata_path = package_path.join("metadata.xml");
    let files_path = package_path.join("files.xml");
    let tar_path = package_path.join("install.tar.xz");

    // Чтение и вывод информации из metadata.xml
    let metadata = parse_metadata_xml(metadata_path.to_str().unwrap())
        .expect("Failed to parse metadata.xml");
    println!("Installing package: {}", metadata.Name);
    println!("Description: {}", metadata.Description);
    println!("Architecture: {}", metadata.Architecture);

    // Чтение files.xml
    let files = parse_files_xml(files_path.to_str().unwrap()).expect("Failed to parse files.xml");

    // Распаковка файлов
    let tar_file = FsFile::open(&tar_path).expect("Failed to open install.tar.xz");
    let decompressed = XzDecoder::new(BufReader::new(tar_file));
    let mut archive = Archive::new(decompressed);

    for file in &files.File {
        let extracted_path = Path::new("/").join(&file.Path);

        // Извлечение файла
        if let Err(e) = archive.unpack_in("/") {
            eprintln!("Failed to unpack file {}: {}", file.Path, e);
            continue;
        }

        // Проверка хэша
        let mut extracted_file = FsFile::open(&extracted_path)
            .expect(&format!("Failed to open extracted file: {}", file.Path));
        let mut buffer = Vec::new();
        extracted_file
            .read_to_end(&mut buffer)
            .expect("Failed to read file for hash check");
        let hash = Sha1::digest(&buffer);
        let calculated_hash = format!("{:x}", hash);
        if calculated_hash != file.Hash {
            eprintln!("Hash mismatch for file {}: expected {}, got {}", file.Path, file.Hash, calculated_hash);
        }
    }

    println!("Package installed successfully!");
}
